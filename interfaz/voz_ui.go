package interfaz

import (
	"image"
	"sort"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

func clipRect(max image.Point) clip.Op { return clip.Rect{Max: max}.Op() }

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
