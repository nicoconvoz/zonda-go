// Command zondago is Zonda written in Go with no browser: the window is drawn by Gio, links are WebRTC (pion) and
// the Nostr board is reached over plain WebSockets. It speaks the same protocol as the Flutter app.
package main

import (
	"log"
	"os"
	"path/filepath"

	"gioui.org/app"

	"github.com/nicoconvoz/zonda/zondago/interfaz"
	"github.com/nicoconvoz/zonda/zondago/nucleo"
)

func main() {
	dir := os.Getenv("ZONDAGO_DATOS")
	if dir == "" {
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "ZondaGo")
	}
	alm, err := nucleo.AbrirAlmacen(dir)
	if err != nil {
		log.Fatal(err)
	}
	n := nucleo.NuevoNodo(alm)
	go n.Correr()
	ui := interfaz.Nueva(n, nil)
	go func() {
		err := ui.Correr()
		n.Consultar(n.Chau)
		alm.Guardar()
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}
