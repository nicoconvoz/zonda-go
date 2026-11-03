// Command zondago is Zonda written in Go with no browser: the window is drawn by Gio, links are WebRTC (pion) and
// the Nostr board is reached over plain WebSockets. It speaks the same protocol as the Flutter app.
package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"gioui.org/app"

	"github.com/nicoconvoz/zonda/zondago/interfaz"
	"github.com/nicoconvoz/zonda/zondago/nucleo"
	"github.com/nicoconvoz/zonda/zondago/voz"
)

func main() {
	// For measuring: ZONDAGO_PERFIL=archivo writes a 40 s CPU profile of the running program.
	if f := os.Getenv("ZONDAGO_PERFIL"); f != "" {
		if out, err := os.Create(f); err == nil {
			pprof.StartCPUProfile(out)
			go func() {
				time.Sleep(40 * time.Second)
				pprof.StopCPUProfile()
				out.Close()
			}()
		}
	}
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
	// For screenshots of the room: ZONDAGO_ENTRAR=1 opens the first room at start.
	entrar := os.Getenv("ZONDAGO_ENTRAR") == "1"
	sala := voz.NuevaSala(n)
	ui := interfaz.Nueva(n, sala)
	if entrar {
		n.Consultar(func() {
			if m := n.Salas().Mias(); len(m) > 0 {
				ui.EntrarSala(m[0].ID())
			}
		})
	}
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
