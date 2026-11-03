//go:build windows

package voz

import (
	"errors"
	"sync"
	"syscall"
	"unsafe"
)

// The microphone and the speaker through winmm (waveIn / waveOut), pure Go: 48 kHz mono 16-bit, the rate Opus
// carries in WebRTC, in 20 ms frames. Windows resamples from and to the device. The wait for each buffer is an event the
// driver signals (CALLBACK_EVENT): no polling loop.

var (
	winmm          = syscall.NewLazyDLL("winmm.dll")
	kernel         = syscall.NewLazyDLL("kernel32.dll")
	waveInOpen     = winmm.NewProc("waveInOpen")
	waveInPrepare  = winmm.NewProc("waveInPrepareHeader")
	waveInAdd      = winmm.NewProc("waveInAddBuffer")
	waveInStart    = winmm.NewProc("waveInStart")
	waveInReset    = winmm.NewProc("waveInReset")
	waveInClose    = winmm.NewProc("waveInClose")
	waveInUnprep   = winmm.NewProc("waveInUnprepareHeader")
	waveOutOpen    = winmm.NewProc("waveOutOpen")
	waveOutPrepare = winmm.NewProc("waveOutPrepareHeader")
	waveOutWrite   = winmm.NewProc("waveOutWrite")
	waveOutReset   = winmm.NewProc("waveOutReset")
	waveOutClose   = winmm.NewProc("waveOutClose")
	waveOutUnprep  = winmm.NewProc("waveOutUnprepareHeader")
	createEvent    = kernel.NewProc("CreateEventW")
	waitSingle     = kernel.NewProc("WaitForSingleObject")
	closeHandle    = kernel.NewProc("CloseHandle")
	setEvent       = kernel.NewProc("SetEvent")
)

const (
	waveMapper    = 0xFFFFFFFF
	callbackEvent = 0x00050000
	whdrDone      = 1
	Muestras      = 960 // 20 ms at 48 kHz
	nBuffers      = 6
)

type waveFormat struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}

type waveHdr struct {
	Data          *byte
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

var formato = waveFormat{FormatTag: 1, Channels: 1, SamplesPerSec: 48000, AvgBytesPerSec: 96000, BlockAlign: 2, BitsPerSample: 16}

type dispositivo struct {
	h     uintptr
	ev    uintptr
	hdrs  []*waveHdr
	bufs  [][]byte
	parar chan struct{}
	listo sync.WaitGroup
}

func nuevoEvento() uintptr { h, _, _ := createEvent.Call(0, 0, 0, 0); return h }

// Microfono calls frame with every 20 ms of captured audio until Cerrar.
type Microfono struct{ d dispositivo }

func AbrirMicrofono(frame func([]int16)) (*Microfono, error) {
	m := &Microfono{d: dispositivo{ev: nuevoEvento(), parar: make(chan struct{})}}
	if r, _, _ := waveInOpen.Call(uintptr(unsafe.Pointer(&m.d.h)), waveMapper, uintptr(unsafe.Pointer(&formato)), m.d.ev, 0, callbackEvent); r != 0 {
		closeHandle.Call(m.d.ev)
		return nil, errors.New("no se pudo abrir el micrófono")
	}
	for i := 0; i < nBuffers; i++ {
		b := make([]byte, Muestras*2)
		h := &waveHdr{Data: &b[0], BufferLength: uint32(len(b))}
		m.d.bufs, m.d.hdrs = append(m.d.bufs, b), append(m.d.hdrs, h)
		waveInPrepare.Call(m.d.h, uintptr(unsafe.Pointer(h)), unsafe.Sizeof(*h))
		waveInAdd.Call(m.d.h, uintptr(unsafe.Pointer(h)), unsafe.Sizeof(*h))
	}
	waveInStart.Call(m.d.h)
	m.d.listo.Add(1)
	go func() {
		defer m.d.listo.Done()
		next := 0
		for {
			waitSingle.Call(m.d.ev, 0xFFFFFFFF)
			select {
			case <-m.d.parar:
				return
			default:
			}
			// Buffers complete in order: take every finished one from where the last pass stopped.
			for m.d.hdrs[next].Flags&whdrDone != 0 {
				h, b := m.d.hdrs[next], m.d.bufs[next]
				pcm := make([]int16, h.BytesRecorded/2)
				for i := range pcm {
					pcm[i] = int16(uint16(b[2*i]) | uint16(b[2*i+1])<<8)
				}
				h.Flags &^= whdrDone
				waveInAdd.Call(m.d.h, uintptr(unsafe.Pointer(h)), unsafe.Sizeof(*h))
				if len(pcm) > 0 {
					frame(pcm)
				}
				next = (next + 1) % nBuffers
			}
		}
	}()
	return m, nil
}

func (m *Microfono) Cerrar() {
	close(m.d.parar)
	setEvent.Call(m.d.ev)
	m.d.listo.Wait()
	waveInReset.Call(m.d.h)
	for _, h := range m.d.hdrs {
		waveInUnprep.Call(m.d.h, uintptr(unsafe.Pointer(h)), unsafe.Sizeof(*h))
	}
	waveInClose.Call(m.d.h)
	closeHandle.Call(m.d.ev)
}

// Parlante plays what siguiente returns, 20 ms at a time (silence when nothing comes), until Cerrar.
type Parlante struct{ d dispositivo }

func AbrirParlante(siguiente func([]int16)) (*Parlante, error) {
	p := &Parlante{d: dispositivo{ev: nuevoEvento(), parar: make(chan struct{})}}
	if r, _, _ := waveOutOpen.Call(uintptr(unsafe.Pointer(&p.d.h)), waveMapper, uintptr(unsafe.Pointer(&formato)), p.d.ev, 0, callbackEvent); r != 0 {
		closeHandle.Call(p.d.ev)
		return nil, errors.New("no se pudo abrir el parlante")
	}
	pcm := make([]int16, Muestras)
	llenar := func(i int) {
		for j := range pcm {
			pcm[j] = 0
		}
		siguiente(pcm)
		b := p.d.bufs[i]
		for j, s := range pcm {
			b[2*j], b[2*j+1] = byte(s), byte(uint16(s)>>8)
		}
		h := p.d.hdrs[i]
		h.Flags &^= whdrDone
		waveOutWrite.Call(p.d.h, uintptr(unsafe.Pointer(h)), unsafe.Sizeof(*h))
	}
	// Three buffers queued: 60 ms of latency, enough to ride out the scheduler.
	const enCola = 3
	for i := 0; i < enCola; i++ {
		b := make([]byte, Muestras*2)
		h := &waveHdr{Data: &b[0], BufferLength: uint32(len(b))}
		p.d.bufs, p.d.hdrs = append(p.d.bufs, b), append(p.d.hdrs, h)
		waveOutPrepare.Call(p.d.h, uintptr(unsafe.Pointer(h)), unsafe.Sizeof(*h))
	}
	for i := 0; i < enCola; i++ {
		llenar(i)
	}
	p.d.listo.Add(1)
	go func() {
		defer p.d.listo.Done()
		next := 0
		for {
			waitSingle.Call(p.d.ev, 0xFFFFFFFF)
			select {
			case <-p.d.parar:
				return
			default:
			}
			for p.d.hdrs[next].Flags&whdrDone != 0 {
				llenar(next)
				next = (next + 1) % enCola
			}
		}
	}()
	return p, nil
}

func (p *Parlante) Cerrar() {
	close(p.d.parar)
	setEvent.Call(p.d.ev)
	p.d.listo.Wait()
	waveOutReset.Call(p.d.h)
	for _, h := range p.d.hdrs {
		waveOutUnprep.Call(p.d.h, uintptr(unsafe.Pointer(h)), unsafe.Sizeof(*h))
	}
	waveOutClose.Call(p.d.h)
	closeHandle.Call(p.d.ev)
}
