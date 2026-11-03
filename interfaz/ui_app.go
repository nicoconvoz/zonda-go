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

	"github.com/nicoconvoz/zonda-go/nucleo"
)

/* ---------- the shell (app/shell.dart): the bar with the three sections ---------- */

// secciones: app/sections.dart, in its order, without Chat (private conversations open from a room) and without Apagar (quitting is in the tray icon's menu).
var secciones = []struct{ id, icono, nombre string }{
	{"salas", "salas", "Salas"}, {"usuarios", "usuarios", "Usuarios"}, {"ajustes", "gear", "Configuración"},
}

func (a *App) principalUI(gtx layout.Context) layout.Dimensions {
	for i := range a.tabs {
		if a.tabs[i].Clicked(gtx) {
			a.tab = i
		}
	}
	return columna(gtx,
		layout.Rigid(a.barra),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			switch secciones[a.tab].id {
			case "usuarios":
				return a.usuarios(gtx)
			case "ajustes":
				return a.ajustes(gtx)
			}
			return a.salas(gtx)
		}),
	)
}

// barra is the shell's top bar: the skin's bar color, one button per section (46 px high, a 30 px icon, a small
// line under the active one).
func (a *App) barra(gtx layout.Context) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(10), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		var hijos []layout.FlexChild
		for i, sec := range secciones {
			i, sec := i, sec
			hijos = append(hijos, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.tabs[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					w, h := gtx.Constraints.Max.X, dp(gtx, 46)
					on := a.tab == i
					c := piel.BarInk
					if on {
						c = piel.BarActive
					}
					ic := dp(gtx, 30)
					st := op.Offset(image.Pt((w-ic)/2, (h-ic)/2)).Push(gtx.Ops)
					dibujarIcono(gtx, sec.icono, ic, c)
					st.Pop()
					if on {
						lw, lh := dp(gtx, 18), dp(gtx, 3)
						rr := clip.UniformRRect(image.Rectangle{Min: image.Pt((w-lw)/2, h-lh), Max: image.Pt((w+lw)/2, h)}, dp(gtx, 2))
						paint.FillShape(gtx.Ops, piel.BarActive, rr.Op(gtx.Ops))
					}
					return layout.Dimensions{Size: image.Pt(w, h)}
				})
			}))
		}
		return layout.Flex{}.Layout(gtx, hijos...)
	})
	call := m.Stop()
	paint.FillShape(gtx.Ops, piel.Bar, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, d.Size.Y)}.Op())
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, d.Size.Y)}
}

// fondoPagina paints the skin's page background behind a section.
func fondoPagina(gtx layout.Context) {
	paint.FillShape(gtx.Ops, piel.AppBg, clip.Rect{Max: gtx.Constraints.Max}.Op())
}

/* ---------- Salas (features/salas/presentation/salas_screen.dart) ---------- */

func (a *App) salas(gtx layout.Context) layout.Dimensions {
	s := a.n.Salas()
	if a.gotching.Clicked(gtx) {
		abrirURL("https://store.steampowered.com/app/5451020")
	}
	if a.wenas.Clicked(gtx) {
		abrirURL("https://wenas.app")
	}
	if a.crear.Clicked(gtx) {
		a.abrirDialogo("crear")
	}
	if a.golpear.Clicked(gtx) {
		a.abrirDialogo("codigo")
	}
	if a.cancelarGolpe.Clicked(gtx) {
		s.DejarDeGolpear()
	}
	mias := s.Mias()
	for _, b := range mias {
		if a.clic("sala:" + b.ID()).Clicked(gtx) {
			a.entrarSala(b.ID())
		}
		if a.clic("cod:" + b.ID()).Clicked(gtx) {
			a.copiarTexto(gtx, s.Codigo(b.ID()))
			a.aviso = "Código de " + b.Nombre() + " copiado. Quien lo pegue en «Entrar con código» pide permiso para entrar."
		}
	}
	items := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.botonAncho(gtx, &a.crear, "plus", "Crear sala", true)
					}),
					sep(8),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.botonAncho(gtx, &a.golpear, "next", "Entrar con código", false)
					}))
			})
		},
	}
	if a.aviso != "" {
		aviso := a.aviso
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.texto(gtx, aviso, piel.Ink)
			})
		})
	}
	if g := s.Golpe; g != nil {
		nombre := g.Name
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return caja(gtx, piel.Surface, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fila(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.texto(gtx, "Esperando que te dejen entrar a "+nombre+". Te puede aceptar cualquier miembro que esté adentro.", piel.Ink)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.iconoBoton(gtx, &a.cancelarGolpe, "x", 36, 20, piel.Ink)
							}))
					})
				})
			})
		})
	}
	cola := ""
	if len(mias) > 0 {
		cola = fmt.Sprint(len(mias))
	}
	items = append(items, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return a.regla(gtx, "Mis salas", cola) })
	})
	if len(mias) == 0 {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return a.vacio(gtx, "salas", "Creá una sala o pedí entrar a una con su código. Se guardan en este dispositivo.")
		})
	}
	me := a.n.Clave()
	for _, b := range mias {
		b := b
		var partes []string
		if b.EsCreador(me) {
			partes = append(partes, "Creada por vos")
		}
		partes = append(partes, fmt.Sprintf("%d miembros", len(b.Miembros())))
		if p := len(s.Pedidos(b.ID())); p > 0 {
			partes = append(partes, fmt.Sprintf("%d pidiendo entrar", p))
		}
		sub := strings.Join(partes, " · ")
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return a.filaSala(gtx, a.clic("sala:"+b.ID()), a.clic("cod:"+b.ID()), b.Nombre(), sub)
		})
	}
	return columna(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(8), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fila(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.cuadrado(gtx, a.clic("sub:salas"), "salas", true) }), sep(10),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.cuadradoImagen(gtx, &a.gotching, imgGotching) }), sep(10),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.cuadradoImagen(gtx, &a.wenas, imgWenas) }))
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.listado(gtx, &a.principal, items) }),
	)
}

// filaSala is _RoomRow: the room's icon, its name and line of facts, and the copy-code button.
func (a *App) filaSala(gtx layout.Context, abrir, copiar *widget.Clickable, nombre, sub string) layout.Dimensions {
	return abrir.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		d := layout.Inset{Top: unit.Dp(13), Bottom: unit.Dp(13), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return fila(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icono2(gtx, "salas", 26, piel.Ink) }), sep(12),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return columna(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.th, unit.Sp(16), nombre)
							l.Color, l.Font.Weight = piel.Ink, font.Bold
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.th, unit.Sp(12.5), sub)
							l.Color = piel.Muted
							return l.Layout(gtx)
						}))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.iconoBoton(gtx, copiar, "postular", 38, 20, piel.Ink)
				}))
		})
		call := m.Stop()
		linea := piel.Line
		paint.FillShape(gtx.Ops, linea, clip.Rect{Min: image.Pt(0, d.Size.Y-1), Max: d.Size}.Op())
		call.Add(gtx.Ops)
		return d
	})
}

// EntrarSala opens a room on screen (main uses it for ZONDAGO_ENTRAR).
func (a *App) EntrarSala(room string) { a.entrarSala(room) }

func (a *App) entrarSala(room string) {
	a.n.EntrarASala(room)
	a.vista, a.salaTab, a.menuDe = pSala, "chat", ""
}

/* ---------- dialogs: create a room, enter with a code, a new voice channel ---------- */

func (a *App) abrirDialogo(cual string) {
	a.dialogo, a.aviso = cual, ""
	a.editorDialogo.SetText("")
	a.editorDialogo.SingleLine, a.editorDialogo.Submit = true, true
}

func (a *App) dialogoUI(gtx layout.Context) layout.Dimensions {
	if a.dialogo == "" {
		return layout.Dimensions{}
	}
	titulo, hint, ok := "Crear sala", "Nombre de la sala", "Crear"
	switch a.dialogo {
	case "codigo":
		titulo, hint, ok = "Entrar a una sala", "Pegá el código de la sala (empieza con ZR1:)", "Pedir entrar"
	case "canal":
		titulo, hint, ok = "Nuevo canal de voz", "Nombre del canal", "Crear"
	}
	if a.cancelarDialogo.Clicked(gtx) {
		a.dialogo = ""
		return layout.Dimensions{}
	}
	if a.aceptarDialogo.Clicked(gtx) || enviado(gtx, &a.editorDialogo) {
		a.confirmarDialogo(strings.TrimSpace(a.editorDialogo.Text()))
	}
	// A dimmed window under the dialog; a click on it does nothing (the dialog decides).
	velo := color.NRGBA{A: 0x99}
	paint.FillShape(gtx.Ops, velo, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, dp(gtx, 440))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return caja(gtx, piel.Surface, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return columna(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.titulo(gtx, titulo) }), sep(14),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.campo(gtx, &a.editorDialogo, hint) }), sep(16),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fila(gtx, layout.Flexed(1, layout.Spacer{}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.cancelarDialogo, "Cancelar") }), sep(8),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.boton(gtx, &a.aceptarDialogo, ok) }))
					}))
			})
		})
	})
}

func (a *App) confirmarDialogo(v string) {
	cual := a.dialogo
	a.dialogo = ""
	if v == "" {
		return
	}
	s := a.n.Salas()
	switch cual {
	case "crear":
		if l := len([]rune(v)); l < 2 || l > 32 || strings.ContainsAny(v, "#|") {
			a.aviso = "Usá de 2 a 32 letras, sin # ni |."
			return
		}
		c := s.Crear(v)
		a.entrarSala(c.ID)
	case "codigo":
		c, ok := nucleo.CartaDeCodigo(v)
		if !ok {
			a.aviso = "Ese no es un código de sala de Zonda."
			return
		}
		if s.EsMiembro(c.ID, a.n.Clave()) {
			a.aviso = "Ya estás en " + c.Name + "."
			return
		}
		s.Golpear(c)
		a.aviso = "Pedido enviado a " + c.Name + ". Cualquier miembro que esté adentro te puede aceptar."
	case "canal":
		if b := s.Libro(a.n.SalaActual); b != nil {
			s.FijarCanales(b.ID(), append(append([]string(nil), b.Canales()...), v))
		}
	}
}

// filaPersona: a person's row (the card color of their gender, their icon, name and a line under it).
func (a *App) filaPersona(gtx layout.Context, c *widget.Clickable, k, sub string) layout.Dimensions {
	n := a.n
	bg, ink := piel.Dark, piel.DarkInk
	if n.GeneroDe(k) == "f" {
		bg, ink = piel.Light, piel.LightInk
	}
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		d := layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return fila(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.avatar(gtx, k, n.NombreDe(k), 36) }), sep(10),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return punto(gtx, n.EnLinea(k)) }), sep(8),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return columna(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(a.th, unit.Sp(15), n.NombreDe(k))
							l.Color, l.Font.Weight, l.MaxLines = ink, font.Bold, 1
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if sub == "" {
								return layout.Dimensions{}
							}
							l := material.Label(a.th, unit.Sp(12.5), sub)
							l.Color, l.MaxLines = mezcla(ink, bg, .35), 1
							return l.Layout(gtx)
						}))
				}))
		})
		call := m.Stop()
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: d.Size}.Op())
		paint.FillShape(gtx.Ops, piel.Line, clip.Rect{Min: image.Pt(0, d.Size.Y-1), Max: d.Size}.Op())
		call.Add(gtx.Ops)
		return d
	})
}

/* ---------- Configuración: the network and the style ---------- */

func (a *App) ajustes(gtx layout.Context) layout.Dimensions {
	if a.clic("aj:red").Clicked(gtx) {
		a.subAjustes = "red"
	}
	if a.clic("aj:estilo").Clicked(gtx) {
		a.subAjustes = "estilo"
	}
	return columna(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(8), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fila(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.cuadrado(gtx, a.clic("aj:estilo"), "palette", a.subAjustes != "red")
					}), sep(10),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.cuadrado(gtx, a.clic("aj:red"), "wifi", a.subAjustes == "red")
					}))
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if a.subAjustes == "red" {
					return a.red(gtx)
				}
				return a.estilo(gtx)
			})
		}))
}
