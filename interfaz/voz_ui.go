package interfaz

import (
	"fmt"
	"image"
	"sort"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/nicoconvoz/zonda/zondago/nucleo"
)

func clipRect(max image.Point) clip.Op { return clip.Rect{Max: max}.Op() }

// vozClicks: the voice channel chips (the room's chat is one; each channel is its own voice), talking and screen.
func (a *App) vozClicks(gtx layout.Context, room string, b *nucleo.Libro) {
	if a.voz == nil {
		return
	}
	for _, c := range b.Canales() {
		id := room + "/" + c
		if a.clic("canal:" + id).Clicked(gtx) {
			if a.voz.Canal() == id {
				a.voz.Salir()
			} else if err := a.voz.Entrar(id, a.n.EnSala(room)); err != nil {
				a.avisoVoz = "No se pudo abrir la voz: " + err.Error()
			} else {
				a.avisoVoz = ""
			}
		}
	}
	if a.hablar.Clicked(gtx) {
		a.voz.AlternarHablar()
	}
	if a.compartir.Clicked(gtx) {
		if err := a.voz.AlternarPantalla(); err != nil {
			a.avisoVoz = err.Error()
		}
	}
	for k := range a.voz.Pantallas() {
		if a.clic("pant:" + k).Clicked(gtx) {
			if a.pantallaGrande == k {
				a.pantallaGrande = ""
			} else {
				a.pantallaGrande = k
			}
		}
	}
}

func (a *App) barraVoz(gtx layout.Context, b *nucleo.Libro) layout.Dimensions {
	if a.voz == nil {
		return layout.Dimensions{}
	}
	room := b.ID()
	canal := a.voz.Canal()
	return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return columna(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				hijos := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.chico(gtx, "Voz (tocá un canal para entrar): ") })}
				for _, c := range b.Canales() {
					c := c
					id := room + "/" + c
					hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if canal == id {
							return a.boton(gtx, a.clic("canal:"+id), "🔊 "+c+" · salir")
						}
						return a.botonSuave(gtx, a.clic("canal:"+id), "🔈 "+c)
					}), sep(6))
				}
				if canal != "" && len(canal) > len(room) && canal[:len(room)] == room {
					hablar := "🎙 Hablar"
					if a.voz.Hablando() {
						hablar = "🎙 Hablando (tocá para callar)"
					}
					pant := "Compartir pantalla"
					if a.voz.Compartiendo() {
						pant = "Dejar de compartir"
					}
					hijos = append(hijos,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.boton(gtx, &a.hablar, hablar) }), sep(6),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.compartir, pant) }), sep(6),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.chico(gtx, fmt.Sprintf("%d conectados", a.voz.Conectados()))
						}))
				}
				if b.EsCreador(a.n.Clave()) {
					hijos = append(hijos, layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Max.X = gtx.Dp(220)
							return a.campo(gtx, &a.canales, "Canales: General, Juego")
						}), sep(6),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.botonSuave(gtx, &a.fijarCanales, "Fijar") }))
				}
				return fila(gtx, hijos...)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.avisoVoz == "" {
					return layout.Dimensions{}
				}
				return a.texto(gtx, a.avisoVoz, acento)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.pantallas(gtx) }),
		)
	})
}

// pantallas: the screens shared in the voice channel; one tap makes one fill the room, another tap shrinks it.
func (a *App) pantallas(gtx layout.Context) layout.Dimensions {
	ps := a.voz.Pantallas()
	if len(ps) == 0 {
		a.pantallaGrande = ""
		return layout.Dimensions{}
	}
	var ks []string
	for k := range ps {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	alto := float32(180)
	if a.pantallaGrande != "" {
		if img, ok := ps[a.pantallaGrande]; ok {
			ks, alto = []string{a.pantallaGrande}, float32(gtx.Metric.PxToDp(gtx.Constraints.Max.Y))*0.6
			_ = img
		}
	}
	var hijos []layout.FlexChild
	for _, k := range ks {
		k, img := k, ps[k]
		hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.clic("pant:"+k).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return columna(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.chico(gtx, "Pantalla de "+a.n.NombreDe(k)) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(alto))
						return widget.Image{Src: paint.NewImageOp(img), Fit: widget.Contain}.Layout(gtx)
					}))
			})
		}), sep(8))
	}
	return fila(gtx, hijos...)
}
