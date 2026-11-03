package webrtcns

import "math"

func fabs(x float32) float32  { return float32(math.Abs(float64(x))) }
func tanhf(x float32) float32 { return float32(math.Tanh(float64(x))) }

/* ---------- prior_signal_model_estimator.cc ---------- */

func findFirstOfTwoLargestPeaks(binSize float32, hist *[kHistogramSize]int) (peakPosition float32, peakWeight int) {
	peakValue, secondaryPeakValue := 0, 0
	var secondaryPeakPosition float32
	secondaryPeakWeight := 0
	for i := 0; i < kHistogramSize; i++ {
		binMid := (float32(i) + 0.5) * binSize
		if hist[i] > peakValue {
			secondaryPeakValue, secondaryPeakWeight, secondaryPeakPosition = peakValue, peakWeight, peakPosition
			peakValue, peakWeight, peakPosition = hist[i], hist[i], binMid
		} else if hist[i] > secondaryPeakValue {
			secondaryPeakValue, secondaryPeakWeight, secondaryPeakPosition = hist[i], hist[i], binMid
		}
	}
	if fabs(secondaryPeakPosition-peakPosition) < 2*binSize && float32(secondaryPeakWeight) > 0.5*float32(peakWeight) {
		peakWeight += secondaryPeakWeight
		peakPosition = 0.5 * (peakPosition + secondaryPeakPosition)
	}
	return
}

func updateLrt(lrtHistogram *[kHistogramSize]int, priorModelLrt *float32) (lowLrtFluctuations bool) {
	var average, averageCompl, averageSquared float32
	count := 0
	for i := 0; i < 10; i++ {
		binMid := (float32(i) + 0.5) * kBinSizeLrt
		average += float32(lrtHistogram[i]) * binMid
		count += lrtHistogram[i]
	}
	if count > 0 {
		average = average / float32(count)
	}
	for i := 0; i < kHistogramSize; i++ {
		binMid := (float32(i) + 0.5) * kBinSizeLrt
		averageSquared += float32(lrtHistogram[i]) * binMid * binMid
		averageCompl += float32(lrtHistogram[i]) * binMid
	}
	const kOneFeatureUpdateWindowSize = float32(1) / kFeatureUpdateWindowSize
	averageSquared = averageSquared * kOneFeatureUpdateWindowSize
	averageCompl = averageCompl * kOneFeatureUpdateWindowSize
	lowLrtFluctuations = averageSquared-average*averageCompl < 0.05
	const kMaxLrt, kMinLrt = float32(1), float32(.2)
	if lowLrtFluctuations {
		*priorModelLrt = kMaxLrt
	} else {
		*priorModelLrt = min(kMaxLrt, max(kMinLrt, 1.2*average))
	}
	return
}

type priorSignalModelEstimator struct{ prior priorSignalModel }

func (p *priorSignalModelEstimator) update(h *histograms) {
	low := updateLrt(&h.lrt, &p.prior.lrt)
	sfPos, sfWeight := findFirstOfTwoLargestPeaks(kBinSizeSpecFlat, &h.spectralFlatness)
	sdPos, sdWeight := findFirstOfTwoLargestPeaks(kBinSizeSpecDiff, &h.spectralDiff)
	useSpecFlat := 1
	if float32(sfWeight) < 0.3*500 || sfPos < 0.6 {
		useSpecFlat = 0
	}
	useSpecDiff := 1
	if float32(sdWeight) < 0.3*500 || low {
		useSpecDiff = 0
	}
	p.prior.templateDiffThreshold = 1.2 * sdPos
	p.prior.templateDiffThreshold = min(1, max(0.16, p.prior.templateDiffThreshold))
	oneByFeatureSum := 1 / (1 + float32(useSpecFlat) + float32(useSpecDiff))
	p.prior.lrtWeighting = oneByFeatureSum
	if useSpecFlat == 1 {
		p.prior.flatnessThreshold = 0.9 * sfPos
		p.prior.flatnessThreshold = min(.95, max(0.1, p.prior.flatnessThreshold))
		p.prior.flatnessWeighting = oneByFeatureSum
	} else {
		p.prior.flatnessWeighting = 0
	}
	if useSpecDiff == 1 {
		p.prior.differenceWeighting = oneByFeatureSum
	} else {
		p.prior.differenceWeighting = 0
	}
}

/* ---------- signal_model_estimator.cc ---------- */

const kOneByFftSizeBy2Plus1 = float32(1) / kFftSizeBy2Plus1

func computeSpectralDiff(conservativeNoise, signal *espectro, signalSpectralSum, diffNormalization float32) float32 {
	var noiseAverage float32
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		noiseAverage += conservativeNoise[i]
	}
	noiseAverage = noiseAverage * kOneByFftSizeBy2Plus1
	signalAverage := signalSpectralSum * kOneByFftSizeBy2Plus1
	var covariance, noiseVariance, signalVariance float32
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		signalDiff := signal[i] - signalAverage
		noiseDiff := conservativeNoise[i] - noiseAverage
		covariance += signalDiff * noiseDiff
		noiseVariance += noiseDiff * noiseDiff
		signalVariance += signalDiff * signalDiff
	}
	covariance *= kOneByFftSizeBy2Plus1
	noiseVariance *= kOneByFftSizeBy2Plus1
	signalVariance *= kOneByFftSizeBy2Plus1
	spectralDiff := signalVariance - (covariance*covariance)/(noiseVariance+0.0001)
	return spectralDiff / (diffNormalization + 0.0001)
}

func updateSpectralFlatness(signal *espectro, signalSpectralSum float32, spectralFlatness *float32) {
	const kAveraging = float32(0.3)
	var avgSpectFlatnessNum float32
	for i := 1; i < kFftSizeBy2Plus1; i++ {
		if signal[i] == 0 {
			*spectralFlatness -= kAveraging * (*spectralFlatness)
			return
		}
	}
	for i := 1; i < kFftSizeBy2Plus1; i++ {
		avgSpectFlatnessNum += logApproximation(signal[i])
	}
	avgSpectFlatnessDenom := signalSpectralSum - signal[0]
	avgSpectFlatnessDenom = avgSpectFlatnessDenom * kOneByFftSizeBy2Plus1
	avgSpectFlatnessNum = avgSpectFlatnessNum * kOneByFftSizeBy2Plus1
	spectralTmp := expApproximation(avgSpectFlatnessNum) / avgSpectFlatnessDenom
	*spectralFlatness += kAveraging * (spectralTmp - *spectralFlatness)
}

func updateSpectralLrt(priorSnr, postSnr *espectro, avgLogLrt *espectro, lrt *float32) {
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		tmp1 := 1 + 2*priorSnr[i]
		tmp2 := 2 * priorSnr[i] / (tmp1 + 0.0001)
		besselTmp := (postSnr[i] + 1) * tmp2
		avgLogLrt[i] += .5 * (besselTmp - logApproximation(tmp1) - avgLogLrt[i])
	}
	var sum float32
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		sum += avgLogLrt[i]
	}
	*lrt = sum * kOneByFftSizeBy2Plus1
}

type signalModelEstimator struct {
	diffNormalization        float32
	signalEnergySum          float32
	histograms               histograms
	histogramAnalysisCounter int
	priorModelEstimator      priorSignalModelEstimator
	features                 signalModel
}

func newSignalModelEstimator() *signalModelEstimator {
	return &signalModelEstimator{histogramAnalysisCounter: 500, priorModelEstimator: priorSignalModelEstimator{newPriorSignalModel(kLtrFeatureThr)}, features: newSignalModel()}
}

func (s *signalModelEstimator) adjustNormalization(numAnalyzedFrames int32, signalEnergy float32) {
	s.diffNormalization *= float32(numAnalyzedFrames)
	s.diffNormalization += signalEnergy
	s.diffNormalization /= float32(numAnalyzedFrames + 1)
}

func (s *signalModelEstimator) update(priorSnr, postSnr, conservativeNoise, signal *espectro, signalSpectralSum, signalEnergy float32) {
	updateSpectralFlatness(signal, signalSpectralSum, &s.features.spectralFlatness)
	spectralDiff := computeSpectralDiff(conservativeNoise, signal, signalSpectralSum, s.diffNormalization)
	s.features.spectralDiff += 0.3 * (spectralDiff - s.features.spectralDiff)
	s.signalEnergySum += signalEnergy
	s.histogramAnalysisCounter--
	if s.histogramAnalysisCounter > 0 {
		s.histograms.update(&s.features)
	} else {
		s.priorModelEstimator.update(&s.histograms)
		s.histograms.clear()
		s.histogramAnalysisCounter = kFeatureUpdateWindowSize
		s.signalEnergySum = s.signalEnergySum / kFeatureUpdateWindowSize
		s.diffNormalization = 0.5 * (s.signalEnergySum + s.diffNormalization)
		s.signalEnergySum = 0
	}
	updateSpectralLrt(priorSnr, postSnr, &s.features.avgLogLrt, &s.features.lrt)
}

/* ---------- speech_probability_estimator.cc ---------- */

type speechProbabilityEstimator struct {
	signalModel       *signalModelEstimator
	priorSpeechProb   float32
	speechProbability espectro
}

func newSpeechProbabilityEstimator() *speechProbabilityEstimator {
	return &speechProbabilityEstimator{signalModel: newSignalModelEstimator(), priorSpeechProb: .5}
}

func (e *speechProbabilityEstimator) update(numAnalyzedFrames int32, priorSnr, postSnr, conservativeNoise, signal *espectro, signalSpectralSum, signalEnergy float32) {
	if numAnalyzedFrames < kLongStartupPhaseBlocks {
		e.signalModel.adjustNormalization(numAnalyzedFrames, signalEnergy)
	}
	e.signalModel.update(priorSnr, postSnr, conservativeNoise, signal, signalSpectralSum, signalEnergy)
	model := &e.signalModel.features
	prior := &e.signalModel.priorModelEstimator.prior
	const kWidthPrior0 = float32(4)
	const kWidthPrior1 = 2 * kWidthPrior0
	widthPrior := kWidthPrior0
	if model.lrt < prior.lrt {
		widthPrior = kWidthPrior1
	}
	indicator0 := 0.5 * (tanhf(widthPrior*(model.lrt-prior.lrt)) + 1)
	widthPrior = kWidthPrior0
	if model.spectralFlatness > prior.flatnessThreshold {
		widthPrior = kWidthPrior1
	}
	indicator1 := 0.5 * (tanhf(1*widthPrior*(prior.flatnessThreshold-model.spectralFlatness)) + 1)
	widthPrior = kWidthPrior0
	if model.spectralDiff < prior.templateDiffThreshold {
		widthPrior = kWidthPrior1
	}
	indicator2 := 0.5 * (tanhf(widthPrior*(model.spectralDiff-prior.templateDiffThreshold)) + 1)
	indPrior := prior.lrtWeighting*indicator0 + prior.flatnessWeighting*indicator1 + prior.differenceWeighting*indicator2
	e.priorSpeechProb += 0.1 * (indPrior - e.priorSpeechProb)
	e.priorSpeechProb = max(min(e.priorSpeechProb, 1), 0.01)
	gainPrior := (1 - e.priorSpeechProb) / (e.priorSpeechProb + 0.0001)
	for i := 0; i < kFftSizeBy2Plus1; i++ {
		invLrt := expApproximation(-model.avgLogLrt[i])
		e.speechProbability[i] = 1 / (1 + gainPrior*invLrt)
	}
}
