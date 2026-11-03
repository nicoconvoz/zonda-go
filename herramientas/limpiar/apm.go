package main

import (
	"os"

	"github.com/nicoconvoz/zonda/zondago/voz"
)

// procesarAPM runs the recording through WebRTC's real APM (when ZONDA_APM=1): the bench for the bridge.
func procesarAPM(out []int16) bool {
	if os.Getenv("ZONDA_APM") != "1" {
		return false
	}
	a, err := voz.NuevoAPM(0)
	if err != nil {
		panic(err)
	}
	defer a.Cerrar()
	silencio := make([]int16, voz.Muestras)
	for i := 0; i+voz.Muestras <= len(out); i += voz.Muestras {
		a.Parlante(silencio)
		a.Microfono(out[i : i+voz.Muestras])
	}
	return true
}
