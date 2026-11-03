package interfaz

import (
	"image"
	"image/color"
	"sort"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func clipRect(max image.Point) clip.Op { return clip.Rect{Max: max}.Op() }

var negro = color.NRGBA{A: 0xff}

// clavesPantallas: who is sharing now, in a stable order.
func (a *App) clavesPantallas() ([]string, map[string]image.Image) {
	if a.voz == nil {
		return nil, nil
	}
	ps := a.voz.Pantallas()
	var ks []string
	for k := range ps {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks, ps
}

// pantallas is the room's _screens: the shared screens side by side on black, filling the space they are given, each
// with its owner's name and the full-screen button. A tap on one, or its button, opens it full screen.
func (a *App) pantallas(gtx layout.Context) layout.Dimensions {
	ks, ps := a.clavesPantallas()
	paint.FillShape(gtx.Ops, negro, clip.Rect{Max: gtx.Constraints.Max}.Op())
	var hijos []layout.FlexChild
	for _, k := range ks {
		k, img := k, ps[k]
		hijos = append(hijos, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.clic("pant:"+k).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						return widget.Image{Src: a.texturaPantalla(k, img), Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions { return a.rotuloPantalla(gtx, k, "pant:"+k) }))
			})
		}))
	}
	return layout.Flex{}.Layout(gtx, hijos...)
}

// rotuloPantalla: "Pantalla de X" on the left and the full-screen (or back) button on the right.
func (a *App) rotuloPantalla(gtx layout.Context, k, boton string) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(4), Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		icono := "screen"
		if a.pantallaGrande == k {
			icono = "x"
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.th, unit.Sp(14), "Pantalla de "+a.n.NombreDe(k))
				l.Color, l.Font.Weight = color.NRGBA{0xff, 0xff, 0xff, 0xff}, font.Bold
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.iconoBoton(gtx, a.clic(boton+":max"), icono, 40, 24, color.NRGBA{0xff, 0xff, 0xff, 0xff})
			}))
	})
}

// texturaPantalla: the image op of a screen, made again only when a new picture arrived (the same picture stays the
// same texture across redraws).
func (a *App) texturaPantalla(k string, img image.Image) paint.ImageOp {
	if t, ok := a.texturas[k]; ok && t.img == img {
		return t.op
	}
	op := paint.NewImageOp(img)
	a.texturas[k] = textura{img, op}
	return op
}

type textura struct {
	img image.Image
	op  paint.ImageOp
}

// clicsPantallas: a tap on a screen or its button opens it full screen; on the full screen, it goes back.
func (a *App) clicsPantallas(gtx layout.Context) {
	ks, _ := a.clavesPantallas()
	for _, k := range ks {
		if a.clic("pant:"+k).Clicked(gtx) || a.clic("pant:"+k+":max").Clicked(gtx) {
			a.maximizar(k)
		}
		if a.clic("grande:"+k).Clicked(gtx) || a.clic("grande:"+k+":max").Clicked(gtx) {
			a.maximizar("")
		}
	}
}

// maximizar puts one screen over the whole window, and the window full screen (k == "" goes back).
func (a *App) maximizar(k string) {
	a.pantallaGrande = k
	if k != "" {
		a.W.Option(app.Fullscreen.Option())
	} else {
		a.W.Option(app.Windowed.Option())
	}
}

// pantallaCompleta draws the chosen screen over the whole window; when it stops being shared, the room comes back.
func (a *App) pantallaCompleta(gtx layout.Context) bool {
	if a.pantallaGrande == "" {
		return false
	}
	_, ps := a.clavesPantallas()
	img, ok := ps[a.pantallaGrande]
	if !ok {
		a.maximizar("")
		return false
	}
	k := a.pantallaGrande
	paint.FillShape(gtx.Ops, negro, clip.Rect{Max: gtx.Constraints.Max}.Op())
	a.clic("grande:"+k).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return widget.Image{Src: a.texturaPantalla(k, img), Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
	})
	st := op.Offset(image.Pt(0, 0)).Push(gtx.Ops)
	a.rotuloPantalla(gtx, k, "grande:"+k)
	st.Pop()
	return true
}
