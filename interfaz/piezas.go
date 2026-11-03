package interfaz

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// The app's own pieces (zonda_ui: IbSquare, IbButton, IbRule, IbEmpty, IbFab, IbGlowRing…), with their sizes and
// colors. The app animates the glow ring and the active icon; here they are still (nothing is redrawn on a timer).

//go:embed marcas/gotching.png
var pngGotching []byte

//go:embed marcas/wenas.png
var pngWenas []byte

func decodificarPNG(b []byte) paint.ImageOp {
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return paint.ImageOp{}
	}
	return paint.NewImageOp(img)
}

var (
	imgGotching = decodificarPNG(pngGotching)
	imgWenas    = decodificarPNG(pngWenas)
)

// glow: IbBands.glow, the colors of the "more" ring.
var glow = []color.NRGBA{hx(0xFFFF3B6B), hx(0xFFFFB13B), hx(0xFFFFF13B), hx(0xFF3BFF7A), hx(0xFF3BC8FF), hx(0xFF8A5BFF), hx(0xFFFF3B6B)}

func dp(gtx layout.Context, v float32) int { return gtx.Dp(unit.Dp(v)) }

// sombraDe paints the skin's shadow under a rounded box: a drop for most skins, a glow for the neon one.
func sombraDe(gtx layout.Context, tam image.Point, r int, circulo bool) {
	forma := func(inset int) clip.Op {
		rect := image.Rectangle{Min: image.Pt(-inset, -inset), Max: tam.Add(image.Pt(inset, inset))}
		if circulo {
			return clip.Ellipse(rect).Op(gtx.Ops)
		}
		return clip.UniformRRect(rect, r+inset).Op(gtx.Ops)
	}
	if piel.Brillo {
		// Neon's shadow (ib_skins.dart): a pink halo, 18 px of blur fading out from the box. Without blur, rings of
		// falling alpha drawn from the outside in.
		for i, a := range []uint8{0x0C, 0x14, 0x1E, 0x2A, 0x3A} {
			c := piel.Sombra
			c.A = a
			paint.FillShape(gtx.Ops, c, forma(dp(gtx, float32(10-2*i))))
		}
		return
	}
	st := op.Offset(image.Pt(0, dp(gtx, piel.SombraDY))).Push(gtx.Ops)
	paint.FillShape(gtx.Ops, piel.Sombra, forma(0))
	st.Pop()
}

// cuadrado is IbSquare: 50 px, the "on" color when selected, the icon at 52 % of its side.
func (a *App) cuadrado(gtx layout.Context, c *widget.Clickable, icono string, on bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		s := dp(gtx, 50)
		tam := image.Pt(s, s)
		r := dp(gtx, piel.RadioChico)
		bg, line, fg := piel.Btn, piel.BtnLine, piel.BtnInk
		if on {
			bg, line, fg = piel.On, piel.On, piel.OnInk
		}
		sombraDe(gtx, tam, r, false)
		rr := clip.UniformRRect(image.Rectangle{Max: tam}, r)
		paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
		paint.FillShape(gtx.Ops, line, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(dp(gtx, 1.5))}.Op())
		lado := int(float32(s) * .52)
		st := op.Offset(image.Pt((s-lado)/2, (s-lado)/2)).Push(gtx.Ops)
		dibujarIcono(gtx, icono, lado, fg)
		st.Pop()
		return layout.Dimensions{Size: tam}
	})
}

// cuadradoImagen is the Gotching and Wenas squares: the picture clipped to the skin's corners, with its line.
func (a *App) cuadradoImagen(gtx layout.Context, c *widget.Clickable, img paint.ImageOp) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		s := dp(gtx, 50)
		tam := image.Pt(s, s)
		r := dp(gtx, piel.Radio)
		sombraDe(gtx, tam, r, false)
		rr := clip.UniformRRect(image.Rectangle{Max: tam}, r)
		st := rr.Push(gtx.Ops)
		g := gtx
		g.Constraints = layout.Exact(tam)
		widget.Image{Src: img, Fit: widget.Cover}.Layout(g)
		st.Pop()
		paint.FillShape(gtx.Ops, piel.Line, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(dp(gtx, 1.5))}.Op())
		return layout.Dimensions{Size: tam}
	})
}

// botonAncho is IbButton(compact, expand): icon and bold label centered, filled or ghost.
func (a *App) botonAncho(gtx layout.Context, c *widget.Clickable, icono, s string, lleno bool) layout.Dimensions {
	bg, fg, line := piel.Btn, piel.BtnInk, piel.BtnLine
	if lleno {
		bg, fg, line = piel.On, piel.OnInk, piel.On
	}
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		m := op.Record(gtx.Ops)
		d := layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icono2(gtx, icono, 18, fg) }),
					sep(8),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(a.th, unit.Sp(13), s)
						l.Color, l.Font.Weight, l.MaxLines = fg, font.Bold, 1
						return l.Layout(gtx)
					}))
			})
		})
		call := m.Stop()
		r := dp(gtx, piel.RadioChico)
		sombraDe(gtx, d.Size, r, false)
		rr := clip.UniformRRect(image.Rectangle{Max: d.Size}, r)
		fill := bg
		if c.Hovered() {
			fill = mezcla(bg, fg, .08)
		}
		paint.FillShape(gtx.Ops, fill, rr.Op(gtx.Ops))
		paint.FillShape(gtx.Ops, line, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(dp(gtx, 1.5))}.Op())
		call.Add(gtx.Ops)
		return d
	})
}

// regla is IbRule: the title in muted, a hairline to the end, an optional count.
func (a *App) regla(gtx layout.Context, titulo, cola string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(2), Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hijos := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.th, unit.Sp(15), titulo)
				l.Color, l.Font.Weight, l.MaxLines = piel.Muted, font.Medium, 1
				return l.Layout(gtx)
			}),
			sep(10),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				c := piel.Muted
				c.A = 0x73
				h := max(1, dp(gtx, 1))
				paint.FillShape(gtx.Ops, c, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, h)}.Op())
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
			}),
		}
		if cola != "" {
			hijos = append(hijos, sep(10), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.th, unit.Sp(12), cola)
				l.Color = piel.Muted
				return l.Layout(gtx)
			}))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, hijos...)
	})
}

// vacio is IbEmpty: the icon at half opacity and one honest sentence, centered.
func (a *App) vacio(gtx layout.Context, icono, s string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(36), Bottom: unit.Dp(36), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				c := piel.Muted
				c.A = 0x80
				return icono2(gtx, icono, 44, c)
			}),
			sep(8),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Label(a.th, unit.Sp(14.5), s)
				l.Color, l.Alignment = piel.Muted, text.Middle
				return l.Layout(gtx)
			}))
	})
}

// fab is IbFab: a 50 px circle in the "on" color with its icon.
func (a *App) fab(gtx layout.Context, c *widget.Clickable, icono string) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		s := dp(gtx, 50)
		tam := image.Pt(s, s)
		sombraDe(gtx, tam, s/2, true)
		paint.FillShape(gtx.Ops, piel.On, clip.Ellipse{Max: tam}.Op(gtx.Ops))
		lado := dp(gtx, 23)
		st := op.Offset(image.Pt((s-lado)/2, (s-lado)/2)).Push(gtx.Ops)
		dibujarIcono(gtx, icono, lado, piel.OnInk)
		st.Pop()
		return layout.Dimensions{Size: tam}
	})
}

// iconoBoton is IbIconButton: just the icon in a square area.
func (a *App) iconoBoton(gtx layout.Context, c *widget.Clickable, icono string, lado, tamIcono float32, col color.NRGBA) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		s, ic := dp(gtx, lado), dp(gtx, tamIcono)
		st := op.Offset(image.Pt((s-ic)/2, (s-ic)/2)).Push(gtx.Ops)
		dibujarIcono(gtx, icono, ic, col)
		st.Pop()
		return layout.Dimensions{Size: image.Pt(s, s)}
	})
}

func icono2(gtx layout.Context, nombre string, lado float32, c color.NRGBA) layout.Dimensions {
	s := dp(gtx, lado)
	dibujarIcono(gtx, nombre, s, c)
	return layout.Dimensions{Size: image.Pt(s, s)}
}

// dibujarIcono draws an icon of s pixels at the current offset.
func dibujarIcono(gtx layout.Context, nombre string, s int, c color.NRGBA) {
	g := gtx
	g.Metric.PxPerDp = 1
	icono(g, nombre, float32(s), c)
}
