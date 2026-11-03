package voz

import (
	"sync"
	"time"

	"github.com/tphakala/go-opus/opus"
)

// Fuente is one voice that arrives (a member over WebRTC, or a speaker of the tree): its Opus packets wait in a line
// and are decoded only when the speaker needs the next 20 ms. The line is a small adaptive jitter buffer, after
// NetEQ's idea:
//   - its target depth follows the measured jitter of this voice (how far packet gaps stray from 20 ms), so a steady
//     network waits 20 ms and a bursty Wi-Fi waits what it needs;
//   - when the line is deeper than the target, the extra packets are dropped only while they are quiet (between
//     words), never in the middle of the voice: a dropped voice packet is a cut, and cuts are heard as static;
//   - when the next packet is late, Opus conceals the gap (a frame like the last one) instead of playing silence.
//
// Measured before this, with a fixed 40 ms line on the Captain's Wi-Fi: 57 of 1751 packets dropped (3 %), jitter p95
// 21 ms.
type Fuente struct {
	mu      sync.Mutex
	fila    [][]byte
	dec     *opus.Decoder
	ultimo  time.Time
	sonando bool
	vaiven  float64 // smoothed jitter of this voice, ms
}

const (
	// ocultarHasta: how long a silent member is concealed before playing silence (a real pause, or they left).
	ocultarHasta = 120 * time.Millisecond
	// filaMax: past this (240 ms) the oldest packet goes whatever it holds: a delay that big is worse than a cut.
	filaMax = 12
	// silencioRMS: a decoded frame this quiet (about −46 dBFS) is between words and may be skipped. Criterion, not a
	// measurement: well under speech (−20 to −30 dBFS on the Captain's microphone) and over a cleaned room.
	silencioRMS = 150
)

func nuevaFuente() *Fuente { return &Fuente{dec: nuevoDecodificador()} }

// objetivo: how many packets the line should hold: one, plus what the jitter needs, at most filaMax/2.
func (f *Fuente) objetivo() int {
	return max(1, min(filaMax/2, 1+int(f.vaiven*2/20+0.5)))
}

// Llego queues one packet.
func (f *Fuente) Llego(pkt []byte) {
	f.mu.Lock()
	ahora := time.Now()
	if !f.ultimo.IsZero() && ahora.Sub(f.ultimo) < time.Second {
		gap := ahora.Sub(f.ultimo)
		Medidas.llegada(gap)
		d := float64(gap.Microseconds())/1000 - 20
		if d < 0 {
			d = -d
		}
		// Up fast (a burst must be absorbed now), down slowly (the network may burst again).
		if d > f.vaiven {
			f.vaiven = 0.7*f.vaiven + 0.3*d
		} else {
			f.vaiven = 0.98*f.vaiven + 0.02*d
		}
	}
	f.fila = append(f.fila, append([]byte(nil), pkt...))
	f.ultimo = ahora
	f.mu.Unlock()
	Medidas.recibidos.Add(1)
}

// Siguiente gives the next 20 ms of this voice: the next packet decoded, a concealed frame when it is late, or nil
// when nothing came for a while.
func (f *Fuente) Siguiente() []int16 {
	f.mu.Lock()
	defer f.mu.Unlock()
	for len(f.fila) > filaMax {
		f.fila = f.fila[1:]
		Medidas.tirados.Add(1)
	}
	out := make([]int16, Muestras)
	for len(f.fila) > 0 {
		pkt := f.fila[0]
		f.fila = f.fila[1:]
		n, err := f.dec.Decode(pkt, out)
		if err != nil || n <= 0 {
			continue
		}
		// Deeper than needed and this frame is between words: skip it, the delay shrinks without cutting the voice.
		if len(f.fila) >= f.objetivo() && rms(out[:n]) < silencioRMS {
			Medidas.acortados.Add(1)
			continue
		}
		f.sonando = true
		Medidas.decodificados.Add(1)
		return out[:n]
	}
	if f.sonando && time.Since(f.ultimo) < ocultarHasta {
		if n, err := f.dec.Decode(nil, out); err == nil && n > 0 {
			Medidas.ocultados.Add(1)
			return out[:n]
		}
	}
	f.sonando = false
	return nil
}
