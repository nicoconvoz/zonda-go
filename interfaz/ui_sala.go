package interfaz

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/nicoconvoz/zonda/zondago/nucleo"
	"github.com/nicoconvoz/zonda/zondago/voz"
)

/* ---------- a room (features/salas/presentation/room_screen.dart) ---------- */

// From this width the users list sits on the right of the chat (room_screen.dart: wideMin 900, usersWidth 280).
const anchoConLista, anchoLista = 900, 280

func (a *App) canalActual(b *nucleo.Libro) string {
	for _, c := range b.Canales() {
		if c == a.canalElegido {
			return c
		}
	}
	return b.Canales()[0]
}

func (a *App) sala(gtx layout.Context) layout.Dimensions {
	n := a.n
	room := n.SalaActual
	b := n.Salas().Libro(room)
	if b == nil {
		a.vista = pPrincipal
		return layout.Dimensions{}
	}
	me := n.Clave()
	creador := b.EsCreador(me)
	enVoz := a.voz != nil && a.voz.Canal() != ""
	idVoz := room + "/" + a.canalActual(b)

	if a.clic("s:salir").Clicked(gtx) {
		if enVoz {
			a.voz.Salir()
		}
		n.SalirDeSala()
		a.vista = pPrincipal
		return layout.Dimensions{}
	}
	for _, t := range []string{"chat", "users", "priv"} {
		if a.clic("s:" + t).Clicked(gtx) {
			a.salaTab, a.menuDe = t, ""
		}
	}
	if a.voz != nil {
		if a.clic("s:audio").Clicked(gtx) {
			if enVoz {
				a.voz.Salir()
			} else if err := a.voz.Entrar(idVoz, n.EnSala(room)); err != nil {
				a.aviso = "No se pudo abrir el audio: " + err.Error()
			}
		}
		if a.clic("s:mic").Clicked(gtx) {
			a.voz.AlternarHablar()
		}
		if a.clic("s:pantalla").Clicked(gtx) {
			if err := a.voz.AlternarPantalla(); err != nil {
				a.aviso = err.Error()
			}
		}
	}
	if a.clic("s:codigo").Clicked(gtx) {
		a.copiarTexto(gtx, n.Salas().Codigo(room))
		a.aviso = "Código copiado. Quien lo pegue en «Entrar con código» pide permiso para entrar."
	}
	if creador && a.clic("s:cerrar").Clicked(gtx) {
		if enVoz {
			a.voz.Salir()
		}
		n.Salas().Cerrar(room)
		a.vista = pPrincipal
		return layout.Dimensions{}
	}
	for _, c := range b.Canales() {
		if a.clic("canal:"+c).Clicked(gtx) && c != a.canalActual(b) {
			a.canalElegido = c
			if enVoz {
				a.voz.Salir()
				a.voz.Entrar(room+"/"+c, n.EnSala(room))
			}
		}
	}
	if creador && a.clic("s:nuevocanal").Clicked(gtx) {
		a.abrirDialogo("canal")
	}
	for _, p := range n.Salas().Pedidos(room) {
		if a.clic("si:" + p.Clave).Clicked(gtx) {
			n.Salas().Aprobar(room, p.Clave)
		}
		if a.clic("no:" + p.Clave).Clicked(gtx) {
			n.Salas().Rechazar(room, p.Clave)
		}
	}
	a.clicsComponer(gtx, room)
	a.clicsMiembros(gtx, room, b)
	for k := range a.voz.Pantallas() {
		if a.clic("pant:" + k).Clicked(gtx) {
			if a.pantallaGrande == k {
				a.pantallaGrande = ""
			} else {
				a.pantallaGrande = k
			}
		}
	}

	miembros := n.MiembrosSala(room)
	tituloRegla := "Sala " + b.Nombre()
	switch a.salaTab {
	case "users":
		tituloRegla = fmt.Sprintf("Usuarios (%d)", len(miembros))
	case "priv":
		tituloRegla = "Privados (0)"
	}
	return columna(gtx,
		// The row of squares: the room's tabs on the left, its actions on the right.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(8), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				hijos := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.cuadrado(gtx, a.clic("s:salir"), "salas", false) }), sep(10),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.cuadrado(gtx, a.clic("s:chat"), "chat", a.salaTab == "chat")
					}), sep(10),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.cuadrado(gtx, a.clic("s:users"), "users", a.salaTab == "users")
					}), sep(10),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.cuadrado(gtx, a.clic("s:priv"), "privados", a.salaTab == "priv")
					}),
					layout.Flexed(1, layout.Spacer{}.Layout),
				}
				icAudio := "mute"
				if enVoz {
					icAudio = "volume"
				}
				if a.voz != nil {
					hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.cuadrado(gtx, a.clic("s:audio"), icAudio, enVoz) }), sep(10))
					if enVoz {
						hijos = append(hijos,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.cuadrado(gtx, a.clic("s:mic"), "mic", a.voz.Hablando())
							}), sep(10),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.cuadrado(gtx, a.clic("s:pantalla"), "screen", a.voz.Compartiendo())
							}), sep(10))
					}
				}
				hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.cuadrado(gtx, a.clic("s:codigo"), "postular", false)
				}))
				if creador {
					hijos = append(hijos, sep(10), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.cuadrado(gtx, a.clic("s:cerrar"), "trash", false) }))
				}
				return fila(gtx, hijos...)
			})
		}),
		// The rule with the room's name, and the signal meter.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fila(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.regla(gtx, tituloRegla, "") }), sep(10),
					layout.Rigid(a.medidor))
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.fichasCanales(gtx, b, creador) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.pantallas(gtx) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !enVoz {
				return layout.Dimensions{}
			}
			estado := " · escuchando"
			if a.voz.Hablando() {
				estado = " · estás hablando"
			}
			return a.franja(gtx, fmt.Sprintf("Audio de la sala · %d conectados%s", a.voz.Conectados(), estado), a.voz.Medidas())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.aviso == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.texto(gtx, a.aviso, piel.Ink)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.pedidosSala(gtx, room) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			cuerpo := func(gtx layout.Context) layout.Dimensions {
				switch a.salaTab {
				case "users":
					return a.listaMiembros(gtx, miembros, b)
				case "priv":
					return a.vacio(gtx, "privados", "Abrí un privado desde la lista de usuarios de la sala.")
				}
				return a.charla(gtx, b)
			}
			if gtx.Constraints.Max.X < dp(gtx, anchoConLista) || a.salaTab == "users" {
				return cuerpo(gtx)
			}
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, cuerpo),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					w := dp(gtx, anchoLista)
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
					paint.FillShape(gtx.Ops, piel.Line, clip.Rect{Max: image.Pt(max(1, dp(gtx, 1)), gtx.Constraints.Max.Y)}.Op())
					st := op.Offset(image.Pt(max(1, dp(gtx, 1)), 0)).Push(gtx.Ops)
					gtx.Constraints.Max.X -= max(1, dp(gtx, 1))
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					d := a.listaMiembros(gtx, miembros, b)
					st.Pop()
					return layout.Dimensions{Size: image.Pt(w, d.Size.Y)}
				}))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.salaTab != "chat" {
				return layout.Dimensions{}
			}
			return a.escribir(gtx, true)
		}),
	)
}

// medidor is _SignalMeter: on green, the room's voices (rising bars) and mine (falling bars), 8 each.
func (a *App) medidor(gtx layout.Context) layout.Dimensions {
	var sala, mio float32
	if a.voz != nil {
		sala, mio = a.voz.Niveles()
	}
	barras := func(gtx layout.Context, alturas []float32, encendidas int, desdeElFinal bool) layout.Dimensions {
		w, h, pad := dp(gtx, 36), dp(gtx, 20), dp(gtx, 2)
		paint.FillShape(gtx.Ops, hx(0xFFFFFFFF), clip.UniformRRect(image.Rectangle{Max: image.Pt(w, h)}, dp(gtx, 3)).Op(gtx.Ops))
		n := len(alturas)
		ancho := (float32(w-2*pad) - 1.5*float32(n-1)*float32(dp(gtx, 1))) / float32(n)
		for i, al := range alturas {
			on := i < encendidas
			if desdeElFinal {
				on = i >= n-encendidas
			}
			c := hx(0xFF9A9A9A)
			if on {
				c = hx(0xFF121212)
			}
			x0 := pad + int(float32(i)*(ancho+1.5*float32(dp(gtx, 1))))
			bh := dp(gtx, al)
			paint.FillShape(gtx.Ops, c, clip.UniformRRect(image.Rectangle{Min: image.Pt(x0, h-pad-bh), Max: image.Pt(x0+int(ancho), h-pad)}, dp(gtx, 1)).Op(gtx.Ops))
		}
		return layout.Dimensions{Size: image.Pt(w, h)}
	}
	m := op.Record(gtx.Ops)
	d := layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fila(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return barras(gtx, []float32{3, 5, 6, 8, 9, 11, 13, 15}, voz.Barras(sala), false)
			}), sep(3),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return barras(gtx, []float32{15, 13, 10, 8, 6, 5, 4, 3}, voz.Barras(mio), true)
			}))
	})
	call := m.Stop()
	rr := clip.UniformRRect(image.Rectangle{Max: d.Size}, dp(gtx, 9))
	paint.FillShape(gtx.Ops, hx(0xFF2FBF4A), rr.Op(gtx.Ops))
	borde := piel.Ink
	borde.A = 0xB3
	paint.FillShape(gtx.Ops, borde, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(dp(gtx, 2))}.Op())
	call.Add(gtx.Ops)
	return d
}

// canales: the voice channel chips (the chosen one with its check), and "+" for the creator.
func (a *App) fichasCanales(gtx layout.Context, b *nucleo.Libro, creador bool) layout.Dimensions {
	actual := a.canalActual(b)
	return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		var hijos []layout.FlexChild
		for _, c := range b.Canales() {
			c := c
			hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.ficha(gtx, a.clic("canal:"+c), c, c == actual)
				})
			}))
		}
		if creador {
			hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.iconoBoton(gtx, a.clic("s:nuevocanal"), "plus", 36, 20, piel.Ink)
			}))
		}
		return fila(gtx, hijos...)
	})
}

// ficha is a ChoiceChip: rounded, the chosen one tinted and checked.
func (a *App) ficha(gtx layout.Context, c *widget.Clickable, s string, elegida bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		d := layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(12), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			hijos := []layout.FlexChild{}
			if elegida {
				hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icono2(gtx, "check", 16, piel.Ink) }), sep(8))
			}
			// The app writes "🔊 name"; Gio's fonts carry no emoji, so the speaker is the app's own icon.
			hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icono2(gtx, "volume", 16, piel.Ink) }), sep(6))
			hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.th, unit.Sp(14), s)
				l.Color, l.Font.Weight = piel.Ink, font.Medium
				return l.Layout(gtx)
			}))
			return fila(gtx, hijos...)
		})
		call := m.Stop()
		rr := clip.UniformRRect(image.Rectangle{Max: d.Size}, dp(gtx, 8))
		bg := piel.Surface
		if elegida {
			bg = mezcla(piel.Surface, piel.On, .28)
		}
		paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
		paint.FillShape(gtx.Ops, piel.Line, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(dp(gtx, 1))}.Op())
		call.Add(gtx.Ops)
		return d
	})
}

// franja: the green strip of the room's audio, with the voice counters under it.
func (a *App) franja(gtx layout.Context, s, medidas string) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return columna(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.th, unit.Sp(12.5), s)
				l.Color, l.Font.Weight = piel.Ink, font.SemiBold
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.th, unit.Sp(11), medidas)
				l.Color = piel.Muted
				return l.Layout(gtx)
			}))
	})
	call := m.Stop()
	c := hx(0xFF2FBF4A)
	c.A = 0x2E
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: d.Size}.Op())
	call.Add(gtx.Ops)
	return d
}

// pedidosSala: someone asking to enter, with ✕ and ✓, as in the app.
func (a *App) pedidosSala(gtx layout.Context, room string) layout.Dimensions {
	ps := a.n.Salas().Pedidos(room)
	var hijos []layout.FlexChild
	for _, p := range ps {
		p := p
		nick := p.Nick
		if nick == "" {
			nick = "Alguien"
		}
		hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return caja(gtx, piel.Surface, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(14), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fila(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								l := material.Label(a.th, unit.Sp(15), nick+" pide entrar a la sala")
								l.Color, l.Font.Weight = piel.Ink, font.SemiBold
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.iconoBoton(gtx, a.clic("no:"+p.Clave), "x", 36, 20, piel.Ink)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.iconoBoton(gtx, a.clic("si:"+p.Clave), "check", 36, 20, hx(0xFF2FBF4A))
							}))
					})
				})
			})
		}))
	}
	return columna(gtx, hijos...)
}

// charla: the room's lines, newest at the bottom; each one on the card color of its writer's gender. The welcome
// line goes first.
func (a *App) charla(gtx layout.Context, b *nucleo.Libro) layout.Dimensions {
	n := a.n
	lineas := n.LineasSala
	for i, l := range lineas {
		if l.Archivo != nil && a.clic(fmt.Sprintf("arch:%d", i)).Clicked(gtx) {
			if bs := n.Adjunto(l, nucleo.Interpolar(l.Archivo["h"])); bs != nil {
				a.guardarArchivo(nucleo.Interpolar(l.Archivo["n"]), bs)
			}
		}
	}
	bienvenida := "Bienvenido a " + b.Nombre() + ". Los mensajes, fotos y archivos viajan cifrados de dispositivo en dispositivo: solo los miembros de la sala los pueden abrir."
	return material.List(a.th, &a.lista).Layout(gtx, len(lineas)+1, func(gtx layout.Context, i int) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		if i == 0 {
			return a.lineaCampana(gtx, bienvenida)
		}
		l := lineas[i-1]
		if l.Sistema {
			return a.lineaCampana(gtx, l.Texto)
		}
		return a.lineaPersona(gtx, i-1, l)
	})
}

func (a *App) lineaCampana(gtx layout.Context, s string) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return fila(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icono2(gtx, "bell", 17, piel.Ink) }), sep(10),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.th, unit.Sp(14), s)
				l.Color = piel.Muted
				return l.Layout(gtx)
			}))
	})
	call := m.Stop()
	paint.FillShape(gtx.Ops, piel.Surface2, clip.Rect{Max: d.Size}.Op())
	paint.FillShape(gtx.Ops, piel.Line, clip.Rect{Min: image.Pt(0, d.Size.Y-1), Max: d.Size}.Op())
	call.Add(gtx.Ops)
	return d
}

func (a *App) lineaPersona(gtx layout.Context, i int, l nucleo.LineaSala) layout.Dimensions {
	n := a.n
	bg, ink := piel.Dark, piel.DarkInk
	if l.Genero == "f" {
		bg, ink = piel.Light, piel.LightInk
	}
	m := op.Record(gtx.Ops)
	d := layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		media := l.Foto != "" || l.Archivo != nil
		return columna(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lb := material.Label(a.th, unit.Sp(15), l.Nick+":")
				lb.Color, lb.Font.Weight = ink, font.Bold
				return lb.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if media && (l.Texto == "🖼️" || strings.HasPrefix(l.Texto, "📎 ")) {
					return layout.Dimensions{}
				}
				lb := material.Label(a.th, unit.Sp(15), l.Texto)
				lb.Color = ink
				return lb.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if l.Foto == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.foto(gtx, fmt.Sprintf("%s|%d|%s", l.Sala, l.Epoca, l.Foto), n.Adjunto(l, l.Foto), 240)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if l.Archivo == nil {
					return layout.Dimensions{}
				}
				nombre := nucleo.Interpolar(l.Archivo["n"])
				if n.Adjunto(l, nucleo.Interpolar(l.Archivo["h"])) == nil {
					nombre = "Trayendo " + nombre + "…"
				}
				return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.clic(fmt.Sprintf("arch:%d", i)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fila(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icono2(gtx, "clip", 18, ink) }), sep(6),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lb := material.Label(a.th, unit.Sp(14), nombre)
								lb.Color = ink
								return lb.Layout(gtx)
							}))
					})
				})
			}))
	})
	call := m.Stop()
	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: d.Size}.Op())
	if l.Genero == "f" {
		paint.FillShape(gtx.Ops, piel.Line, clip.Rect{Min: image.Pt(0, d.Size.Y-1), Max: d.Size}.Op())
	}
	call.Add(gtx.Ops)
	return d
}

// clicsMiembros handles the "⋯" menu of each member: a private message, and kick or ban for the creator.
func (a *App) clicsMiembros(gtx layout.Context, room string, b *nucleo.Libro) {
	for _, m := range a.n.MiembrosSala(room) {
		k := m.Clave
		if a.clic("menu:" + k).Clicked(gtx) {
			if a.menuDe == k {
				a.menuDe = ""
			} else {
				a.menuDe = k
			}
		}
		if a.clic("priv:" + k).Clicked(gtx) {
			a.n.AgregarContacto(k)
			a.chat, a.vista, a.menuDe = k, pChat, ""
		}
		if a.clic("echar:" + k).Clicked(gtx) {
			a.n.Salas().Echar(room, k, false)
			a.menuDe = ""
		}
		if a.clic("banear:" + k).Clicked(gtx) {
			a.n.Salas().Echar(room, k, true)
			a.menuDe = ""
		}
	}
}

// listaMiembros: each row is the person's icon and name on their card color, with everything else in "⋯".
func (a *App) listaMiembros(gtx layout.Context, ms []nucleo.Miembro, b *nucleo.Libro) layout.Dimensions {
	me := a.n.Clave()
	creador := b.EsCreador(me)
	return material.List(a.th, &a.miembros).Layout(gtx, len(ms), func(gtx layout.Context, i int) layout.Dimensions {
		m := ms[i]
		mio := m.Clave == me
		bg, ink, linea := piel.Dark, piel.DarkInk, hx(0xFF333333)
		ic := "perfil"
		if m.Genero == "f" {
			bg, ink, linea, ic = piel.Light, piel.LightInk, piel.Line, "perfilF"
		}
		nombre := m.Nombre
		if mio {
			nombre += " (vos)"
		}
		if !m.EnLinea && !mio {
			ink = mezcla(ink, bg, .45)
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		mm := op.Record(gtx.Ops)
		d := layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(14), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return columna(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					hijos := []layout.FlexChild{
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icono2(gtx, ic, 24, ink) }), sep(10),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.th, unit.Sp(15), nombre)
							l.Color, l.Font.Weight, l.MaxLines = ink, font.Bold, 1
							return l.Layout(gtx)
						}),
					}
					if mio {
						hijos = append(hijos, layout.Rigid(layout.Spacer{Width: unit.Dp(40), Height: unit.Dp(36)}.Layout))
					} else {
						hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.clic("menu:"+m.Clave).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								// "⋯" drawn as three dots: the skins' fonts have no such glyph.
								w, h, r := dp(gtx, 40), dp(gtx, 36), dp(gtx, 2)
								for k := -1; k <= 1; k++ {
									cx, cy := w/2+k*dp(gtx, 6), h/2
									paint.FillShape(gtx.Ops, ink, clip.Ellipse{Min: image.Pt(cx-r, cy-r), Max: image.Pt(cx+r, cy+r)}.Op(gtx.Ops))
								}
								return layout.Dimensions{Size: image.Pt(w, h)}
							})
						}))
					}
					return fila(gtx, hijos...)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if a.menuDe != m.Clave {
						return layout.Dimensions{}
					}
					opciones := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.opcion(gtx, a.clic("priv:"+m.Clave), "Mensaje privado", ink)
					})}
					if creador {
						opciones = append(opciones,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.opcion(gtx, a.clic("echar:"+m.Clave), "Echar de la sala", ink)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.opcion(gtx, a.clic("banear:"+m.Clave), "Banear de la sala", ink)
							}))
					}
					return columna(gtx, opciones...)
				}))
		})
		call := mm.Stop()
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: d.Size}.Op())
		paint.FillShape(gtx.Ops, linea, clip.Rect{Min: image.Pt(0, d.Size.Y-1), Max: d.Size}.Op())
		call.Add(gtx.Ops)
		return d
	})
}

func (a *App) opcion(gtx layout.Context, c *widget.Clickable, s string, ink color.NRGBA) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(34)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Label(a.th, unit.Sp(14), s)
			l.Color = ink
			return l.Layout(gtx)
		})
	})
}

/* ---------- the composer: a pill with smileys, photo and file, and the round send button ---------- */

func (a *App) clicsComponer(gtx layout.Context, room string) {
	n := a.n
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
}

func (a *App) escribir(gtx layout.Context, conArchivo bool) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(12), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fila(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				m := op.Record(gtx.Ops)
				d := layout.Inset{Left: unit.Dp(6), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					hijos := []layout.FlexChild{
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.iconoBoton(gtx, a.clic("emojis"), "masks", 36, 22, piel.Muted)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.iconoBoton(gtx, &a.adjFoto, "image", 36, 22, piel.Muted)
						}),
					}
					if conArchivo {
						hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.iconoBoton(gtx, &a.archivo, "clip", 36, 22, piel.Muted)
						}))
					}
					hijos = append(hijos, sep(6), layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(a.th, &a.mensaje, "Escribir mensaje")
							ed.Color, ed.HintColor = piel.Ink, piel.Muted
							return ed.Layout(gtx)
						})
					}))
					return fila(gtx, hijos...)
				})
				call := m.Stop()
				r := d.Size.Y / 2
				sombraDe(gtx, d.Size, r, false)
				rr := clip.UniformRRect(image.Rectangle{Max: d.Size}, r)
				paint.FillShape(gtx.Ops, piel.Surface, rr.Op(gtx.Ops))
				paint.FillShape(gtx.Ops, piel.Line, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(dp(gtx, 1.5))}.Op())
				call.Add(gtx.Ops)
				return d
			}),
			sep(8),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.fab(gtx, &a.enviar, "send") }))
	})
}
