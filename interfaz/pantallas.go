package interfaz

import (
	"fmt"

	"image"
	"image/color"
	"sort"
	"strings"

	"gioui.org/op/clip"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
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

// The main screen, as in the app: a bar of the skin's color with icons (the active one in its own color), the
// links to Gotching and Wenas on the right, and the section below.

// estilo: the skins, each with its four colors, as the app's skins list shows them.
func (a *App) estilo(gtx layout.Context) layout.Dimensions {
	for _, p := range pieles {
		if a.clic("piel:" + p.ID).Clicked(gtx) {
			a.aplicarPiel(p.ID)
			a.n.Alm.Escribir("piel", p.ID)
		}
	}
	items := []layout.Widget{func(gtx layout.Context) layout.Dimensions { return a.titulo(gtx, "Estilo") }}
	for _, p := range pieles {
		p := p
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return a.clic("piel:"+p.ID).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return caja(gtx, piel.Surface, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						var hijos []layout.FlexChild
						for _, c := range p.Muestra {
							c := c
							hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								s := gtx.Dp(26)
								rr := clip.UniformRRect(image.Rectangle{Max: image.Pt(s, s)}, gtx.Dp(6))
								paint.FillShape(gtx.Ops, c, rr.Op(gtx.Ops))
								paint.FillShape(gtx.Ops, piel.Line, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
								return layout.Dimensions{Size: image.Pt(s, s)}
							}), sep(4))
						}
						nombre := p.Nombre
						if p.ID == piel.ID {
							nombre += "  ✓"
						}
						hijos = append(hijos, sep(10), layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return columna(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.titulo(gtx, nombre) }),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.chico(gtx, p.Origen) }))
						}))
						return fila(gtx, hijos...)
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
	sort.Slice(contactos, func(i, j int) bool {
		return strings.ToLower(n.NombreDe(contactos[i])) < strings.ToLower(n.NombreDe(contactos[j]))
	})
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

func (a *App) mini(gtx layout.Context, c *widget.Clickable, s string) layout.Dimensions {
	return material.Clickable(gtx, c, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(a.th, s)
			l.Color = acento
			return l.Layout(gtx)
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
