package interfaz

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/nicoconvoz/zonda/zondago/nucleo"
)

func enviado(gtx layout.Context, e *widget.Editor) bool {
	sent := false
	for {
		ev, ok := e.Update(gtx)
		if !ok {
			return sent
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			sent = true
		}
	}
}

/* ---------- signup: only a nickname, man or woman, and an optional photo ---------- */

func (a *App) alta(gtx layout.Context) layout.Dimensions {
	if a.hombre.Clicked(gtx) {
		a.genero = "m"
	}
	if a.mujer.Clicked(gtx) {
		a.genero = "f"
	}
	if a.elegirFoto.Clicked(gtx) {
		a.elegirArchivo([]string{".jpg", ".jpeg", ".png"}, func(_ string, b []byte) { a.fotoAlta = b })
	}
	if (a.entrar.Clicked(gtx) || enviado(gtx, &a.nick)) && strings.TrimSpace(a.nick.Text()) != "" {
		a.n.CrearCuenta(a.nick.Text(), a.genero, a.fotoAlta)
		a.vista = pPrincipal
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return columna(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.H3(a.th, "Zonda")
				l.Color = acento
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.chico(gtx, "Salas de dispositivo a dispositivo, sin servidores. Experimento en Go.")
			}),
			sep(20),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.campo(gtx, &a.nick, "Tu apodo") }),
			sep(12),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				h, m := "Hombre", "Mujer"
				if a.genero == "m" {
					h = "● Hombre"
				} else {
					m = "● Mujer"
				}
				return fila(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.hombre, h) }),
					sep(8),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.mujer, m) }),
				)
			}),
			sep(12),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				s := "Elegir foto (opcional)"
				if a.fotoAlta != nil {
					s = "Foto elegida ✓"
				}
				return a.botonSuave(gtx, &a.elegirFoto, s)
			}),
			sep(20),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.boton(gtx, &a.entrar, "Entrar a Zonda") }),
		)
	})
}

/* ---------- the main screen: rooms, people, network ---------- */

func (a *App) principalUI(gtx layout.Context) layout.Dimensions {
	for i := range a.tabs {
		if a.tabs[i].Clicked(gtx) {
			a.tab = i
		}
	}
	if a.gotching.Clicked(gtx) {
		abrirURL("https://store.steampowered.com/app/5451020")
	}
	if a.wenas.Clicked(gtx) {
		abrirURL("https://wenas.app")
	}
	return columna(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return caja(gtx, panel, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					nombres := []string{"Salas", "Gente", "Red"}
					var hijos []layout.FlexChild
					for i, s := range nombres {
						i, s := i, s
						hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if a.tab == i {
								return a.boton(gtx, &a.tabs[i], s)
							}
							return a.botonSuave(gtx, &a.tabs[i], s)
						}), sep(6))
					}
					hijos = append(hijos, layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.gotching, "Gotching") }), sep(6),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.wenas, "Wenas") }), sep(10),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.avatar(gtx, a.n.Clave(), a.n.Nombre, 32) }), sep(6),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.texto(gtx, a.n.Nombre, tinta) }))
					return fila(gtx, hijos...)
				})
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return margen(gtx, func(gtx layout.Context) layout.Dimensions {
				switch a.tab {
				case 1:
					return a.gente(gtx)
				case 2:
					return a.red(gtx)
				}
				return a.salas(gtx)
			})
		}),
	)
}

func (a *App) salas(gtx layout.Context) layout.Dimensions {
	s := a.n.Salas()
	if (a.crear.Clicked(gtx) || enviado(gtx, &a.salaNombre)) && strings.TrimSpace(a.salaNombre.Text()) != "" {
		c := s.Crear(a.salaNombre.Text())
		a.salaNombre.SetText("")
		a.n.EntrarASala(c.ID)
		a.vista = pSala
	}
	if a.golpear.Clicked(gtx) || enviado(gtx, &a.codigo) {
		if c, ok := nucleo.CartaDeCodigo(a.codigo.Text()); ok {
			a.codigo.SetText("")
			if s.EsMiembro(c.ID, a.n.Clave()) {
				a.n.EntrarASala(c.ID)
				a.vista = pSala
			} else {
				s.Golpear(c)
				a.aviso = ""
			}
		} else {
			a.aviso = "Ese código no es de una sala de Zonda."
		}
	}
	if a.cancelarGolpe.Clicked(gtx) {
		s.DejarDeGolpear()
	}
	mias := s.Mias()
	for _, b := range mias {
		if a.clic("sala:" + b.ID()).Clicked(gtx) {
			a.n.EntrarASala(b.ID())
			a.vista = pSala
		}
	}
	items := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return a.titulo(gtx, "Crear una sala") },
		func(gtx layout.Context) layout.Dimensions {
			return fila(gtx, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.campo(gtx, &a.salaNombre, "Nombre de la sala") }), sep(8),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.boton(gtx, &a.crear, "Crear") }))
		},
		func(gtx layout.Context) layout.Dimensions { return a.titulo(gtx, "Entrar con código") },
		func(gtx layout.Context) layout.Dimensions {
			return fila(gtx, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.campo(gtx, &a.codigo, "Pegá el código ZR1:…") }), sep(8),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.boton(gtx, &a.golpear, "Pedir entrar") }))
		},
	}
	if a.aviso != "" {
		items = append(items, func(gtx layout.Context) layout.Dimensions { return a.texto(gtx, a.aviso, acento) })
	}
	if g := s.Golpe; g != nil {
		nombre := g.Name
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return fila(gtx, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.texto(gtx, "Esperando que alguien de «"+nombre+"» te deje entrar…", acento)
			}), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.cancelarGolpe, "Cancelar") }))
		})
	}
	items = append(items, func(gtx layout.Context) layout.Dimensions { return a.titulo(gtx, "Mis salas") })
	if len(mias) == 0 {
		items = append(items, func(gtx layout.Context) layout.Dimensions { return a.chico(gtx, "Todavía no estás en ninguna.") })
	}
	for _, b := range mias {
		b := b
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return material.Clickable(gtx, a.clic("sala:"+b.ID()), func(gtx layout.Context) layout.Dimensions {
				return caja(gtx, panel, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						rol := fmt.Sprintf("%d miembros", len(b.Miembros()))
						if b.EsCreador(a.n.Clave()) {
							rol += " · la creaste vos"
						}
						if p := len(a.n.Salas().Pedidos(b.ID())); p > 0 {
							rol += fmt.Sprintf(" · %d piden entrar", p)
						}
						return columna(gtx, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.texto(gtx, b.Nombre(), tinta) }),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.chico(gtx, rol) }))
					})
				})
			})
		})
	}
	return a.listado(gtx, &a.principal, items)
}

func (a *App) listado(gtx layout.Context, l *widget.List, items []layout.Widget) layout.Dimensions {
	return material.List(a.th, l).Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, items[i])
	})
}

/* ---------- people: the magnifier, contacts and who is around ---------- */

func (a *App) persona(gtx layout.Context, key, nombre string, on bool, extra ...layout.FlexChild) layout.Dimensions {
	return caja(gtx, panel, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			hijos := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.avatar(gtx, key, nombre, 36) }), sep(8),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return punto(gtx, on) }), sep(6),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.texto(gtx, nombre, tinta) }),
			}
			return fila(gtx, append(hijos, extra...)...)
		})
	})
}

func (a *App) gente(gtx layout.Context) layout.Dimensions {
	n := a.n
	if (a.buscarBtn.Clicked(gtx) || enviado(gtx, &a.buscar)) && strings.TrimSpace(a.buscar.Text()) != "" {
		n.Tablero.Buscar(a.buscar.Text())
	}
	abrir := func(k string) {
		a.chat = k
		a.vista = pChat
	}
	items := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return fila(gtx, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.campo(gtx, &a.buscar, "🔍 Buscar por apodo") }), sep(8),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.boton(gtx, &a.buscarBtn, "Buscar") }))
		},
	}
	var encontrados []string
	for k := range n.Encontrados {
		encontrados = append(encontrados, k)
	}
	sort.Strings(encontrados)
	for _, k := range encontrados {
		k := k
		if a.clic("agregar:" + k).Clicked(gtx) {
			n.AgregarContacto(k)
		}
		if a.clic("msj:" + k).Clicked(gtx) {
			abrir(k)
		}
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			extra := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, a.clic("msj:"+k), "Mensaje") })}
			if !n.Contactos[k] {
				extra = append(extra, sep(6), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.boton(gtx, a.clic("agregar:"+k), "Agregar") }))
			}
			return a.persona(gtx, k, n.NombreDe(k), n.EnLinea(k), extra...)
		})
	}
	items = append(items, func(gtx layout.Context) layout.Dimensions { return a.titulo(gtx, "Contactos") })
	var contactos []string
	for k := range n.Contactos {
		contactos = append(contactos, k)
	}
	sort.Slice(contactos, func(i, j int) bool { return strings.ToLower(n.NombreDe(contactos[i])) < strings.ToLower(n.NombreDe(contactos[j])) })
	if len(contactos) == 0 {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return a.chico(gtx, "Buscá a alguien por su apodo con la lupa, o conocelo en una sala.")
		})
	}
	for _, k := range contactos {
		k := k
		if a.clic("c:" + k).Clicked(gtx) {
			abrir(k)
		}
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return material.Clickable(gtx, a.clic("c:"+k), func(gtx layout.Context) layout.Dimensions {
				return a.persona(gtx, k, n.NombreDe(k), n.EnLinea(k))
			})
		})
	}
	return a.listado(gtx, &a.principal, items)
}

/* ---------- the network, to see where a link stops ---------- */

func (a *App) red(gtx layout.Context) layout.Dimensions {
	n := a.n
	if a.clic("copiarclave").Clicked(gtx) {
		a.copiarTexto(gtx, n.Clave())
	}
	items := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return a.titulo(gtx, "Tablero (Nostr)") },
	}
	for _, r := range n.Tablero.Relays() {
		r := r
		items = append(items, func(gtx layout.Context) layout.Dimensions { return a.texto(gtx, r, tinta) })
	}
	items = append(items, func(gtx layout.Context) layout.Dimensions {
		return a.titulo(gtx, fmt.Sprintf("Enlaces por Internet: %d", n.Red.Abiertos()))
	})
	estad := func(m map[string]int) {
		var ks []string
		for k := range m {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			s := fmt.Sprintf("%s: %d", k, m[k])
			items = append(items, func(gtx layout.Context) layout.Dimensions { return a.texto(gtx, s, tinta) })
		}
	}
	items = append(items, func(gtx layout.Context) layout.Dimensions { return a.chico(gtx, "Cómo se arman los enlaces") })
	estad(n.Red.Stats)
	estad(n.Tablero.Stats)
	items = append(items, func(gtx layout.Context) layout.Dimensions {
		return fila(gtx, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.chico(gtx, "Tu clave: "+n.Clave()) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, a.clic("copiarclave"), "Copiar") }))
	})
	return a.listado(gtx, &a.principal, items)
}

/* ---------- a room: chat on the left, members on the right ---------- */

func (a *App) sala(gtx layout.Context) layout.Dimensions {
	n := a.n
	room := n.SalaActual
	b := n.Salas().Libro(room)
	if b == nil {
		a.vista = pPrincipal
		return layout.Dimensions{}
	}
	me := n.Clave()
	if a.volver.Clicked(gtx) {
		if a.voz != nil && a.voz.Canal() != "" {
			a.voz.Salir()
		}
		n.SalirDeSala()
		a.vista = pPrincipal
		return layout.Dimensions{}
	}
	if a.copiar.Clicked(gtx) {
		a.copiarTexto(gtx, n.Salas().Codigo(room))
	}
	if a.cerrarSala.Clicked(gtx) && b.EsCreador(me) {
		n.Salas().Cerrar(room)
		a.vista = pPrincipal
		return layout.Dimensions{}
	}
	if a.enviar.Clicked(gtx) || enviado(gtx, &a.mensaje) {
		if t := strings.TrimSpace(a.mensaje.Text()); t != "" {
			n.DecirEnSala(t, nil, nil)
			a.mensaje.SetText("")
		}
	}
	if a.adjFoto.Clicked(gtx) {
		a.elegirArchivo([]string{".jpg", ".jpeg", ".png", ".gif"}, func(_ string, f []byte) {
			if n.SalaActual == room {
				n.DecirEnSala(a.mensaje.Text(), f, nil)
			}
		})
	}
	if a.archivo.Clicked(gtx) {
		a.elegirArchivo(nil, func(nombre string, f []byte) {
			if n.SalaActual == room {
				n.DecirEnSala("", nil, &nucleo.Archivo{Bytes: f, Nombre: nombre, Mime: "application/octet-stream", Tipo: "file"})
			}
		})
	}
	if b.EsCreador(me) && (a.fijarCanales.Clicked(gtx) || enviado(gtx, &a.canales)) {
		n.Salas().FijarCanales(room, strings.Split(a.canales.Text(), ","))
		a.canales.SetText("")
	}
	for _, p := range n.Salas().Pedidos(room) {
		if a.clic("si:" + p.Clave).Clicked(gtx) {
			n.Salas().Aprobar(room, p.Clave)
		}
		if a.clic("no:" + p.Clave).Clicked(gtx) {
			n.Salas().Rechazar(room, p.Clave)
		}
	}
	a.vozClicks(gtx, room, b)
	ancho := gtx.Constraints.Max.X >= gtx.Dp(900)
	return columna(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.cabeceraSala(gtx, b) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.pedidos(gtx, room) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.barraVoz(gtx, b) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !ancho {
				return a.charla(gtx)
			}
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, a.charla),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(280), gtx.Dp(280)
					return a.listaMiembros(gtx, room, b)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.escribir(gtx, true) }),
	)
}

func (a *App) cabeceraSala(gtx layout.Context, b *nucleo.Libro) layout.Dimensions {
	return caja(gtx, panel, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			hijos := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.volver, "← Salas") }), sep(10),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.titulo(gtx, b.Nombre()) }),
			}
			if a.aviso == "Copiado." {
				hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.chico(gtx, "Código copiado ") }))
			}
			hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.copiar, "Copiar código") }))
			if b.EsCreador(a.n.Clave()) {
				hijos = append(hijos, sep(6), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.cerrarSala, "Cerrar sala") }))
			}
			return fila(gtx, hijos...)
		})
	})
}

func (a *App) pedidos(gtx layout.Context, room string) layout.Dimensions {
	ps := a.n.Salas().Pedidos(room)
	if len(ps) == 0 {
		return layout.Dimensions{}
	}
	var hijos []layout.FlexChild
	for _, p := range ps {
		p := p
		hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fila(gtx, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.texto(gtx, p.Nick+" pide entrar a la sala", acento) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.boton(gtx, a.clic("si:"+p.Clave), "Dejar entrar") }), sep(6),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, a.clic("no:"+p.Clave), "No") }))
			})
		}))
	}
	return columna(gtx, hijos...)
}

func (a *App) charla(gtx layout.Context) layout.Dimensions {
	n := a.n
	lineas := n.LineasSala
	for i, l := range lineas {
		if l.Archivo != nil && a.clic(fmt.Sprintf("arch:%d", i)).Clicked(gtx) {
			if b := n.Adjunto(l, nucleo.Interpolar(l.Archivo["h"])); b != nil {
				a.guardarArchivo(nucleo.Interpolar(l.Archivo["n"]), b)
			}
		}
	}
	if len(lineas) == 0 {
		return margen(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.chico(gtx, "Lo que se diga mientras estás adentro aparece acá. Viaja cifrado con la llave de la sala.")
		})
	}
	return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.List(a.th, &a.lista).Layout(gtx, len(lineas), func(gtx layout.Context, i int) layout.Dimensions {
			l := lineas[i]
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if l.Sistema {
					return a.chico(gtx, l.Texto)
				}
				return fila(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.avatar(gtx, l.Por, l.Nick, 30) }), sep(8),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return columna(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.chico(gtx, n.NombreDe(l.Por)+" · "+hora(l.At))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if l.Foto != "" && (l.Texto == "🖼️") {
									return layout.Dimensions{}
								}
								return a.texto(gtx, l.Texto, tinta)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if l.Foto == "" {
									return layout.Dimensions{}
								}
								return a.foto(gtx, fmt.Sprintf("%s|%d|%s", l.Sala, l.Epoca, l.Foto), n.Adjunto(l, l.Foto), 240)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if l.Archivo == nil {
									return layout.Dimensions{}
								}
								s := "Guardar " + nucleo.Interpolar(l.Archivo["n"])
								if n.Adjunto(l, nucleo.Interpolar(l.Archivo["h"])) == nil {
									s = "Recibiendo " + nucleo.Interpolar(l.Archivo["n"]) + "…"
								}
								return a.botonSuave(gtx, a.clic(fmt.Sprintf("arch:%d", i)), s)
							}),
						)
					}),
				)
			})
		})
	})
}

func (a *App) listaMiembros(gtx layout.Context, room string, b *nucleo.Libro) layout.Dimensions {
	n := a.n
	me := n.Clave()
	ms := n.MiembrosSala(room)
	for _, m := range ms {
		if a.clic("echar:"+m.Clave).Clicked(gtx) {
			n.Salas().Echar(room, m.Clave, false)
		}
		if a.clic("banear:"+m.Clave).Clicked(gtx) {
			n.Salas().Echar(room, m.Clave, true)
		}
		if a.clic("hablar:"+m.Clave).Clicked(gtx) && m.Clave != me {
			n.AgregarContacto(m.Clave)
			a.chat = m.Clave
			a.vista = pChat
		}
	}
	paint.FillShape(gtx.Ops, panel, clipRect(gtx.Constraints.Max))
	return material.List(a.th, &a.miembros).Layout(gtx, len(ms)+1, func(gtx layout.Context, i int) layout.Dimensions {
		return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if i == 0 {
				return a.chico(gtx, fmt.Sprintf("MIEMBROS — %d", len(ms)))
			}
			m := ms[i-1]
			nombre := m.Nombre
			if m.Creador {
				nombre += " ★"
			}
			hijos := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.avatar(gtx, m.Clave, m.Nombre, 28) }), sep(6),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return punto(gtx, m.EnLinea) }), sep(6),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Clickable(gtx, a.clic("hablar:"+m.Clave), func(gtx layout.Context) layout.Dimensions {
						c := tinta
						if !m.EnLinea {
							c = gris
						}
						return a.texto(gtx, nombre, c)
					})
				}),
			}
			if b.EsCreador(me) && m.Clave != me {
				hijos = append(hijos,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.mini(gtx, a.clic("echar:"+m.Clave), "Echar") }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.mini(gtx, a.clic("banear:"+m.Clave), "Ban") }))
			}
			return fila(gtx, hijos...)
		})
	})
}

func (a *App) mini(gtx layout.Context, c *widget.Clickable, s string) layout.Dimensions {
	return material.Clickable(gtx, c, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(a.th, s)
			l.Color = acento
			return l.Layout(gtx)
		})
	})
}

func (a *App) escribir(gtx layout.Context, conArchivo bool) layout.Dimensions {
	return caja(gtx, panel, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			hijos := []layout.FlexChild{
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return caja(gtx, fondo, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.UniformInset(unit.Dp(8)).Layout(gtx, material.Editor(a.th, &a.mensaje, "Escribí y Enter").Layout)
					})
				}), sep(6),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.adjFoto, "Foto") }), sep(6),
			}
			if conArchivo {
				hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.archivo, "Archivo") }), sep(6))
			}
			hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.boton(gtx, &a.enviar, "Enviar") }))
			return fila(gtx, hijos...)
		})
	})
}

/* ---------- a private chat ---------- */

func (a *App) chatUI(gtx layout.Context) layout.Dimensions {
	n, k := a.n, a.chat
	if a.volver.Clicked(gtx) {
		a.vista = pPrincipal
		if n.SalaActual != "" {
			a.vista = pSala
		}
		return layout.Dimensions{}
	}
	if a.enviar.Clicked(gtx) || enviado(gtx, &a.mensaje) {
		if t := strings.TrimSpace(a.mensaje.Text()); t != "" {
			n.EnviarMensaje(k, t, nil)
			a.mensaje.SetText("")
		}
	}
	if a.adjFoto.Clicked(gtx) {
		a.elegirArchivo([]string{".jpg", ".jpeg", ".png", ".gif"}, func(_ string, f []byte) { n.EnviarMensaje(k, a.mensaje.Text(), f) })
	}
	lineas := n.Chats[k]
	return columna(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return caja(gtx, panel, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return fila(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.volver, "← Volver") }), sep(10),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.avatar(gtx, k, n.NombreDe(k), 32) }), sep(8),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return punto(gtx, n.EnLinea(k)) }), sep(6),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.titulo(gtx, n.NombreDe(k)) }))
				})
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.List(a.th, &a.lista).Layout(gtx, len(lineas), func(gtx layout.Context, i int) layout.Dimensions {
					l := lineas[i]
					dir := layout.W
					c := panel
					if l.Mia {
						dir, c = layout.E, color.NRGBA{0xd5, 0xe1, 0xe6, 0xff}
					}
					return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return dir.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = 0
							gtx.Constraints.Max.X = gtx.Constraints.Max.X * 3 / 4
							return caja(gtx, c, func(gtx layout.Context) layout.Dimensions {
								return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									estado := hora(l.At)
									switch l.Estado {
									case "sending":
										estado += " · enviando"
									case "ok":
										estado += " · ✓"
									}
									return columna(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if l.Foto == "" {
												return layout.Dimensions{}
											}
											b := n.Foto(l.Foto)
											if b == nil && !l.Mia {
												n.PedirFoto(l.Foto, k)
											}
											return a.foto(gtx, l.Foto, b, 240)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if l.Texto == "" {
												return layout.Dimensions{}
											}
											return a.texto(gtx, l.Texto, tinta)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.chico(gtx, estado) }))
								})
							})
						})
					})
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.escribir(gtx, false) }),
	)
}

var _ = image.Pt
