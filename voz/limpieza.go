package voz

import "math"

// Compuerta is a noise gate that learns the microphone's own floor: the quietest level heard lately is the hiss of
// the room and the device, and frames close to it are faded out, so the hiss between words does not travel. Speech
// is well above the floor and passes untouched. The thresholds are relative to the floor (they hold for a quiet and a
// noisy microphone alike), and the gain moves smoothly, so words are not chopped.
type Compuerta struct {
	piso     float64 // RMS of the noise floor
	ganancia float64
}

// Ratios over the floor where the gate starts opening and is fully open. Criterion, not a measurement: speech at a
// normal distance is 10-30 dB over a room's hiss; 2× (6 dB) still sounds like hiss, 4× (12 dB) is clearly voice.
const (
	abreDesde = 2.0
	abiertaEn = 4.0
)

func NuevaCompuerta() *Compuerta { return &Compuerta{piso: 30, ganancia: 1} }

func rms(pcm []int16) float64 {
	var s float64
	for _, x := range pcm {
		s += float64(x) * float64(x)
	}
	return math.Sqrt(s / float64(max(1, len(pcm))))
}

// Limpiar applies the gate to one frame in place.
func (c *Compuerta) Limpiar(pcm []int16) {
	r := rms(pcm)
	// The floor follows the quiet fast and the loud very slowly: it stays on the hiss while someone talks.
	if r < c.piso {
		c.piso = 0.9*c.piso + 0.1*r
	} else {
		c.piso *= 1.002
	}
	c.piso = max(c.piso, 4)
	objetivo := (r/c.piso - abreDesde) / (abiertaEn - abreDesde)
	objetivo = max(0, min(1, objetivo))
	// Opens at once (the first syllable must pass), closes over ~100 ms.
	if objetivo > c.ganancia {
		c.ganancia = objetivo
	} else {
		c.ganancia = 0.8*c.ganancia + 0.2*objetivo
	}
	for i, x := range pcm {
		pcm[i] = int16(float64(x) * c.ganancia)
	}
}
