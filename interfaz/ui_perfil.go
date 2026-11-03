package interfaz

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/nicoconvoz/zonda-go/nucleo"
)

/* ---------- Usuarios: the profile (features/perfil) ---------- */

var rojo = hx(0xFFE5383B)

// perfilEdicion is "Editar mis datos" while open: the same three things the sign-up asks.
type perfilEdicion struct {
	abierto   bool
	apodo     widget.Editor
	genero    string
	foto      []byte // a new photo chosen, not saved yet
	claveFoto string // its fingerprint, computed once when chosen
	err       string
}

// usuarios is the profile: my card, and the other profiles in reach; my card opens "Editar mis datos".
func (a *App) usuarios(gtx layout.Context) layout.Dimensions {
	if a.perfil.abierto {
		return a.editarPerfil(gtx)
	}
	n := a.n
	yo := n.Clave()
	if a.clic("perfil:yo").Clicked(gtx) || a.clic("perfil:editar").Clicked(gtx) {
		a.abrirEdicion()
	}
	gente := n.Gente()
	items := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return a.regla(gtx, "Mi perfil", "") },
		func(gtx layout.Context) layout.Dimensions {
			return a.filaPersona(gtx, a.clic("perfil:yo"), yo, "Tocá para editar tus datos")
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.botonAncho(gtx, a.clic("perfil:editar"), "edit", "Editar mis datos", false)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.regla(gtx, "Otros perfiles", fmt.Sprint(len(gente)))
		},
	}
	if len(gente) == 0 {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return a.vacio(gtx, "perfil", "Todavía no hay nadie cerca.")
		})
	}
	for _, m := range gente {
		k := m.Clave
		items = append(items, func(gtx layout.Context) layout.Dimensions { return a.filaPersona(gtx, a.clic("perfil:"+k), k, "") })
	}
	return layout.Inset{Top: unit.Dp(12), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.listado(gtx, &a.principal, items)
	})
}

func (a *App) abrirEdicion() {
	p := &a.perfil
	p.abierto, p.foto, p.err = true, nil, ""
	p.apodo.SingleLine, p.apodo.Submit, p.apodo.MaxLen = true, true, nucleo.ApodoMax
	p.apodo.SetText(a.n.Nombre)
	p.genero = a.n.Genero
	if p.genero != "f" {
		p.genero = "m"
	}
}

var problemasApodo = map[string]string{
	"short": fmt.Sprintf("Mínimo %d caracteres.", nucleo.ApodoMin),
	"long":  fmt.Sprintf("Máximo %d caracteres.", nucleo.ApodoMax),
	"bad":   "Sin saltos de línea ni el signo |.",
}

// editarPerfil is EditProfileScreen: the photo in a circle, the nickname, man or woman, and Guardar.
func (a *App) editarPerfil(gtx layout.Context) layout.Dimensions {
	p := &a.perfil
	if a.clic("perfil:foto").Clicked(gtx) || a.clic("perfil:fotoTxt").Clicked(gtx) {
		a.elegirArchivo([]string{".jpg", ".jpeg", ".png"}, func(_ string, b []byte) {
			h := sha256.Sum256(b)
			p.foto, p.claveFoto = b, "nueva:"+hex.EncodeToString(h[:8])
		})
	}
	for _, g := range []string{"m", "f"} {
		if a.clic("perfil:g:" + g).Clicked(gtx) {
			p.genero = g
		}
	}
	if a.clic("perfil:cancelar").Clicked(gtx) {
		p.abierto = false
		return layout.Dimensions{}
	}
	if a.clic("perfil:guardar").Clicked(gtx) || enviado(gtx, &p.apodo) {
		if por := nucleo.ProblemaApodo(p.apodo.Text()); por != "" {
			p.err = problemasApodo[por]
		} else {
			// The same account with new data: the card is signed again and the others get it on the next hello.
			a.n.CrearCuenta(p.apodo.Text(), p.genero, p.foto)
			p.abierto = false
			a.aviso = "Perfil guardado."
			return layout.Dimensions{}
		}
	}
	textoFoto := "Elegir foto de perfil"
	if p.foto != nil || a.n.MiFoto != "" {
		textoFoto = "Cambiar foto"
	}
	items := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return a.regla(gtx, "Editar mis datos", "") },
		func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.clic("perfil:foto").Layout(gtx, func(gtx layout.Context) layout.Dimensions { return a.fotoPerfil(gtx, 112) })
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return a.mini(gtx, a.clic("perfil:fotoTxt"), textoFoto) })
		},
		func(gtx layout.Context) layout.Dimensions { return a.regla(gtx, "Quién sos en Zonda", "") },
		func(gtx layout.Context) layout.Dimensions { return a.campo(gtx, &p.apodo, "Apodo") },
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fila(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.botonAncho(gtx, a.clic("perfil:g:m"), "perfil", "Hombre", p.genero == "m")
					}), sep(8),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.botonAncho(gtx, a.clic("perfil:g:f"), "perfilF", "Mujer", p.genero == "f")
					}))
			})
		},
	}
	if p.err != "" {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			l := material.Label(a.th, unit.Sp(14), p.err)
			l.Color, l.Font.Weight = rojo, font.SemiBold
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, l.Layout)
		})
	}
	items = append(items,
		func(gtx layout.Context) layout.Dimensions {
			return a.botonAncho(gtx, a.clic("perfil:guardar"), "check", "Guardar", true)
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.botonAncho(gtx, a.clic("perfil:cancelar"), "x", "Cancelar", false)
			})
		})
	return layout.Inset{Top: unit.Dp(12), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(520))
		return a.listado(gtx, &a.principal, items)
	})
}

// fotoPerfil: the photo being edited in a circle (the new one if chosen, else mine), or the image icon.
func (a *App) fotoPerfil(gtx layout.Context, lado float32) layout.Dimensions {
	p := &a.perfil
	s := gtx.Dp(unit.Dp(lado))
	gtx.Constraints = layout.Exact(image.Pt(s, s))
	defer clip.Ellipse{Max: image.Pt(s, s)}.Push(gtx.Ops).Pop()
	var op paint.ImageOp
	ok := false
	if p.foto != nil {
		op, ok = a.imagen(p.claveFoto, func() []byte { return p.foto })
	} else if h := a.n.MiFoto; h != "" {
		op, ok = a.imagen(h, func() []byte { return a.n.Foto(h) })
	}
	if ok {
		return widget.Image{Src: op, Fit: widget.Cover}.Layout(gtx)
	}
	paint.Fill(gtx.Ops, piel.Surface2)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return icono2(gtx, "image", 40, gris) })
}
