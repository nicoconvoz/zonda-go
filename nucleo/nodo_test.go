package nucleo

import (
	"bytes"
	"testing"
	"time"
)

func nodoDePrueba(t *testing.T, nombre string) *Nodo {
	alm, err := AbrirAlmacen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	n := NuevoNodo(alm)
	n.SoloLocal = true
	n.CrearCuenta(nombre, "m", nil)
	return n
}

// unirPorRadio links two nodes the way a radio would: each one's mesh sends straight into the other's loop.
func unirPorRadio(a, b *Nodo) {
	a.Router.enviar = func(to, js string) {
		if to == "radio" {
			b.Hacer(func() { b.Router.Recibir("radio", js) })
		} else {
			a.Red.Enviar(to, js)
		}
	}
	b.Router.enviar = func(to, js string) {
		if to == "radio" {
			a.Hacer(func() { a.Router.Recibir("radio", js) })
		} else {
			b.Red.Enviar(to, js)
		}
	}
	a.Hacer(func() { a.Router.Agregar("radio") })
	b.Hacer(func() { b.Router.Agregar("radio") })
}

func esperar(t *testing.T, n *Nodo, que string, ok func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		var listo bool
		n.Consultar(func() { listo = ok() })
		if listo {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("nunca pasó: " + que)
}

func TestRoomWithOwnerEndToEnd(t *testing.T) {
	a, b := nodoDePrueba(t, "Ana"), nodoDePrueba(t, "Beto")
	go a.Correr()
	go b.Correr()
	unirPorRadio(a, b)
	esperar(t, b, "Beto ve a Ana", func() bool { return b.EnLinea(a.Clave()) })

	var c Carta
	a.Consultar(func() { c = a.Salas().Crear("Los Pibes") })
	b.Consultar(func() { b.Salas().Golpear(c) })
	esperar(t, a, "le llega el pedido", func() bool { return len(a.Salas().Pedidos(c.ID)) == 1 })
	a.Consultar(func() { a.Salas().Aprobar(c.ID, b.Clave()) })
	esperar(t, b, "Beto queda adentro con la llave", func() bool { return b.Salas().EsMiembro(c.ID, b.Clave()) })

	a.Consultar(func() { a.EntrarASala(c.ID) })
	b.Consultar(func() { b.EntrarASala(c.ID) })
	foto := bytes.Repeat([]byte("foto"), 5000)
	a.Consultar(func() { a.DecirEnSala("miren", foto, nil) })
	var linea LineaSala
	esperar(t, b, "llega la línea sellada", func() bool {
		for _, l := range b.LineasSala {
			if !l.Sistema {
				linea = l
				return true
			}
		}
		return false
	})
	if linea.Texto != "miren" || linea.Epoca != 0 {
		t.Fatalf("línea: %+v", linea)
	}
	esperar(t, b, "la foto se pide por huella y se abre con la llave", func() bool { return bytes.Equal(b.Adjunto(linea, linea.Foto), foto) })
}

func TestInternetLinkOpensWithWebRTC(t *testing.T) {
	a, b := nodoDePrueba(t, "Ana"), nodoDePrueba(t, "Beto")
	go a.Correr()
	go b.Correr()
	unirPorRadio(a, b)
	esperar(t, a, "se conocen", func() bool { return a.EnLinea(b.Clave()) })
	a.Hacer(func() { a.Red.MarcarYa(b.Clave()) })
	esperar(t, a, "enlace WebRTC arriba", func() bool { return a.Router.EsVecino(VecinoNet(b.Clave())) })
	esperar(t, b, "enlace WebRTC arriba del otro lado", func() bool { return b.Router.EsVecino(VecinoNet(a.Clave())) })
	// The radio goes away: a private message still crosses by the Internet link.
	a.Hacer(func() { a.Router.Quitar("radio") })
	b.Hacer(func() { b.Router.Quitar("radio") })
	a.Consultar(func() { a.EnviarMensaje(b.Clave(), "por internet", nil) })
	esperar(t, b, "mensaje privado", func() bool { return len(b.Chats[a.Clave()]) == 1 })
	esperar(t, a, "confirmado", func() bool { return a.Chats[b.Clave()][0].Estado == "ok" })
}
