// Command crearsala makes a room in a data folder, for screenshots: go run ./herramientas/crearsala DIR NAME
package main

import (
	"os"

	"github.com/nicoconvoz/zonda-go/nucleo"
)

func main() {
	alm, _ := nucleo.AbrirAlmacen(os.Args[1])
	n := nucleo.NuevoNodo(alm)
	n.SoloLocal = true
	n.Salas().Crear(os.Args[2])
	alm.Guardar()
}
