package voz

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// The voice's instrument: what happened to every packet and every buffer, shown live in the room, so the cause of a
// bad sound is measured instead of guessed. And, with ZONDAGO_GRABAR=1, the microphone as it came in, as it went out,
// and what the speaker played, as WAV files to listen to and analyze.

type medidas struct {
	recibidos, tirados, ocultados, decodificados, acortados atomic.Int64
	parlanteSeco, parlanteAtrasado                          atomic.Int64 // speaker buffers that ran out / several due at once
	micAtrasado, micPerdido                                 atomic.Int64 // capture buffers found late / all of them full (audio lost)

	mu       sync.Mutex
	llegadas []float64 // the gaps between packets of one source, ms (the last few hundred)
}

var Medidas = &medidas{}

func (m *medidas) llegada(gap time.Duration) {
	m.mu.Lock()
	m.llegadas = append(m.llegadas, float64(gap.Microseconds())/1000)
	if len(m.llegadas) > 500 {
		m.llegadas = m.llegadas[len(m.llegadas)-500:]
	}
	m.mu.Unlock()
}

// vaivén: the 95th percentile of how far the gaps between packets stray from 20 ms (the jitter a buffer must absorb).
func (m *medidas) vaiven() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.llegadas) < 10 {
		return 0
	}
	d := make([]float64, len(m.llegadas))
	for i, g := range m.llegadas {
		d[i] = g - 20
		if d[i] < 0 {
			d[i] = -d[i]
		}
	}
	sort.Float64s(d)
	return d[len(d)*95/100]
}

// Texto: the live line for the room.
func (m *medidas) Texto() string {
	return fmt.Sprintf("paquetes %d · tirados %d · salteados en silencio %d · rellenados %d · vaivén %.0f ms · parlante seco %d atrasado %d · micrófono atrasado %d perdido %d",
		m.recibidos.Load(), m.tirados.Load(), m.acortados.Load(), m.ocultados.Load(), m.vaiven(), m.parlanteSeco.Load(), m.parlanteAtrasado.Load(), m.micAtrasado.Load(), m.micPerdido.Load())
}

/* ---------- recording (only with ZONDAGO_GRABAR=1) ---------- */

type grabacion struct {
	mu     sync.Mutex
	pistas map[string]*pista
}

// Off unless ZONDAGO_GRABAR=1: it was for studying the sound while it was being fixed. When on, every call leaves its
// last minute as WAV files on this computer.
var grabar = os.Getenv("ZONDAGO_GRABAR") == "1"
var cinta = &grabacion{pistas: map[string]*pista{}}

// muestrasCinta: one minute at 48 kHz, 5.5 MB a track, allocated once.
const muestrasCinta = 48000 * 60

// pista is a ring: a fixed buffer written in a circle, so keeping the last minute costs one copy of each frame.
type pista struct {
	buf   []int16
	pos   int
	llena bool
}

func nuevaPista(n int) *pista { return &pista{buf: make([]int16, n)} }

func (p *pista) anotar(pcm []int16) {
	if len(pcm) > len(p.buf) {
		pcm = pcm[len(pcm)-len(p.buf):]
	}
	for len(pcm) > 0 {
		c := copy(p.buf[p.pos:], pcm)
		pcm = pcm[c:]
		p.pos += c
		if p.pos == len(p.buf) {
			p.pos, p.llena = 0, true
		}
	}
}

// enOrden: what the ring holds, oldest first.
func (p *pista) enOrden() []int16 {
	if !p.llena {
		return append([]int16(nil), p.buf[:p.pos]...)
	}
	return append(append([]int16(nil), p.buf[p.pos:]...), p.buf[:p.pos]...)
}

// anotar keeps a copy of pcm in the named track: the last minute.
func anotar(nombre string, pcm []int16) {
	if !grabar {
		return
	}
	cinta.mu.Lock()
	p := cinta.pistas[nombre]
	if p == nil {
		p = nuevaPista(muestrasCinta)
		cinta.pistas[nombre] = p
	}
	p.anotar(pcm)
	cinta.mu.Unlock()
}

// volcar writes every track as a 48 kHz mono WAV into dir and starts over.
func volcar(dir string) {
	if !grabar {
		return
	}
	cinta.mu.Lock()
	pistas := map[string][]int16{}
	for nombre, p := range cinta.pistas {
		pistas[nombre] = p.enOrden()
	}
	cinta.pistas = map[string]*pista{}
	cinta.mu.Unlock()
	if len(pistas) == 0 {
		return
	}
	os.MkdirAll(dir, 0o755)
	marca := time.Now().Format("150405")
	os.WriteFile(filepath.Join(dir, marca+"-contadores.txt"), []byte(Medidas.Texto()+"\n"), 0o644)
	for nombre, pcm := range pistas {
		f, err := os.Create(filepath.Join(dir, marca+"-"+nombre+".wav"))
		if err != nil {
			continue
		}
		n := uint32(len(pcm) * 2)
		f.Write([]byte("RIFF"))
		binary.Write(f, binary.LittleEndian, 36+n)
		f.Write([]byte("WAVEfmt "))
		// binary.Write takes one fixed-size value at a time: a []any wrote nothing and left the header broken.
		for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(48000), uint32(96000), uint16(2), uint16(16)} {
			binary.Write(f, binary.LittleEndian, v)
		}
		f.Write([]byte("data"))
		binary.Write(f, binary.LittleEndian, n)
		binary.Write(f, binary.LittleEndian, pcm)
		f.Close()
	}
}
