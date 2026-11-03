package voz

import (
	"math"
	"math/cmplx"

	"github.com/nicoconvoz/zonda/zondago/voz/webrtcns"
)

// Procesador is the microphone's voice processing, the way WebRTC does it (audio_processing_impl.cc,
// ProcessCaptureStreamLocked) and with its own pieces:
//
//  1. WebRTC's high-pass filter, identical (pasa_altos.go);
//  2. echo cancellation: not yet (WebRTC's AEC3 needs the speaker's signal as reference);
//  3. WebRTC's noise suppressor, translated line by line (webrtcns), at its default level (12 dB), with the
//     three-band split it runs inside at 48 kHz;
//
// and then the gain (Compuerta). WebRTC works in 10 ms frames: each 20 ms frame here is two of them.
type Procesador struct {
	altos    *PasaAltos
	supresor *webrtcns.Supresor
}

func NuevoProcesador() *Procesador {
	return &Procesador{altos: NuevoPasaAltos(), supresor: webrtcns.NuevoSupresor(webrtcns.K12dB)}
}

// Procesar cleans one frame in place (a multiple of 10 ms).
func (p *Procesador) Procesar(pcm []int16) {
	var x [webrtcns.KFullBandSize]float32
	for i := 0; i+webrtcns.KFullBandSize <= len(pcm); i += webrtcns.KFullBandSize {
		for k := range x {
			x[k] = float32(pcm[i+k])
		}
		p.altos.Procesar(x[:])
		p.supresor.Procesar(&x)
		for k, v := range x {
			pcm[i+k] = int16(max(-32768, min(32767, v)))
		}
	}
}

// FFT is the forward transform, for the bench tools.
func FFT(a []complex128) {
	n := len(a)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	for l := 2; l <= n; l <<= 1 {
		w := cmplx.Rect(1, -2*math.Pi/float64(l))
		for i := 0; i < n; i += l {
			wk := complex(1, 0)
			for k := 0; k < l/2; k++ {
				u, v := a[i+k], a[i+k+l/2]*wk
				a[i+k], a[i+k+l/2] = u+v, u-v
				wk *= w
			}
		}
	}
}
