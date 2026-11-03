package voz

import (
	"math"
	"math/cmplx"
)

// Procesador is the microphone's voice processing, in WebRTC's order (audio_processing_impl.cc,
// ProcessCaptureStreamLocked): high-pass filter, then echo cancellation, then noise suppression, then gain. Measured on
// the Captain's own microphone (herramientas/limpiar reads a recording and reports it): its worst noise was the mains
// hum (a peak at 50 Hz), then a steady hiss in the highs.
//
//  1. WebRTC's own high-pass filter, identical (pasa_altos.go): −68 dB at 50 Hz;
//  2. (echo cancellation: not yet; WebRTC's AEC3 needs the speaker's signal as reference);
//  3. a spectral noise suppressor: the noise of each band is the lowest level it had in the last ~1.5 s (steady
//     hiss stays, a voice does not), and what is only that noise is turned down by up to pisoGanancia. Everything
//     that changes (voice, people, the room's sounds) passes. (Being replaced by a port of WebRTC's own.)
//
// The gain (AGC) runs after it, in Compuerta. It works in hops of 10 ms with a 20 ms window (overlap-add), so it adds
// 10 ms of delay.
type Procesador struct {
	altos *PasaAltos

	ventana   []float64
	entrada   []float64 // the last tamVentana input samples
	salida    []float64 // overlap-add accumulator
	pendiente []int16   // processed samples ready, not yet handed out
	potencia  []float64 // smoothed power per bin
	minPrevio []float64 // the lowest of that power in the previous window of saltosMinimo hops
	minActual []float64 // and in the current one: the noise is the lower of the two
	saltos    int
	ganancia  []float64
}

const (
	corteHz    = 90.0
	tamVentana = 960 // 20 ms
	salto      = 480 // 10 ms
	tamFFT     = 1024
	// pisoGanancia: how far steady noise is turned down (−12 dB). Criterion, not a measurement: deeper suppression
	// makes the remaining room sound "underwater"; the Captain wants the room kept and the static gone.
	pisoGanancia = 0.25
	// The noise floor is the minimum over this many hops (1.5 s): longer than a word, shorter than a change of room.
	saltosMinimo = 150
	sobreRuido   = 2.0 // power over the floor that counts as signal (3 dB)
)

func NuevoProcesador() *Procesador {
	p := &Procesador{
		// The two stages of a 4th-order Butterworth: Q = 0.541 and 1.307.
		altos:   NuevoPasaAltos(),
		ventana: make([]float64, tamVentana), entrada: make([]float64, tamVentana), salida: make([]float64, tamVentana),
		potencia: make([]float64, tamFFT/2+1), minPrevio: make([]float64, tamFFT/2+1), minActual: make([]float64, tamFFT/2+1),
		ganancia: make([]float64, tamFFT/2+1),
	}
	// sqrt-Hann on analysis and synthesis: their product, a Hann at 50 % overlap, adds up to one.
	for i := range p.ventana {
		p.ventana[i] = math.Sqrt(0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/tamVentana))
	}
	for i := range p.minPrevio {
		p.minPrevio[i], p.minActual[i], p.ganancia[i] = math.Inf(1), math.Inf(1), 1
	}
	return p
}

// Procesar cleans one frame in place (any length; the output is delayed by one hop).
func (p *Procesador) Procesar(pcm []int16) {
	x := make([]float64, len(pcm))
	for i, v := range pcm {
		x[i] = float64(v)
	}
	f := make([]float32, len(x))
	for i, v := range x {
		f[i] = float32(v)
	}
	p.altos.Procesar(f)
	for i, v := range f {
		x[i] = float64(v)
	}
	for len(x) > 0 {
		n := min(salto, len(x))
		copy(p.entrada, p.entrada[n:])
		copy(p.entrada[tamVentana-n:], x[:n])
		x = x[n:]
		if n == salto {
			p.suprimir()
		}
	}
	for i := range pcm {
		if len(p.pendiente) > 0 {
			pcm[i] = p.pendiente[0]
			p.pendiente = p.pendiente[1:]
		} else {
			pcm[i] = 0
		}
	}
}

func (p *Procesador) suprimir() {
	buf := make([]complex128, tamFFT)
	for i, v := range p.entrada {
		buf[i] = complex(v*p.ventana[i], 0)
	}
	fft(buf, false)
	p.saltos++
	renovar := p.saltos%saltosMinimo == 0
	for k := 0; k <= tamFFT/2; k++ {
		pw := real(buf[k])*real(buf[k]) + imag(buf[k])*imag(buf[k])
		if p.potencia[k] == 0 {
			p.potencia[k] = pw
		}
		p.potencia[k] = 0.8*p.potencia[k] + 0.2*pw
		p.minActual[k] = min(p.minActual[k], p.potencia[k])
		// Minimum statistics: the floor is the lowest smoothed power of the last one or two windows, so it follows a
		// room that gets quieter at once and one that gets noisier within a window or two.
		// The minimum sits under the mean noise: its bias. Measured on the Captain's microphone (herramientas/limpiar):
		// 1.5 took the hiss down 7-8 dB, 3 takes it 10-11 dB while the voice loses only 0.2-0.4 dB more.
		ruido := min(p.minPrevio[k], p.minActual[k]) * 3.0
		if renovar {
			p.minPrevio[k], p.minActual[k] = p.minActual[k], p.potencia[k]
		}
		g := 1 - ruido/max(pw, 1e-9)
		if pw < ruido*sobreRuido {
			g = min(g, pisoGanancia)
		}
		g = max(pisoGanancia, min(1, g))
		// The gain moves smoothly across hops: a gain that flickers is heard as "musical noise".
		p.ganancia[k] = 0.6*p.ganancia[k] + 0.4*g
		buf[k] *= complex(p.ganancia[k], 0)
		if k > 0 && k < tamFFT/2 {
			buf[tamFFT-k] = cmplx.Conj(buf[k])
		}
	}
	fft(buf, true)
	for i := range p.salida {
		p.salida[i] += real(buf[i]) * p.ventana[i]
	}
	for i := 0; i < salto; i++ {
		p.pendiente = append(p.pendiente, limitar(p.salida[i]))
	}
	copy(p.salida, p.salida[salto:])
	for i := tamVentana - salto; i < tamVentana; i++ {
		p.salida[i] = 0
	}
}

// fft: in-place radix-2 FFT (inverse scales by 1/n).
func fft(a []complex128, inversa bool) {
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
		ang := 2 * math.Pi / float64(l)
		if !inversa {
			ang = -ang
		}
		w := cmplx.Rect(1, ang)
		for i := 0; i < n; i += l {
			wk := complex(1, 0)
			for k := 0; k < l/2; k++ {
				u, v := a[i+k], a[i+k+l/2]*wk
				a[i+k], a[i+k+l/2] = u+v, u-v
				wk *= w
			}
		}
	}
	if inversa {
		for i := range a {
			a[i] /= complex(float64(n), 0)
		}
	}
}

// FFT is the forward transform, for the bench tools.
func FFT(a []complex128) { fft(a, false) }
