package voz

import "math"

// Compuerta cleans the microphone the way a browser's voice processing does, in two steps:
//   - automatic gain: a voice far from the microphone is brought up to a talking level (up to maxGanancia), slowly,
//     and a limiter keeps loud peaks from clipping;
//   - a gentle gate: the room's own floor (the quietest level heard lately) is turned down to fondoAmbiente between
//     words, never cut, so the room still sounds like a room.
//
// Every threshold is relative to the floor this microphone hears, so it holds for a quiet and a noisy one alike.
type Compuerta struct {
	piso     float64 // RMS of the noise floor: the quietest frame of the last ventana frames
	ultimos  []float64
	voz      float64 // RMS of the speech heard lately (only frames over the floor)
	ganancia float64 // the gate's gain now (fondoAmbiente..1)
	agc      float64 // the automatic gain now
}

// Criterion, not a measurement (to be tuned by ear with the Captain):
//   - speech counts from 1.5× the floor (3.5 dB): a voice a meter away is only a few dB over a quiet room;
//   - the gate is fully open at 2.5× (8 dB);
//   - between words the room stays at 30 % (about −10 dB): heard, not loud;
//   - the voice is brought to an RMS of 3000 (about −21 dBFS, a normal talking level), never more than 8× (18 dB).
const (
	abreDesde     = 1.5
	abiertaEn     = 2.5
	fondoAmbiente = 0.3
	nivelVoz      = 3000.0
	maxGanancia   = 8.0
)

// ventana: 3 s of 20 ms frames. Whoever talks pauses within 3 s, so its quietest frame is the room (minimum
// statistics, as noise suppressors estimate their floor).
const ventana = 150

func NuevaCompuerta() *Compuerta { return &Compuerta{voz: nivelVoz, ganancia: 1, agc: 1} }

func rms(pcm []int16) float64 {
	var s float64
	for _, x := range pcm {
		s += float64(x) * float64(x)
	}
	return math.Sqrt(s / float64(max(1, len(pcm))))
}

// Limpiar applies the gain and the gate to one frame in place.
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
	habla := r > c.piso*abreDesde
	if habla {
		// The speech level moves over about a second: the gain follows how someone talks, not each syllable.
		c.voz = 0.95*c.voz + 0.05*r
	}
	objetivoAGC := max(1, min(maxGanancia, nivelVoz/max(c.voz, 1)))
	c.agc = 0.95*c.agc + 0.05*objetivoAGC
	// Fast attack: someone who suddenly talks loud (or comes close) is never multiplied into a wall of sound.
	if r*c.agc > 3*nivelVoz {
		c.agc = max(1, 3*nivelVoz/r)
	}

	objetivo := fondoAmbiente + (1-fondoAmbiente)*max(0, min(1, (r/c.piso-abreDesde)/(abiertaEn-abreDesde)))
	// Opens at once (the first syllable must pass), closes over ~100 ms.
	if objetivo > c.ganancia {
		c.ganancia = objetivo
	} else {
		c.ganancia = 0.8*c.ganancia + 0.2*objetivo
	}
	// The room stays at its own level turned down; only the voice gets the automatic gain.
	abierta := (c.ganancia - fondoAmbiente) / (1 - fondoAmbiente)
	g := fondoAmbiente + (c.agc-fondoAmbiente)*abierta
	for i, x := range pcm {
		pcm[i] = limitar(float64(x) * g)
	}
}

// limitar: a soft limiter, linear up to 80 % of full scale and curving into it above, so a loud laugh never clips.
func limitar(v float64) int16 {
	const tope, codo = 32000.0, 0.8 * 32767.0
	a := math.Abs(v)
	if a > codo {
		a = codo + (tope-codo)*math.Tanh((a-codo)/(tope-codo))
	}
	return int16(math.Copysign(a, v))
}
