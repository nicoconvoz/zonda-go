package webrtcns

/* ---------- wiener_filter.cc ---------- */

type wienerFilter struct {
	params                  *suppressionParams
	spectrumPrevProcess     espectro
	initialSpectralEstimate espectro
	filter                  espectro
}

func newWienerFilter(p *suppressionParams) *wienerFilter {
	w := &wienerFilter{params: p}
	for i := range w.filter {
		w.filter[i] = 1
	}
	return w
}

func (w *wienerFilter) update(numAnalyzedFrames int32, noise, prevNoise, parametricNoise, signal *espectro) {
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		prevTsa := w.spectrumPrevProcess[i] / (prevNoise[i] + 0.0001) * w.filter[i]
		var currentTsa float32
		if signal[i] > noise[i] {
			currentTsa = signal[i]/(noise[i]+0.0001) - 1
		}
		snrPrior := 0.98*prevTsa + (1-0.98)*currentTsa
		w.filter[i] = snrPrior / (w.params.overSubtractionFactor + snrPrior)
		w.filter[i] = max(min(w.filter[i], 1), w.params.minimumAttenuatingGain)
	}
	if numAnalyzedFrames < kShortStartupPhaseBlocks {
		for i := 0; i < kFftSizeBy2Plus1; i++ {
			w.initialSpectralEstimate[i] += signal[i]
			filterInitial := w.initialSpectralEstimate[i] - w.params.overSubtractionFactor*parametricNoise[i]
			filterInitial /= w.initialSpectralEstimate[i] + 0.0001
			filterInitial = max(min(filterInitial, 1), w.params.minimumAttenuatingGain)
			const kOnyByShortStartupPhaseBlocks = float32(1) / kShortStartupPhaseBlocks
			filterInitial *= kShortStartupPhaseBlocks - float32(numAnalyzedFrames)
			w.filter[i] *= float32(numAnalyzedFrames)
			w.filter[i] += filterInitial
			w.filter[i] *= kOnyByShortStartupPhaseBlocks
		}
	}
	w.spectrumPrevProcess = *signal
}

func (w *wienerFilter) computeOverallScalingFactor(numAnalyzedFrames int32, priorSpeechProbability, energyBefore, energyAfter float32) float32 {
	if !w.params.useAttenuationAdjustment || numAnalyzedFrames <= kLongStartupPhaseBlocks {
		return 1
	}
	gain := sqrtFastApproximation(energyAfter / (energyBefore + 1))
	const kBLim = float32(0.5)
	scaleFactor1 := float32(1)
	if gain > kBLim {
		scaleFactor1 = 1 + 1.3*(gain-kBLim)
		if gain*scaleFactor1 > 1 {
			scaleFactor1 = 1 / gain
		}
	}
	scaleFactor2 := float32(1)
	if gain < kBLim {
		gain = max(gain, w.params.minimumAttenuatingGain)
		scaleFactor2 = 1 - 0.3*(kBLim-gain)
	}
	return priorSpeechProbability*scaleFactor1 + (1-priorSpeechProbability)*scaleFactor2
}
