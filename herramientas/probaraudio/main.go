// Command probaraudio opens the microphone and the speaker for ten seconds, playing a steady tone, and reports the
// voice's counters: whether the device side (winmm) loses or starves buffers, apart from any network.
package main

import (
	"fmt"
	"math"
	"time"

	"github.com/nicoconvoz/zonda/zondago/voz"
)

func main() {
	cuadros := 0
	var ultimo time.Time
	var peor time.Duration
	m, err := voz.AbrirMicrofono(func(p []int16) {
		cuadros++
		if !ultimo.IsZero() && time.Since(ultimo) > peor {
			peor = time.Since(ultimo)
		}
		ultimo = time.Now()
	})
	fmt.Println("micrófono:", err)
	fase := 0.0
	p, err := voz.AbrirParlante(func(out []int16) {
		for i := range out {
			out[i] = int16(1500 * math.Sin(fase))
			fase += 2 * math.Pi * 440 / 48000
		}
	})
	fmt.Println("parlante:", err)
	time.Sleep(10 * time.Second)
	m.Cerrar()
	p.Cerrar()
	fmt.Printf("cuadros del micrófono: %d en 10 s (deberían ser 500), peor espera entre cuadros %v\n", cuadros, peor)
	fmt.Println(voz.Medidas.Texto())
}
