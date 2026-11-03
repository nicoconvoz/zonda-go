package voz

import (
	"sync"
	"time"

	"github.com/tphakala/go-opus/opus"
)

// Fuente is one voice that arrives (a member over WebRTC, or a speaker of the tree): its Opus packets wait in a short
// line and are decoded only when the speaker needs the next 20 ms. When the next packet is late, Opus conceals the gap
// (the decoder makes up a frame like the last one) instead of playing silence: those silences, every few packets, are
// heard as static.
type Fuente struct {
	mu      sync.Mutex
	fila    [][]byte
	dec     *opus.Decoder
	ultimo  time.Time
	sonando bool
}

// colchon: how many 20 ms packets may wait for the speaker (40 ms). Two devices' clocks never run exactly alike and
// the network delivers in bursts: without a ceiling the wait grows and stays. Criterion, not a measurement: the
// smallest cushion that rides out one late packet.
const colchon = 2

// ocultarHasta: how long a silent member is concealed before playing silence (a real pause, or they left).
const ocultarHasta = 120 * time.Millisecond

func nuevaFuente() *Fuente { return &Fuente{dec: nuevoDecodificador()} }

// Llegó queues one packet.
func (f *Fuente) Llego(pkt []byte) {
	f.mu.Lock()
	f.fila = append(f.fila, append([]byte(nil), pkt...))
	f.ultimo = time.Now()
	f.mu.Unlock()
}

// Siguiente gives the next 20 ms of this voice: the next packet decoded (older ones beyond the cushion dropped), a
// concealed frame when it is late, or nil when nothing came for a while.
func (f *Fuente) Siguiente() []int16 {
	f.mu.Lock()
	defer f.mu.Unlock()
	for len(f.fila) > colchon {
		f.fila = f.fila[1:]
	}
	out := make([]int16, Muestras)
	if len(f.fila) > 0 {
		pkt := f.fila[0]
		f.fila = f.fila[1:]
		if n, err := f.dec.Decode(pkt, out); err == nil && n > 0 {
			f.sonando = true
			return out[:n]
		}
	}
	if f.sonando && time.Since(f.ultimo) < ocultarHasta {
		if n, err := f.dec.Decode(nil, out); err == nil && n > 0 {
			return out[:n]
		}
	}
	f.sonando = false
	return nil
}
