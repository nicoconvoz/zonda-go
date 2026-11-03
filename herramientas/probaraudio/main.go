// Command probaraudio opens the microphone and the speaker for two seconds and says what happened: it tells a
// device problem apart from a connection problem.
package main

import (
	"fmt"
	"time"

	"github.com/nicoconvoz/zonda/zondago/voz"
)

func main() {
	cuadros, pico := 0, 0
	m, err := voz.AbrirMicrofono(func(p []int16) {
		cuadros++
		for _, s := range p {
			if int(s) > pico {
				pico = int(s)
			}
		}
	})
	fmt.Println("micrófono:", err)
	p, err := voz.AbrirParlante(func([]int16) {})
	fmt.Println("parlante:", err)
	time.Sleep(2 * time.Second)
	if m != nil {
		m.Cerrar()
	}
	if p != nil {
		p.Cerrar()
	}
	fmt.Printf("cuadros de 20 ms en 2 s: %d (deberían ser ~100), pico %d\n", cuadros, pico)
}
