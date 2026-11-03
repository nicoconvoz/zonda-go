package voz

import (
	"testing"
	"time"

	"github.com/tphakala/go-opus/opus"
)

// What the Go encodes, the decoder (the same one that reads the app's voice) gives back with its energy, and one
// 20 ms frame costs a small fraction of its own 20 ms.
func TestOpusRoundTripKeepsTheVoiceCheaply(t *testing.T) {
	enc, err := opus.NewEncoder(opus.EncoderConfig{SampleRate: 48000, Channels: 1, Bitrate: bitrateVoz, ConstrainedVBR: true, Complexity: 5})
	if err != nil {
		t.Fatal(err)
	}
	d := nuevoDecodificador()
	buf := make([]byte, 1500)
	var entra, sale float64
	var tardo time.Duration
	bytes := 0
	for i := 0; i < 100; i++ {
		pcm := tono(Muestras, 6000)
		inicio := time.Now()
		n, err := enc.Encode(pcm, buf)
		fr := decodificar(d, buf[:n])
		tardo += time.Since(inicio)
		if err != nil || len(fr) != Muestras {
			t.Fatalf("cuadro %d: %v, %d muestras", i, err, len(fr))
		}
		bytes += n
		if i > 10 { // after the codec's warm-up
			entra, sale = entra+rms(pcm), sale+rms(fr)
		}
	}
	if sale < entra*0.7 || sale > entra*1.3 {
		t.Fatalf("energía: entra %.0f sale %.0f", entra, sale)
	}
	kbps := float64(bytes*8) / 2.0 / 1000
	t.Logf("%.1f kbps de voz, %.2f ms por cuadro (de 20 ms)", kbps, float64(tardo.Microseconds())/100/1000)
	if tardo/100 > 5*time.Millisecond {
		t.Fatalf("demasiado caro: %v por cuadro", tardo/100)
	}
}
