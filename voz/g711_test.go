package voz

import (
	"math"
	"testing"
)

func TestMuLawRoundTripStaysClose(t *testing.T) {
	// Known points of the table: silence, and the extremes.
	if aMu(0) != 0xFF || aMu(32767) != 0x80 || aMu(-32768) != 0x00 {
		t.Fatalf("tabla: %x %x %x", aMu(0), aMu(32767), aMu(-32768))
	}
	for _, s := range []int16{0, 100, -100, 1000, -5000, 20000, -32000} {
		back := deMu(aMu(s))
		err := int(back) - int(s)
		if err < 0 {
			err = -err
		}
		// Mu-law keeps the error proportional to the level: about 1/16 of it at worst, plus the step near zero.
		if lim := int(abs16(s))/16 + 8; err > lim {
			t.Fatalf("%d volvió como %d (error %d > %d)", s, back, err, lim)
		}
	}
}

func abs16(s int16) int16 {
	if s < 0 {
		return -s
	}
	return s
}

// A backlog never stays: with more than the cushion waiting, the old frames go and the newest one plays.
func TestBacklogDrainsToTheCushion(t *testing.T) {
	f := make(chan []int16, 10)
	for i := 0; i < 10; i++ {
		f <- []int16{int16(i)}
	}
	fr := tomar(f)
	if fr == nil || fr[0] != 10-colchon || len(f) != colchon-1 {
		t.Fatalf("tocó %v y quedaron %d en la fila", fr, len(f))
	}
	if tomar(make(chan []int16, 10)) != nil {
		t.Fatal("sin nada, silencio")
	}
}

func tono(n int, amp float64) []int16 {
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(amp * math.Sin(float64(i)*0.2))
	}
	return out
}

// Hiss alone is faded to near silence; a voice well above it passes whole.
func TestGateDropsHissKeepsVoice(t *testing.T) {
	c := NuevaCompuerta()
	for i := 0; i < 100; i++ {
		c.Limpiar(tono(Muestras, 80)) // a second of room hiss
	}
	h := tono(Muestras, 80)
	c.Limpiar(h)
	if rms(h) > 5 {
		t.Fatalf("el siseo pasa: rms %.1f", rms(h))
	}
	v := tono(Muestras, 3000)
	c.Limpiar(v)
	if rms(v) < 3000*0.7*0.95 {
		t.Fatalf("la voz se cortó: rms %.1f", rms(v))
	}
}
