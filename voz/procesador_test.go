package voz

import (
	"math"
	"testing"
)

func senoidal(n int, hz, amp float64, desde int) []int16 {
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(amp * math.Sin(2*math.Pi*hz*float64(desde+i)/48000))
	}
	return out
}

// Mains hum (50 Hz) is taken far down; a voice tone (1 kHz) passes whole.
func TestProcessorRemovesHumKeepsVoice(t *testing.T) {
	medir := func(hz float64) (antes, despues float64) {
		p := NuevoProcesador()
		for i := 0; i < 300; i++ { // 6 s: the mains frequency is found, the noise floor settles
			f := senoidal(Muestras, hz, 4000, i*Muestras)
			if i == 299 {
				antes = rms(f)
			}
			p.Procesar(f)
			if i == 299 {
				despues = rms(f)
			}
		}
		return
	}
	a, d := medir(50)
	if 20*math.Log10(d/a) > -20 {
		t.Fatalf("el zumbido de 50 Hz bajó solo %.1f dB", 20*math.Log10(d/a))
	}
	// A steady 1 kHz tone is steady like noise: the suppressor turns it down, but by no more than WebRTC's floor at
	// its 12 dB level: minimum_attenuating_gain 0.25, times the pause scaling it can add (1 - 0.3*(0.5-0.25) = 0.925,
	// wiener_filter.cc), with the three-band filter bank's 0.3 dB ripple (three_band_filter_bank.h).
	a, d = medir(1000)
	if 20*math.Log10(d/a) < 20*math.Log10(0.25*0.925)-0.3-0.2 {
		t.Fatalf("un tono de voz perdió %.1f dB", 20*math.Log10(d/a))
	}
}
