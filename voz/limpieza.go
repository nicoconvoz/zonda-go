package voz

import "math"

// Compuerta is the microphone's automatic gain, as a browser's voice processing does it: a voice far from the
// microphone is brought up to a talking level (up to maxGanancia), slowly, with a fast attack so a loud voice never
// turns into a wall of sound, and a soft limiter for the peaks. The room is not touched apart from that gain: it
// travels as it sounds. The gain glides sample by sample from one frame to the next: a step at each frame's edge is
// heard as a crackle.
type Compuerta struct {
	piso    float64 // RMS of the room: the quietest frame of the last ventana frames
	ultimos []float64
	voz     float64 // RMS of the speech heard lately (only frames clearly over the room)
	agc     float64 // the gain reached at the end of the last frame
}

// Criterion, not a measurement (to be tuned by ear with the Captain):
//   - a frame counts as speech from 1.5× the room (3.5 dB): a voice a meter away is only a few dB over a quiet room;
//   - the voice is brought to an RMS of 3000 (about −21 dBFS, a normal talking level), never more than 8× (18 dB).
const (
	hablaDesde  = 1.5
	nivelVoz    = 3000.0
	maxGanancia = 8.0
)

// ventana: 3 s of 20 ms frames. Whoever talks pauses within 3 s, so its quietest frame is the room (minimum
// statistics, as noise suppressors estimate their floor).
const ventana = 150

func NuevaCompuerta() *Compuerta { return &Compuerta{voz: nivelVoz, agc: 1} }

func rms(pcm []int16) float64 {
	var s float64
	for _, x := range pcm {
		s += float64(x) * float64(x)
	}
	return math.Sqrt(s / float64(max(1, len(pcm))))
}

// Limpiar applies the gain to one frame in place.
func (c *Compuerta) Limpiar(pcm []int16) {
	r := rms(pcm)
	c.ultimos = append(c.ultimos, r)
	if len(c.ultimos) > ventana {
		c.ultimos = c.ultimos[1:]
	}
	c.piso = r
	for _, x := range c.ultimos {
		c.piso = min(c.piso, x)
	}
	c.piso = max(c.piso, 4)
	if r > c.piso*hablaDesde {
		// The speech level moves over about a second: the gain follows how someone talks, not each syllable.
		c.voz = 0.95*c.voz + 0.05*r
	}
	nuevo := 0.95*c.agc + 0.05*max(1, min(maxGanancia, nivelVoz/max(c.voz, 1)))
	// Fast attack: someone who suddenly talks loud (or comes close) is never multiplied into a wall of sound.
	if r*nuevo > 3*nivelVoz {
		nuevo = max(1, 3*nivelVoz/r)
	}
	desde := c.agc
	c.agc = nuevo
	n := float64(max(1, len(pcm)))
	for i, x := range pcm {
		g := desde + (nuevo-desde)*float64(i+1)/n
		pcm[i] = limitar(float64(x) * g)
	}
}

// limitar: a soft limiter, linear up to 80 % of full scale and curving below it above, so a loud laugh never clips.
func limitar(v float64) int16 {
	const tope, codo = 32000.0, 0.8 * 32767.0
	a := math.Abs(v)
	if a > codo {
		a = codo + (tope-codo)*math.Tanh((a-codo)/(tope-codo))
	}
	return int16(math.Copysign(a, v))
}
