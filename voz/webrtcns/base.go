// Package webrtcns is WebRTC's noise suppressor (modules/audio_processing/ns), translated to Go line by line: the
// same constants, the same estimators and the same order of operations, in float32 as the original. Names follow the
// original files so each piece can be checked against its source.
package webrtcns

import (
	"math"
)

/* ---------- ns_common.h ---------- */

const (
	kFftSize                 = 256
	kFftSizeBy2Plus1         = kFftSize/2 + 1
	kNsFrameSize             = 160
	kOverlapSize             = kFftSize - kNsFrameSize
	kShortStartupPhaseBlocks = 50
	kLongStartupPhaseBlocks  = 200
	kFeatureUpdateWindowSize = 500
	kLtrFeatureThr           = float32(0.5)
	kBinSizeLrt              = float32(0.1)
	kBinSizeSpecFlat         = float32(0.05)
	kBinSizeSpecDiff         = float32(0.1)
)

type espectro = [kFftSizeBy2Plus1]float32

/* ---------- ns_config.h, suppression_params.cc ---------- */

// Nivel is NsConfig::SuppressionLevel.
type Nivel int

const (
	K6dB Nivel = iota
	K12dB
	K18dB
	K21dB
)

type suppressionParams struct {
	overSubtractionFactor    float32
	minimumAttenuatingGain   float32
	useAttenuationAdjustment bool
}

func newSuppressionParams(l Nivel) suppressionParams {
	switch l {
	case K6dB:
		return suppressionParams{1, 0.5, false}
	case K18dB:
		return suppressionParams{1.1, 0.125, true}
	case K21dB:
		return suppressionParams{1.25, 0.09, true}
	}
	return suppressionParams{1, 0.25, true}
}

/* ---------- fast_math.cc ---------- */

func fastLog2f(in float32) float32 {
	out := float32(math.Float32bits(in))
	out *= 1.1920929e-7 // 1/2^23
	out -= 126.942695   // Remove bias.
	return out
}

func sqrtFastApproximation(f float32) float32 { return float32(math.Sqrt(float64(f))) }
func pow2Approximation(p float32) float32     { return float32(math.Pow(2, float64(p))) }
func powApproximation(x, p float32) float32   { return pow2Approximation(p * fastLog2f(x)) }

const ln2 = float32(math.Ln2)
const log10e = float32(math.Log10E)

func logApproximation(x float32) float32 { return fastLog2f(x) * ln2 }
func expApproximation(x float32) float32 { return powApproximation(10, x*log10e) }

/* ---------- signal_model.cc, prior_signal_model.cc ---------- */

type signalModel struct {
	lrt              float32
	spectralDiff     float32
	spectralFlatness float32
	avgLogLrt        espectro
}

func newSignalModel() signalModel {
	const kSfFeatureThr = float32(0.5)
	m := signalModel{lrt: kLtrFeatureThr, spectralFlatness: kSfFeatureThr, spectralDiff: kSfFeatureThr}
	for i := range m.avgLogLrt {
		m.avgLogLrt[i] = kLtrFeatureThr
	}
	return m
}

type priorSignalModel struct {
	lrt                   float32
	flatnessThreshold     float32
	templateDiffThreshold float32
	lrtWeighting          float32
	flatnessWeighting     float32
	differenceWeighting   float32
}

func newPriorSignalModel(lrtInitial float32) priorSignalModel {
	return priorSignalModel{lrt: lrtInitial, flatnessThreshold: .5, templateDiffThreshold: .5, lrtWeighting: 1}
}

/* ---------- histograms.cc ---------- */

const kHistogramSize = 1000

type histograms struct {
	lrt, spectralFlatness, spectralDiff [kHistogramSize]int
}

func (h *histograms) clear() { *h = histograms{} }

func (h *histograms) update(f *signalModel) {
	const kOneByBinSizeLrt = 1 / kBinSizeLrt
	if f.lrt < kHistogramSize*kBinSizeLrt && f.lrt >= 0 {
		h.lrt[int(kOneByBinSizeLrt*f.lrt)]++
	}
	const kOneByBinSizeSpecFlat = 1 / kBinSizeSpecFlat
	if f.spectralFlatness < kHistogramSize*kBinSizeSpecFlat && f.spectralFlatness >= 0 {
		h.spectralFlatness[int(f.spectralFlatness*kOneByBinSizeSpecFlat)]++
	}
	const kOneByBinSizeSpecDiff = 1 / kBinSizeSpecDiff
	if f.spectralDiff < kHistogramSize*kBinSizeSpecDiff && f.spectralDiff >= 0 {
		h.spectralDiff[int(f.spectralDiff*kOneByBinSizeSpecDiff)]++
	}
}

/* ---------- quantile_noise_estimator.cc ---------- */

const kSimult = 3

type quantileNoiseEstimator struct {
	density     [kSimult * kFftSizeBy2Plus1]float32
	logQuantile [kSimult * kFftSizeBy2Plus1]float32
	quantile    espectro
	counter     [kSimult]int
	numUpdates  int
}

func newQuantileNoiseEstimator() *quantileNoiseEstimator {
	q := &quantileNoiseEstimator{numUpdates: 1}
	for i := range q.density {
		q.density[i] = 0.3
		q.logQuantile[i] = 8
	}
	const kOneBySimult = float32(1) / kSimult
	for i := 0; i < kSimult; i++ {
		q.counter[i] = int(math.Floor(float64(kLongStartupPhaseBlocks * (float32(i) + 1) * kOneBySimult)))
	}
	return q
}

func (q *quantileNoiseEstimator) estimate(signal *espectro, noise *espectro) {
	var logSpectrum espectro
	for i := range signal {
		logSpectrum[i] = logApproximation(signal[i])
	}
	quantileIndexToReturn := -1
	for s, k := 0, 0; s < kSimult; s, k = s+1, k+kFftSizeBy2Plus1 {
		oneByCounterPlus1 := 1 / (float32(q.counter[s]) + 1)
		for i, j := 0, k; i < kFftSizeBy2Plus1; i, j = i+1, j+1 {
			delta := float32(40)
			if q.density[j] > 1 {
				delta = 40 / q.density[j]
			}
			multiplier := delta * oneByCounterPlus1
			if logSpectrum[i] > q.logQuantile[j] {
				q.logQuantile[j] += 0.25 * multiplier
			} else {
				q.logQuantile[j] -= 0.75 * multiplier
			}
			const kWidth = float32(0.01)
			const kOneByWidthPlus2 = 1 / (2 * kWidth)
			if float32(math.Abs(float64(logSpectrum[i]-q.logQuantile[j]))) < kWidth {
				q.density[j] = (float32(q.counter[s])*q.density[j] + kOneByWidthPlus2) * oneByCounterPlus1
			}
		}
		if q.counter[s] >= kLongStartupPhaseBlocks {
			q.counter[s] = 0
			if q.numUpdates >= kLongStartupPhaseBlocks {
				quantileIndexToReturn = k
			}
		}
		q.counter[s]++
	}
	if q.numUpdates < kLongStartupPhaseBlocks {
		quantileIndexToReturn = kFftSizeBy2Plus1 * (kSimult - 1)
		q.numUpdates++
	}
	if quantileIndexToReturn >= 0 {
		for i := 0; i < kFftSizeBy2Plus1; i++ {
			q.quantile[i] = expApproximation(q.logQuantile[quantileIndexToReturn+i])
		}
	}
	*noise = q.quantile
}

/* ---------- noise_estimator.cc ---------- */

var logTable = [129]float32{
	0, 0, 0, 0, 0, 1.609438, 1.791759,
	1.945910, 2.079442, 2.197225, float32(math.Ln10), 2.397895, 2.484907,
	2.564949,
	2.639057, 2.708050, 2.772589, 2.833213, 2.890372, 2.944439, 2.995732,
	3.044522, 3.091043, 3.135494, 3.178054, 3.218876, 3.258097, 3.295837,
	3.332205, 3.367296, 3.401197, 3.433987, 3.465736, 3.496507, 3.526361,
	3.555348, 3.583519, 3.610918, 3.637586, 3.663562, 3.688879, 3.713572,
	3.737669, 3.761200, 3.784190, 3.806663, 3.828641, 3.850147, 3.871201,
	3.891820, 3.912023, 3.931826, 3.951244, 3.970292, 3.988984, 4.007333,
	4.025352, 4.043051, 4.060443, 4.077538, 4.094345, 4.110874, 4.127134,
	4.143135, 4.158883, 4.174387, 4.189655, 4.204693, 4.219508, 4.234107,
	4.248495, 4.262680, 4.276666, 4.290460, 4.304065, 4.317488, 4.330733,
	4.343805, 4.356709, 4.369448, 4.382027, 4.394449, 4.406719, 4.418841,
	4.430817, 4.442651, 4.454347, 4.465908, 4.477337, 4.488636, 4.499810,
	4.510859, 4.521789, 4.532599, 4.543295, 4.553877, 4.564348, 4.574711,
	4.584968, 4.595119, 4.605170, 4.615121, 4.624973, 4.634729, 4.644391,
	4.653960, 4.663439, 4.672829, 4.682131, 4.691348, 4.700480, 4.709530,
	4.718499, 4.727388, 4.736198, 4.744932, 4.753591, 4.762174, 4.770685,
	4.779124, 4.787492, 4.795791, 4.804021, 4.812184, 4.820282, 4.828314,
	4.836282, 4.844187, 4.852030,
}

type noiseEstimator struct {
	params                    *suppressionParams
	whiteNoiseLevel           float32
	pinkNoiseNumerator        float32
	pinkNoiseExp              float32
	prevNoiseSpectrum         espectro
	conservativeNoiseSpectrum espectro
	parametricNoiseSpectrum   espectro
	noiseSpectrum             espectro
	quantile                  *quantileNoiseEstimator
}

func newNoiseEstimator(p *suppressionParams) *noiseEstimator {
	return &noiseEstimator{params: p, quantile: newQuantileNoiseEstimator()}
}

func (n *noiseEstimator) prepareAnalysis() { n.prevNoiseSpectrum = n.noiseSpectrum }

func (n *noiseEstimator) preUpdate(numAnalyzedFrames int32, signal *espectro, signalSpectralSum float32) {
	n.quantile.estimate(signal, &n.noiseSpectrum)
	if numAnalyzedFrames >= kShortStartupPhaseBlocks {
		return
	}
	const kStartBand = 5
	var sumLogILogMagn, sumLogI, sumLogISquare, sumLogMagn float32
	for i := kStartBand; i < kFftSizeBy2Plus1; i++ {
		logI := logTable[i]
		sumLogI += logI
		sumLogISquare += logI * logI
		logSignal := logApproximation(signal[i])
		sumLogMagn += logSignal
		sumLogILogMagn += logI * logSignal
	}
	const kOneByFftSizeBy2Plus1 = float32(1) / kFftSizeBy2Plus1
	n.whiteNoiseLevel += signalSpectralSum * kOneByFftSizeBy2Plus1 * n.params.overSubtractionFactor
	denom := sumLogISquare*(kFftSizeBy2Plus1-kStartBand) - sumLogI*sumLogI
	num := sumLogISquare*sumLogMagn - sumLogI*sumLogILogMagn
	pinkNoiseAdjustment := max(num/denom, 0)
	n.pinkNoiseNumerator += pinkNoiseAdjustment
	num = sumLogI*sumLogMagn - (kFftSizeBy2Plus1-kStartBand)*sumLogILogMagn
	pinkNoiseAdjustment = max(min(num/denom, 1), 0)
	n.pinkNoiseExp += pinkNoiseAdjustment
	oneByNumAnalyzedFramesPlus1 := 1 / (float32(numAnalyzedFrames) + 1)
	var parametricExp, parametricNum float32
	if n.pinkNoiseExp > 0 {
		parametricNum = expApproximation(n.pinkNoiseNumerator * oneByNumAnalyzedFramesPlus1)
		parametricNum *= float32(numAnalyzedFrames) + 1
		parametricExp = n.pinkNoiseExp * oneByNumAnalyzedFramesPlus1
	}
	const kOneByShortStartupPhaseBlocks = float32(1) / kShortStartupPhaseBlocks
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		if n.pinkNoiseExp == 0 {
			n.parametricNoiseSpectrum[i] = n.whiteNoiseLevel
		} else {
			useBand := float32(i)
			if i < kStartBand {
				useBand = kStartBand
			}
			n.parametricNoiseSpectrum[i] = parametricNum / powApproximation(useBand, parametricExp)
		}
	}
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		n.noiseSpectrum[i] *= float32(numAnalyzedFrames)
		tmp := n.parametricNoiseSpectrum[i] * (kShortStartupPhaseBlocks - float32(numAnalyzedFrames))
		n.noiseSpectrum[i] += tmp * oneByNumAnalyzedFramesPlus1
		n.noiseSpectrum[i] *= kOneByShortStartupPhaseBlocks
	}
}

func (n *noiseEstimator) postUpdate(speechProbability []float32, signal *espectro) {
	const kNoiseUpdate = float32(0.9)
	gamma := kNoiseUpdate
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		probSpeech := speechProbability[i]
		probNonSpeech := 1 - probSpeech
		noiseUpdateTmp := gamma*n.prevNoiseSpectrum[i] + (1-gamma)*(probNonSpeech*signal[i]+probSpeech*n.prevNoiseSpectrum[i])
		gammaOld := gamma
		const kProbRange = float32(.2)
		if probSpeech > kProbRange {
			gamma = .99
		} else {
			gamma = kNoiseUpdate
		}
		if probSpeech < kProbRange {
			n.conservativeNoiseSpectrum[i] += 0.05 * (signal[i] - n.conservativeNoiseSpectrum[i])
		}
		if gamma == gammaOld {
			n.noiseSpectrum[i] = noiseUpdateTmp
		} else {
			n.noiseSpectrum[i] = gamma*n.prevNoiseSpectrum[i] + (1-gamma)*(probNonSpeech*signal[i]+probSpeech*n.prevNoiseSpectrum[i])
			n.noiseSpectrum[i] = min(n.noiseSpectrum[i], noiseUpdateTmp)
		}
	}
}
