package voz

import (
	"math"
	"testing"
	"time"

	"github.com/tphakala/go-opus/opus"
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

func tono(n int, amp float64) []int16 {
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(amp * math.Sin(float64(i)*0.2))
	}
	return out
}

// The room travels as it sounds; a voice a meter away is brought up to a talking level; a loud one is not clipped; and
// the gain never jumps at a frame's edge (that step is a crackle).
func TestGainKeepsTheRoomBringsAFarVoiceUpAndGlides(t *testing.T) {
	c := NuevaCompuerta()
	for i := 0; i < 100; i++ {
		c.Limpiar(tono(Muestras, 80)) // a second of room
	}
	h := tono(Muestras, 80)
	c.Limpiar(h)
	if r := rms(h); r < 80*0.7*0.9 {
		t.Fatalf("el ambiente se apagó: rms %.1f", r)
	}
	var lejos []int16
	for i := 0; i < 100; i++ {
		lejos = tono(Muestras, 160*1.414)
		c.Limpiar(lejos)
	}
	if r := rms(lejos); r < 1000 {
		t.Fatalf("la voz lejana no sube: rms %.1f", r)
	}
	fuerte := tono(Muestras, 30000)
	c.Limpiar(fuerte)
	for _, x := range fuerte {
		if x == 32767 || x == -32768 {
			t.Fatal("satura")
		}
	}
	// A constant signal through a changing gain: neighbouring samples never jump.
	d := NuevaCompuerta()
	for i := 0; i < 50; i++ {
		pcm := make([]int16, Muestras)
		for k := range pcm {
			pcm[k] = 200
		}
		d.Limpiar(pcm)
		for k := 1; k < len(pcm); k++ {
			if diff := int(pcm[k]) - int(pcm[k-1]); diff > 2 || diff < -2 {
				t.Fatalf("salto de ganancia dentro del cuadro %d: %d", i, diff)
			}
		}
	}
}

// A late packet is concealed (a frame like the last one), never a hole of silence; a backlog drains to the cushion.
func TestSourceConcealsALatePacketAndDrains(t *testing.T) {
	enc, _ := opus.NewEncoder(opus.EncoderConfig{SampleRate: 48000, Channels: 1, Bitrate: bitrateVoz})
	f := nuevaFuente()
	buf := make([]byte, 1500)
	for i := 0; i < 10; i++ {
		n, _ := enc.Encode(tono(Muestras, 6000), buf)
		f.Llego(buf[:n])
	}
	for i := 0; i < colchon+1; i++ {
		if f.Siguiente() == nil {
			t.Fatal("hay paquetes y no suena")
		}
	}
	if len(f.fila) != 0 {
		t.Fatalf("quedaron %d en la fila: la demora no se vació", len(f.fila))
	}
	tarde := f.Siguiente()
	if tarde == nil || rms(tarde) < 100 {
		t.Fatal("un paquete tarde dejó un hueco de silencio")
	}
	time.Sleep(ocultarHasta + 20*time.Millisecond)
	if f.Siguiente() != nil {
		t.Fatal("una pausa larga se sigue inventando")
	}
}
