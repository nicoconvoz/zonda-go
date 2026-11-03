// Command limpiar runs a microphone recording through the voice processing and reports, band by band, how much noise
// went and how much voice stayed: the bench for the processor, on a real microphone.
//
//	go run ./herramientas/limpiar entrada.wav salida.wav
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/cmplx"
	"os"
	"sort"

	"github.com/nicoconvoz/zonda/zondago/voz"
)

func leer(p string) []int16 {
	b, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}
	i := bytes.Index(b, []byte("data")) + 8
	pcm := make([]int16, (len(b)-i)/2)
	binary.Read(bytes.NewReader(b[i:]), binary.LittleEndian, pcm)
	return pcm
}

func escribir(p string, pcm []int16) {
	f, _ := os.Create(p)
	defer f.Close()
	n := uint32(len(pcm) * 2)
	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, 36+n)
	f.Write([]byte("WAVEfmt "))
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(48000), uint32(96000), uint16(2), uint16(16)} {
		binary.Write(f, binary.LittleEndian, v)
	}
	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, n)
	binary.Write(f, binary.LittleEndian, pcm)
}

// bandas: mean power (dB) per band over the quietest 15 % and the loudest 15 % of frames.
var cortes = [][2]float64{{0, 100}, {100, 300}, {300, 1000}, {1000, 3000}, {3000, 6000}, {6000, 12000}, {12000, 24000}}

func bandas(pcm []int16) (ruido, vozDb []float64) {
	const n = 1024
	var rs []float64
	var specs [][]float64
	for i := 0; i+960 <= len(pcm); i += 960 {
		buf := make([]complex128, n)
		var e float64
		for k := 0; k < 960; k++ {
			w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(k)/960)
			buf[k] = complex(float64(pcm[i+k])*w, 0)
			e += float64(pcm[i+k]) * float64(pcm[i+k])
		}
		voz.FFT(buf)
		s := make([]float64, n/2)
		for k := range s {
			s[k] = cmplx.Abs(buf[k]) * cmplx.Abs(buf[k])
		}
		rs, specs = append(rs, e), append(specs, s)
	}
	orden := make([]int, len(rs))
	for i := range orden {
		orden[i] = i
	}
	sort.Slice(orden, func(a, b int) bool { return rs[orden[a]] < rs[orden[b]] })
	medio := func(idx []int) []float64 {
		out := make([]float64, len(cortes))
		for bi, c := range cortes {
			var acc float64
			var cnt int
			for _, i := range idx {
				for k, v := range specs[i] {
					hz := float64(k) * 48000 / n
					if hz >= c[0] && hz < c[1] {
						acc += v
						cnt++
					}
				}
			}
			out[bi] = 10 * math.Log10(acc/float64(max(cnt, 1))+1)
		}
		return out
	}
	q := len(orden) * 15 / 100
	return medio(orden[:q]), medio(orden[len(orden)-q:])
}

func main() {
	pcm := leer(os.Args[1])
	antesR, antesV := bandas(pcm)
	p := voz.NuevoProcesador()
	out := append([]int16(nil), pcm...)
	for i := 0; i+voz.Muestras <= len(out); i += voz.Muestras {
		p.Procesar(out[i : i+voz.Muestras])
	}
	if len(os.Args) > 2 {
		escribir(os.Args[2], out)
	}
	despR, despV := bandas(out)
	fmt.Println("banda            ruido antes → después     voz antes → después")
	for i, c := range cortes {
		fmt.Printf("%5.0f-%5.0f Hz   %6.1f → %6.1f (%+5.1f)   %6.1f → %6.1f (%+5.1f)\n", c[0], c[1], antesR[i], despR[i], despR[i]-antesR[i], antesV[i], despV[i], despV[i]-antesV[i])
	}
}
