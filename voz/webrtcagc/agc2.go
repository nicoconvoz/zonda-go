// Package webrtcagc is WebRTC's AGC2 (modules/audio_processing/agc2, gain_controller2.cc) translated to Go line by
// line, with its default configuration (api/audio/audio_processing.h, GainController2::AdaptiveDigital): adaptive
// digital gain driven by a voice activity detector, a speech level estimator, a noise floor estimator and a
// saturation protector, then the limiter. In float32 as the original; names follow the original files.
package webrtcagc

import "math"

/* ---------- agc2_common.h ---------- */

const (
	kMinFloatS16Value                      = float32(-32768)
	kMaxFloatS16Value                      = float32(32767)
	kMaxAbsFloatS16Value                   = float32(32768)
	kMinLevelDbfs                          = float32(-90.31)
	kFrameDurationMs                       = 10
	kSubFramesInFrame                      = 20
	kLimiterThresholdForAgcGainDbfs        = float32(-1)
	kVadResetPeriodMs                      = 1500
	kVadConfidenceThreshold                = float32(0.95)
	kAdjacentSpeechFramesThreshold         = 12
	kLevelEstimatorTimeToConfidenceMs      = 400
	kLevelEstimatorLeakFactor              = float32(1) - float32(1)/kLevelEstimatorTimeToConfidenceMs
	kSaturationProtectorInitialHeadroomDb  = float32(20)
	kSaturationProtectorBufferSize         = 4
	kInterpolatedGainCurveKneePoints       = 22
	kInterpolatedGainCurveBeyondKneePoints = 10
	kInterpolatedGainCurveTotalPoints      = kInterpolatedGainCurveKneePoints + kInterpolatedGainCurveBeyondKneePoints
)

// Config is GainController2::AdaptiveDigital with WebRTC's defaults.
type Config struct {
	HeadroomDb               float32
	MaxGainDb                float32
	InitialGainDb            float32
	MaxGainChangeDbPerSecond float32
	MaxOutputNoiseLevelDbfs  float32
}

var Defecto = Config{HeadroomDb: 5, MaxGainDb: 50, InitialGainDb: 15, MaxGainChangeDbPerSecond: 6, MaxOutputNoiseLevelDbfs: -50}

func dbToRatio(db float32) float32 { return float32(math.Pow(10, float64(db)/20)) }

// floatS16ToDbfs: common_audio/include/audio_util.h.
func floatS16ToDbfs(v float32) float32 {
	const kMinDbfs = float32(-90.30899869919436)
	if v <= 1 {
		return kMinDbfs
	}
	return 20*float32(math.Log10(float64(v))) + kMinDbfs
}

func clamp(v, lo, hi float32) float32 { return max(lo, min(hi, v)) }

/* ---------- speech_level_estimator_impl.cc ---------- */

type ratio struct{ numerator, denominator float32 }

func (r ratio) get() float32 { return r.numerator / r.denominator }

type levelEstimatorState struct {
	timeToConfidenceMs int
	levelDbfs          ratio
}

type speechLevelEstimator struct {
	initialSpeechLevelDbfs        float32
	adjacentSpeechFramesThreshold int
	preliminary, reliable         levelEstimatorState
	levelDbfs                     float32
	isConfident                   bool
	numAdjacentSpeechFrames       int
}

func clampLevelEstimateDbfs(v float32) float32 { return clamp(v, -90, 30) }

func newSpeechLevelEstimator(c Config, threshold int) *speechLevelEstimator {
	s := &speechLevelEstimator{adjacentSpeechFramesThreshold: threshold}
	s.initialSpeechLevelDbfs = clampLevelEstimateDbfs(-kSaturationProtectorInitialHeadroomDb - c.InitialGainDb - c.HeadroomDb)
	s.levelDbfs = s.initialSpeechLevelDbfs
	s.reset()
	return s
}

func (s *speechLevelEstimator) resetState(st *levelEstimatorState) {
	st.timeToConfidenceMs = kLevelEstimatorTimeToConfidenceMs
	st.levelDbfs = ratio{s.initialSpeechLevelDbfs, 1}
}

func (s *speechLevelEstimator) reset() {
	s.resetState(&s.preliminary)
	s.resetState(&s.reliable)
	s.levelDbfs = s.initialSpeechLevelDbfs
	s.numAdjacentSpeechFrames = 0
}

func (s *speechLevelEstimator) update(rmsDbfs, speechProbability float32) {
	if speechProbability < kVadConfidenceThreshold {
		if s.adjacentSpeechFramesThreshold > 1 {
			if s.numAdjacentSpeechFrames >= s.adjacentSpeechFramesThreshold {
				s.reliable = s.preliminary
			} else if s.numAdjacentSpeechFrames > 0 {
				s.preliminary = s.reliable
			}
		}
		s.numAdjacentSpeechFrames = 0
	} else {
		s.numAdjacentSpeechFrames++
		bufferIsFull := s.preliminary.timeToConfidenceMs == 0
		if !bufferIsFull {
			s.preliminary.timeToConfidenceMs -= kFrameDurationMs
		}
		leak := float32(1)
		if bufferIsFull {
			leak = kLevelEstimatorLeakFactor
		}
		s.preliminary.levelDbfs.numerator = s.preliminary.levelDbfs.numerator*leak + rmsDbfs*speechProbability
		s.preliminary.levelDbfs.denominator = s.preliminary.levelDbfs.denominator*leak + speechProbability
		level := s.preliminary.levelDbfs.get()
		if s.numAdjacentSpeechFrames >= s.adjacentSpeechFramesThreshold {
			s.levelDbfs = clampLevelEstimateDbfs(level)
		}
	}
	s.updateIsConfident()
}

func (s *speechLevelEstimator) updateIsConfident() {
	if s.adjacentSpeechFramesThreshold == 1 {
		s.isConfident = s.preliminary.timeToConfidenceMs == 0
		return
	}
	s.isConfident = s.reliable.timeToConfidenceMs == 0 ||
		(s.numAdjacentSpeechFrames >= s.adjacentSpeechFramesThreshold && s.preliminary.timeToConfidenceMs == 0)
}

// The target speaker is active when the default estimator is used: it never classifies background speakers.
func (s *speechLevelEstimator) isTargetSpeakerActive() bool { return true }

/* ---------- noise_level_estimator.cc (NoiseFloorEstimator) ---------- */

const kFramesPerSecond = 100

func energyToDbfs(signalEnergy float32, numSamples int) float32 {
	rmsSquare := signalEnergy / float32(numSamples)
	const kMinDbfs = float32(-90.30899869919436)
	if rmsSquare <= 1 {
		return kMinDbfs
	}
	return 10*float32(math.Log10(float64(rmsSquare))) + kMinDbfs
}

func smoothNoiseFloorEstimate(current, next float32) float32 {
	const kAttack = float32(0.5)
	if current < next {
		return kAttack*next + (1-kAttack)*current
	}
	return next
}

type noiseFloorEstimator struct {
	sampleRateHz              int
	minNoiseEnergy            float32
	firstPeriod               bool
	preliminaryNoiseEnergySet bool
	preliminaryNoiseEnergy    float32
	noiseEnergy               float32
	counter                   int
}

const kUpdatePeriodNumFrames = 500

func newNoiseFloorEstimator() *noiseFloorEstimator {
	n := &noiseFloorEstimator{}
	n.initialize(48000)
	return n
}

func (n *noiseFloorEstimator) initialize(sampleRateHz int) {
	n.sampleRateHz = sampleRateHz
	n.firstPeriod = true
	n.preliminaryNoiseEnergySet = false
	n.minNoiseEnergy = float32(sampleRateHz) * 2 * 2 / kFramesPerSecond
	n.preliminaryNoiseEnergy = n.minNoiseEnergy
	n.noiseEnergy = n.minNoiseEnergy
	n.counter = kUpdatePeriodNumFrames
}

func (n *noiseFloorEstimator) analyze(frame []float32) float32 {
	if sr := len(frame) * kFramesPerSecond; sr != n.sampleRateHz {
		n.initialize(sr)
	}
	var frameEnergy float32
	for _, b := range frame {
		frameEnergy += b * b
	}
	if frameEnergy <= n.minNoiseEnergy {
		return energyToDbfs(n.noiseEnergy, len(frame))
	}
	if n.preliminaryNoiseEnergySet {
		n.preliminaryNoiseEnergy = min(n.preliminaryNoiseEnergy, frameEnergy)
	} else {
		n.preliminaryNoiseEnergy = frameEnergy
		n.preliminaryNoiseEnergySet = true
	}
	if n.counter == 0 {
		n.firstPeriod = false
		n.noiseEnergy = smoothNoiseFloorEstimate(n.noiseEnergy, n.preliminaryNoiseEnergy)
		n.counter = kUpdatePeriodNumFrames
		n.preliminaryNoiseEnergySet = false
	} else if n.firstPeriod {
		n.noiseEnergy = n.preliminaryNoiseEnergy
		n.counter--
	} else {
		n.noiseEnergy = min(n.noiseEnergy, n.preliminaryNoiseEnergy)
		n.counter--
	}
	return energyToDbfs(n.noiseEnergy, len(frame))
}

/* ---------- saturation_protector.cc, saturation_protector_buffer.cc ---------- */

type saturationProtectorBuffer struct {
	buffer     [kSaturationProtectorBufferSize]float32
	next, size int
}

func (b *saturationProtectorBuffer) reset() { b.next, b.size = 0, 0 }

func (b *saturationProtectorBuffer) pushBack(v float32) {
	b.buffer[b.next] = v
	b.next++
	if b.next == len(b.buffer) {
		b.next = 0
	}
	if b.size < len(b.buffer) {
		b.size++
	}
}

func (b *saturationProtectorBuffer) front() (float32, bool) {
	if b.size == 0 {
		return 0, false
	}
	i := 0
	if b.size == len(b.buffer) {
		i = b.next
	}
	return b.buffer[i], true
}

const (
	kPeakEnveloperSuperFrameLengthMs = 400
	kMinMarginDb                     = float32(12)
	kMaxMarginDb                     = float32(25)
	kSpAttack                        = float32(0.9988493699365052)
	kSpDecay                         = float32(0.9997697679981565)
)

type saturationProtectorState struct {
	headroomDb      float32
	peakDelayBuffer saturationProtectorBuffer
	maxPeaksDbfs    float32
	timeSincePushMs int
}

func resetSaturationProtectorState(initial float32, s *saturationProtectorState) {
	s.headroomDb = initial
	s.peakDelayBuffer.reset()
	s.maxPeaksDbfs = kMinLevelDbfs
	s.timeSincePushMs = 0
}

func updateSaturationProtectorState(peakDbfs, speechLevelDbfs float32, s *saturationProtectorState) {
	s.maxPeaksDbfs = max(s.maxPeaksDbfs, peakDbfs)
	s.timeSincePushMs += kFrameDurationMs
	if s.timeSincePushMs > kPeakEnveloperSuperFrameLengthMs {
		s.peakDelayBuffer.pushBack(s.maxPeaksDbfs)
		s.maxPeaksDbfs = kMinLevelDbfs
		s.timeSincePushMs = 0
	}
	delayedPeakDbfs, ok := s.peakDelayBuffer.front()
	if !ok {
		delayedPeakDbfs = s.maxPeaksDbfs
	}
	differenceDb := delayedPeakDbfs - speechLevelDbfs
	if differenceDb > s.headroomDb {
		s.headroomDb = s.headroomDb*kSpAttack + differenceDb*(1-kSpAttack)
	} else {
		s.headroomDb = s.headroomDb*kSpDecay + differenceDb*(1-kSpDecay)
	}
	s.headroomDb = clamp(s.headroomDb, kMinMarginDb, kMaxMarginDb)
}

type saturationProtector struct {
	initialHeadroomDb             float32
	adjacentSpeechFramesThreshold int
	numAdjacentSpeechFrames       int
	headroomDb                    float32
	preliminary, reliable         saturationProtectorState
}

func newSaturationProtector(initial float32, threshold int) *saturationProtector {
	s := &saturationProtector{initialHeadroomDb: initial, adjacentSpeechFramesThreshold: threshold}
	s.reset()
	return s
}

func (s *saturationProtector) reset() {
	s.numAdjacentSpeechFrames = 0
	s.headroomDb = s.initialHeadroomDb
	resetSaturationProtectorState(s.initialHeadroomDb, &s.preliminary)
	resetSaturationProtectorState(s.initialHeadroomDb, &s.reliable)
}

func (s *saturationProtector) analyze(speechProbability, peakDbfs, speechLevelDbfs float32) {
	if speechProbability < kVadConfidenceThreshold {
		if s.adjacentSpeechFramesThreshold > 1 {
			if s.numAdjacentSpeechFrames >= s.adjacentSpeechFramesThreshold {
				s.reliable = s.preliminary
			} else if s.numAdjacentSpeechFrames > 0 {
				s.preliminary = s.reliable
			}
		}
		s.numAdjacentSpeechFrames = 0
	} else {
		s.numAdjacentSpeechFrames++
		updateSaturationProtectorState(peakDbfs, speechLevelDbfs, &s.preliminary)
		if s.numAdjacentSpeechFrames >= s.adjacentSpeechFramesThreshold {
			s.headroomDb = s.preliminary.headroomDb
		}
	}
}

/* ---------- gain_applier.cc ---------- */

type gainApplier struct {
	hardClipSamples          bool
	lastGainFactor           float32
	currentGainFactor        float32
	samplesPerChannel        int
	inverseSamplesPerChannel float32
}

func gainCloseToOne(g float32) bool {
	return 1-1/kMaxFloatS16Value <= g && g <= 1+1/kMaxFloatS16Value
}

func (g *gainApplier) applyGain(signal []float32) {
	if len(signal) != g.samplesPerChannel {
		g.samplesPerChannel = len(signal)
		g.inverseSamplesPerChannel = 1 / float32(g.samplesPerChannel)
	}
	last, end := g.lastGainFactor, g.currentGainFactor
	switch {
	case last == end && gainCloseToOne(end):
	case last == end:
		for i := range signal {
			signal[i] *= end
		}
	default:
		increment := (end - last) * g.inverseSamplesPerChannel
		gain := last
		for i := range signal {
			signal[i] *= gain
			gain += increment
		}
	}
	g.lastGainFactor = g.currentGainFactor
	if g.hardClipSamples {
		for i := range signal {
			signal[i] = clamp(signal[i], kMinFloatS16Value, kMaxFloatS16Value)
		}
	}
}

/* ---------- adaptive_digital_gain_controller.cc ---------- */

func computeGainDb(inputLevelDbfs float32, c *Config) float32 {
	if inputLevelDbfs < -(c.HeadroomDb + c.MaxGainDb) {
		return c.MaxGainDb
	}
	if inputLevelDbfs < -c.HeadroomDb {
		return -c.HeadroomDb - inputLevelDbfs
	}
	return 0
}

func limitGainByNoise(targetGainDb, inputNoiseLevelDbfs, maxOutputNoiseLevelDbfs float32) float32 {
	maxAllowedGainDb := maxOutputNoiseLevelDbfs - inputNoiseLevelDbfs
	return min(targetGainDb, max(maxAllowedGainDb, 0))
}

func limitGainByLowConfidence(targetGainDb, lastGainDb, limiterAudioLevelDbfs float32, confident bool) float32 {
	if confident || limiterAudioLevelDbfs <= kLimiterThresholdForAgcGainDbfs {
		return targetGainDb
	}
	limiterLevelBeforeGain := limiterAudioLevelDbfs - lastGainDb
	newTarget := max(kLimiterThresholdForAgcGainDbfs-limiterLevelBeforeGain, 0)
	return min(newTarget, targetGainDb)
}

func computeGainChangeThisFrameDb(targetGainDb, lastGainDb float32, increaseAllowed bool, maxDecrease, maxIncrease float32) float32 {
	diff := targetGainDb - lastGainDb
	if !increaseAllowed {
		diff = min(diff, 0)
	}
	return clamp(diff, -maxDecrease, maxIncrease)
}

type frameInfo struct {
	speechProbability   float32
	speechLevelDbfs     float32
	speechLevelReliable bool
	noiseRmsDbfs        float32
	headroomDb          float32
	limiterEnvelopeDbfs float32
}

type adaptiveDigitalGainController struct {
	gainApplier                   gainApplier
	config                        Config
	adjacentSpeechFramesThreshold int
	maxGainChangeDbPer10ms        float32
	framesToGainIncreaseAllowed   int
	lastGainDb                    float32
}

func newAdaptiveDigitalGainController(c Config, threshold int) *adaptiveDigitalGainController {
	g := dbToRatio(c.InitialGainDb)
	return &adaptiveDigitalGainController{
		gainApplier: gainApplier{lastGainFactor: g, currentGainFactor: g}, config: c, adjacentSpeechFramesThreshold: threshold,
		maxGainChangeDbPer10ms: c.MaxGainChangeDbPerSecond * kFrameDurationMs / 1000, framesToGainIncreaseAllowed: threshold, lastGainDb: c.InitialGainDb,
	}
}

func (a *adaptiveDigitalGainController) process(info frameInfo, frame []float32) {
	inputLevelDbfs := info.speechLevelDbfs + info.headroomDb
	targetGainDb := limitGainByLowConfidence(
		limitGainByNoise(computeGainDb(inputLevelDbfs, &a.config), info.noiseRmsDbfs, a.config.MaxOutputNoiseLevelDbfs),
		a.lastGainDb, info.limiterEnvelopeDbfs, info.speechLevelReliable)
	firstConfidentSpeechFrame := false
	if info.speechProbability < kVadConfidenceThreshold {
		a.framesToGainIncreaseAllowed = a.adjacentSpeechFramesThreshold
	} else if a.framesToGainIncreaseAllowed > 0 {
		a.framesToGainIncreaseAllowed--
		firstConfidentSpeechFrame = a.framesToGainIncreaseAllowed == 0
	}
	gainIncreaseAllowed := a.framesToGainIncreaseAllowed == 0
	maxGainIncreaseDb := a.maxGainChangeDbPer10ms
	if firstConfidentSpeechFrame {
		maxGainIncreaseDb *= float32(a.adjacentSpeechFramesThreshold)
	}
	change := computeGainChangeThisFrameDb(targetGainDb, a.lastGainDb, gainIncreaseAllowed, a.maxGainChangeDbPer10ms, maxGainIncreaseDb)
	if change != 0 {
		a.gainApplier.currentGainFactor = dbToRatio(a.lastGainDb + change)
	}
	a.gainApplier.applyGain(frame)
	a.lastGainDb = a.lastGainDb + change
}
