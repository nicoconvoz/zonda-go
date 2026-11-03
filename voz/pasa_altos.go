package voz

// PasaAltos is WebRTC's capture high-pass filter, identical: the same three cascaded biquads for 48 kHz
// (modules/audio_processing/high_pass_filter.cc, kHighPassFilterCoefficients48kHz) and the same direct form I in
// float32 (common_audio CascadedBiQuadFilter). Its response, computed from the coefficients: −68 dB at 50 Hz (the
// mains hum), −13.6 dB at 80 Hz, −0.5 dB at 100 Hz, flat above.
type PasaAltos struct{ s [3]seccion }

type seccion struct {
	b    [3]float32
	a    [2]float32
	x, y [2]float32
}

var coefAltos48k = [3]struct {
	b [3]float32
	a [2]float32
}{
	{b: [3]float32{0.9213790163564168, -1.8427552370064049, 0.9213790163564168}, a: [2]float32{-1.9604500061078971, 0.9611862979079667}},
	{b: [3]float32{1.0, -1.9999789078432082, 1.0}, a: [2]float32{-1.9923834169149972, 0.9926001112941157}},
	{b: [3]float32{1.0, -1.9999632520325810, 1.0}, a: [2]float32{-1.9983570340145236, 0.9984928491805198}},
}

func NuevoPasaAltos() *PasaAltos {
	p := &PasaAltos{}
	for i, c := range coefAltos48k {
		p.s[i].b, p.s[i].a = c.b, c.a
	}
	return p
}

// Procesar filters x in place (float samples, in int16 scale, as WebRTC's audio buffer holds them).
func (p *PasaAltos) Procesar(x []float32) {
	for k := range p.s {
		s := &p.s[k]
		for i, v := range x {
			y := s.b[0]*v + s.b[1]*s.x[0] + s.b[2]*s.x[1] - s.a[0]*s.y[0] - s.a[1]*s.y[1]
			s.x[1], s.x[0] = s.x[0], v
			s.y[1], s.y[0] = s.y[0], y
			x[i] = y
		}
	}
}
