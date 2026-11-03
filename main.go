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
	"github.com/nicoconvoz/zonda/zondago/voz"
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
	// For trying several devices on one computer: ZONDAGO_APODO makes the account with no clicks.
	if a := os.Getenv("ZONDAGO_APODO"); a != "" && !n.TieneCuenta() {
		n.CrearCuenta(a, "m", nil)
	}
	go n.Correr()
	sala := voz.NuevaSala(n)
	ui := interfaz.Nueva(n, sala)
	go func() {
		err := ui.Correr()
		sala.Guardar()
		n.Consultar(n.Chau)
		alm.Guardar()
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}
