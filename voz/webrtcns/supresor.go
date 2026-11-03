package webrtcns

import (
	"math"
	"math/cmplx"
)

/* ---------- noise_suppressor.cc, one channel at 48 kHz (three bands) ---------- */

var kBlocks160w256FirstHalf = [96]float32{
	0.00000000, 0.01636173, 0.03271908, 0.04906767, 0.06540313,
	0.08172107, 0.09801714, 0.11428696, 0.13052619, 0.14673047,
	0.16289547, 0.17901686, 0.19509032, 0.21111155, 0.22707626,
	0.24298018, 0.25881905, 0.27458862, 0.29028468, 0.30590302,
	0.32143947, 0.33688985, 0.35225005, 0.36751594, 0.38268343,
	0.39774847, 0.41270703, 0.42755509, 0.44228869, 0.45690388,
	0.47139674, 0.48576339, 0.50000000, 0.51410274, 0.52806785,
	0.54189158, 0.55557023, 0.56910015, 0.58247770, 0.59569930,
	0.60876143, 0.62166057, 0.63439328, 0.64695615, 0.65934582,
	0.67155895, 0.68359230, 0.69544264, 0.70710678, 0.71858162,
	0.72986407, 0.74095113, 0.75183981, 0.76252720, 0.77301045,
	0.78328675, 0.79335334, 0.80320753, 0.81284668, 0.82226822,
	0.83146961, 0.84044840, 0.84920218, 0.85772861, 0.86602540,
	0.87409034, 0.88192126, 0.88951608, 0.89687274, 0.90398929,
	0.91086382, 0.91749450, 0.92387953, 0.93001722, 0.93590593,
	0.94154407, 0.94693013, 0.95206268, 0.95694034, 0.96156180,
	0.96592583, 0.97003125, 0.97387698, 0.97746197, 0.98078528,
	0.98384601, 0.98664333, 0.98917651, 0.99144486, 0.99344778,
	0.99518473, 0.99665524, 0.99785892, 0.99879546, 0.99946459,
	0.99986614,
}

func applyFilterBankWindow(x *[kFftSize]float32) {
	for i := 0; i < 96; i++ {
		x[i] = kBlocks160w256FirstHalf[i] * x[i]
	}
	for i, k := 161, 95; i < kFftSize; i, k = i+1, k-1 {
		x[i] = kBlocks160w256FirstHalf[k] * x[i]
	}
}

func formExtendedFrame(frame *[kNsFrameSize]float32, oldData *[kOverlapSize]float32, ext *[kFftSize]float32) {
	copy(ext[:], oldData[:])
	copy(ext[kOverlapSize:], frame[:])
	copy(oldData[:], ext[kFftSize-kOverlapSize:])
}

func overlapAndAdd(ext *[kFftSize]float32, mem *[kOverlapSize]float32, out *[kNsFrameSize]float32) {
	for i := 0; i < kOverlapSize; i++ {
		out[i] = mem[i] + ext[i]
	}
	copy(out[kOverlapSize:], ext[kOverlapSize:kNsFrameSize])
	copy(mem[:], ext[kNsFrameSize:])
}

func delaySignal(frame *[kNsFrameSize]float32, buf *[kOverlapSize]float32, delayed *[kNsFrameSize]float32) {
	const kSamplesFromFrame = kNsFrameSize - kOverlapSize
	copy(delayed[:], buf[:])
	copy(delayed[kOverlapSize:], frame[:kSamplesFromFrame])
	copy(buf[:], frame[kSamplesFromFrame:])
}

func energyExt(x *[kFftSize]float32) float32 {
	var e float32
	for _, v := range x {
		e += v * v
	}
	return e
}

func energyParts(frame *[kNsFrameSize]float32, old *[kOverlapSize]float32) float32 {
	var e float32
	for _, v := range old {
		e += v * v
	}
	for _, v := range frame {
		e += v * v
	}
	return e
}

// fftReal is NrFft::Fft: the real and imaginary parts of the 256-point transform of time data (bins 0..128). The
// sign of the imaginary part does not matter here: the suppressor only scales each bin by a real gain.
func fftReal(t *[kFftSize]float32, re, im *[kFftSize]float32) {
	var c [kFftSize]complex128
	for i, v := range t {
		c[i] = complex(float64(v), 0)
	}
	fft(c[:], false)
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		re[i], im[i] = float32(real(c[i])), float32(imag(c[i]))
	}
	im[0], im[kFftSizeBy2Plus1-1] = 0, 0
}

// ifftReal is NrFft::Ifft (with its 2/N scaling folded into an exact inverse).
func ifftReal(re, im *[kFftSize]float32, t *[kFftSize]float32) {
	var c [kFftSize]complex128
	c[0] = complex(float64(re[0]), 0)
	c[kFftSizeBy2Plus1-1] = complex(float64(re[kFftSizeBy2Plus1-1]), 0)
	for i := 1; i < kFftSizeBy2Plus1-1; i++ {
		c[i] = complex(float64(re[i]), float64(im[i]))
		c[kFftSize-i] = cmplx.Conj(c[i])
	}
	fft(c[:], true)
	for i := range t {
		t[i] = float32(real(c[i]))
	}
}

func fft(a []complex128, inversa bool) {
	n := len(a)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	for l := 2; l <= n; l <<= 1 {
		ang := 2 * math.Pi / float64(l)
		if !inversa {
			ang = -ang
		}
		w := cmplx.Rect(1, ang)
		for i := 0; i < n; i += l {
			wk := complex(1, 0)
			for k := 0; k < l/2; k++ {
				u, v := a[i+k], a[i+k+l/2]*wk
				a[i+k], a[i+k+l/2] = u+v, u-v
				wk *= w
			}
		}
	}
	if inversa {
		for i := range a {
			a[i] /= complex(float64(n), 0)
		}
	}
}

func computeMagnitudeSpectrum(re, im *[kFftSize]float32, s *espectro) {
	s[0] = fabs(re[0]) + 1
	s[kFftSizeBy2Plus1-1] = fabs(re[kFftSizeBy2Plus1-1]) + 1
	for i := 1; i < kFftSizeBy2Plus1-1; i++ {
		s[i] = sqrtFastApproximation(re[i]*re[i]+im[i]*im[i]) + 1
	}
}

func computeSnr(filter, prevSignal, signal, prevNoise, noise *espectro, priorSnr, postSnr *espectro) {
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		prevEstimate := prevSignal[i] / (prevNoise[i] + 0.0001) * filter[i]
		if signal[i] > noise[i] {
			postSnr[i] = signal[i]/(noise[i]+0.0001) - 1
		} else {
			postSnr[i] = 0
		}
		priorSnr[i] = 0.98*prevEstimate + (1-0.98)*postSnr[i]
	}
}

func computeUpperBandsGain(minimumAttenuatingGain float32, filter *espectro, speechProbability *espectro, prevAnalysis, signal *espectro) float32 {
	const kNumAvgBins = 32
	const kOneByNumAvgBins = float32(1) / kNumAvgBins
	var avgProbSpeech, avgFilterGain float32
	for i := kFftSizeBy2Plus1 - kNumAvgBins - 1; i < kFftSizeBy2Plus1-1; i++ {
		avgProbSpeech += speechProbability[i]
		avgFilterGain += filter[i]
	}
	avgProbSpeech = avgProbSpeech * kOneByNumAvgBins
	avgFilterGain = avgFilterGain * kOneByNumAvgBins
	var sumAnalysis, sumProcessing float32
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		sumAnalysis += prevAnalysis[i]
		sumProcessing += signal[i]
	}
	avgProbSpeech *= sumProcessing / sumAnalysis
	gain := 0.5 * (1 + tanhf(2*avgProbSpeech-1))
	if avgProbSpeech >= 0.5 {
		gain = 0.25*gain + 0.75*avgFilterGain
	} else {
		gain = 0.5*gain + 0.5*avgFilterGain
	}
	return min(max(gain, minimumAttenuatingGain), 1)
}

// Supresor is NoiseSuppressor for one channel at 48 kHz, with the band split the audio buffer does around it.
type Supresor struct {
	params              suppressionParams
	numAnalyzedFrames   int32
	speechProbability   *speechProbabilityEstimator
	wiener              *wienerFilter
	noise               *noiseEstimator
	prevAnalysisSignal  espectro
	analyzeMemory       [kOverlapSize]float32
	processAnalysisMem  [kOverlapSize]float32
	processSynthesisMem [kOverlapSize]float32
	processDelayMemory  [kNumBands - 1][kOverlapSize]float32
	bandas              threeBandFilterBank
}

func NuevoSupresor(l Nivel) *Supresor {
	s := &Supresor{params: newSuppressionParams(l), numAnalyzedFrames: -1}
	s.speechProbability = newSpeechProbabilityEstimator()
	s.wiener = newWienerFilter(&s.params)
	s.noise = newNoiseEstimator(&s.params)
	for i := range s.prevAnalysisSignal {
		s.prevAnalysisSignal[i] = 1
	}
	return s
}

func (s *Supresor) analyze(y0 *[kNsFrameSize]float32) {
	s.noise.prepareAnalysis()
	if energyParts(y0, &s.analyzeMemory) <= 0 {
		return
	}
	if s.numAnalyzedFrames++; s.numAnalyzedFrames < 0 {
		s.numAnalyzedFrames = 0
	}
	var ext [kFftSize]float32
	formExtendedFrame(y0, &s.analyzeMemory, &ext)
	applyFilterBankWindow(&ext)
	var re, im [kFftSize]float32
	fftReal(&ext, &re, &im)
	var signal espectro
	computeMagnitudeSpectrum(&re, &im, &signal)
	var signalEnergy float32
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		signalEnergy += re[i]*re[i] + im[i]*im[i]
	}
	signalEnergy /= kFftSizeBy2Plus1
	var signalSpectralSum float32
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		signalSpectralSum += signal[i]
	}
	s.noise.preUpdate(s.numAnalyzedFrames, &signal, signalSpectralSum)
	var postSnr, priorSnr espectro
	computeSnr(&s.wiener.filter, &s.prevAnalysisSignal, &signal, &s.noise.prevNoiseSpectrum, &s.noise.noiseSpectrum, &priorSnr, &postSnr)
	s.speechProbability.update(s.numAnalyzedFrames, &priorSnr, &postSnr, &s.noise.conservativeNoiseSpectrum, &signal, signalSpectralSum, signalEnergy)
	s.noise.postUpdate(s.speechProbability.speechProbability[:], &signal)
	s.prevAnalysisSignal = signal
}

func (s *Supresor) process(bands *[kNumBands][kSplitBandSize]float32) {
	y0 := &bands[0]
	var ext, re, im [kFftSize]float32
	formExtendedFrame(y0, &s.processAnalysisMem, &ext)
	applyFilterBankWindow(&ext)
	energyBefore := energyExt(&ext)
	fftReal(&ext, &re, &im)
	var signal espectro
	computeMagnitudeSpectrum(&re, &im, &signal)
	s.wiener.update(s.numAnalyzedFrames, &s.noise.noiseSpectrum, &s.noise.prevNoiseSpectrum, &s.noise.parametricNoiseSpectrum, &signal)
	upperBandGain := computeUpperBandsGain(s.params.minimumAttenuatingGain, &s.wiener.filter, &s.speechProbability.speechProbability, &s.prevAnalysisSignal, &signal)
	filter := &s.wiener.filter
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		re[i] *= filter[i]
		im[i] *= filter[i]
	}
	ifftReal(&re, &im, &ext)
	energyAfter := energyExt(&ext)
	applyFilterBankWindow(&ext)
	gainAdjustment := s.wiener.computeOverallScalingFactor(s.numAnalyzedFrames, s.speechProbability.priorSpeechProb, energyBefore, energyAfter)
	for i := range ext {
		ext[i] = gainAdjustment * ext[i]
	}
	overlapAndAdd(&ext, &s.processSynthesisMem, y0)
	for b := 1; b < kNumBands; b++ {
		var delayed [kNsFrameSize]float32
		delaySignal(&bands[b], &s.processDelayMemory[b-1], &delayed)
		for j := 0; j < kNsFrameSize; j++ {
			bands[b][j] = upperBandGain * delayed[j]
		}
	}
	for b := 0; b < kNumBands; b++ {
		for j := 0; j < kNsFrameSize; j++ {
			bands[b][j] = min(max(bands[b][j], -32768), 32767)
		}
	}
}

// Procesar suppresses the noise of one 10 ms frame at 48 kHz (480 samples, int16 scale) in place: split into three
// bands, analyze, process, merge, as the audio processing module does around the suppressor.
func (s *Supresor) Procesar(x *[KFullBandSize]float32) {
	var bands [kNumBands][kSplitBandSize]float32
	s.bandas.analysis(x, &bands)
	s.analyze(&bands[0])
	s.process(&bands)
	s.bandas.synthesis(&bands, x)
}
