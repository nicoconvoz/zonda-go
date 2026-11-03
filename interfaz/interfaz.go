// Package interfaz draws Zonda with Gio: no browser, the window is drawn by the program itself and only when
// something changes (an event or a node change), never on a timer.
package interfaz

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"

	"github.com/nicoconvoz/zonda/zondago/nucleo"
)

type pantalla int

const (
	pAlta pantalla = iota
	pPrincipal
	pSala
	pChat
)

// Voz is what the screen needs from the voice side (implemented by the voz package).
type Voz interface {
	Entrar(canal string, miembros []string) error
	Salir()
	Canal() string
	Hablando() bool
	AlternarHablar()
	Conectados() int
	Compartiendo() bool
	AlternarPantalla() error
	Pantallas() map[string]image.Image
}

type App struct {
	W     *app.Window
	th    *material.Theme
	n     *nucleo.Nodo
	expl  *explorer.Explorer
	voz   Voz
	ops   op.Ops
	vista pantalla
	tab   int
	chat  string // the contact on screen

	// widgets
	nick, salaNombre, codigo, buscar, mensaje, canales widget.Editor
	hombre, mujer, elegirFoto, entrar                widget.Clickable
	tabs                                             [3]widget.Clickable
	gotching, wenas                                  widget.Clickable
	crear, golpear, cancelarGolpe, buscarBtn         widget.Clickable
	volver, copiar, cerrarSala, enviar, adjFoto, archivo widget.Clickable
	hablar, salirVoz, compartir, fijarCanales        widget.Clickable
	lista, miembros, principal                       widget.List
	clicks                                           map[string]*widget.Clickable

	genero    string
	fotoAlta  []byte
	imagenes  map[string]paint.ImageOp
	aviso     string
	avisoVoz  string
	pantallaGrande string
}

func Nueva(n *nucleo.Nodo, voz Voz) *App {
	a := &App{n: n, voz: voz, clicks: map[string]*widget.Clickable{}, imagenes: map[string]paint.ImageOp{}, genero: "m"}
	a.W = new(app.Window)
	a.W.Option(app.Title("Zonda"), app.Size(unit.Dp(1100), unit.Dp(760)), app.MinSize(unit.Dp(420), unit.Dp(500)))
	a.expl = explorer.NewExplorer(a.W)
	a.th = material.NewTheme()
	a.th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	a.th.Palette = material.Palette{Bg: fondo, Fg: tinta, ContrastBg: acento, ContrastFg: color.NRGBA{0xff, 0xff, 0xff, 0xff}}
	for _, e := range []*widget.Editor{&a.nick, &a.salaNombre, &a.codigo, &a.buscar, &a.canales} {
		e.SingleLine, e.Submit = true, true
	}
	a.mensaje.Submit = true
	a.lista.Axis, a.miembros.Axis, a.principal.Axis = layout.Vertical, layout.Vertical, layout.Vertical
	a.lista.ScrollToEnd = true
	n.OnCambio = a.W.Invalidate
	if n.TieneCuenta() {
		a.vista = pPrincipal
	}
	return a
}

// Muted colors of the Mendoza wind: dry earth, poplars, the mountain's blue.
var (
	fondo  = color.NRGBA{0xf4, 0xf0, 0xe8, 0xff}
	panel  = color.NRGBA{0xe7, 0xe0, 0xd2, 0xff}
	tinta  = color.NRGBA{0x2e, 0x33, 0x36, 0xff}
	gris   = color.NRGBA{0x7a, 0x7d, 0x7a, 0xff}
	acento = color.NRGBA{0x4d, 0x6b, 0x7a, 0xff}
	verde  = color.NRGBA{0x5f, 0x8a, 0x4e, 0xff}
)

func (a *App) clic(id string) *widget.Clickable {
	c := a.clicks[id]
	if c == nil {
		c = new(widget.Clickable)
		a.clicks[id] = c
	}
	return c
}

// Correr is the window's event loop.
func (a *App) Correr() error {
	for {
		e := a.W.Event()
		a.expl.ListenEvents(e)
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&a.ops, e)
			// Drawn on the node's loop: what the screen reads and what a click changes is the node's own state.
			a.n.Consultar(func() { a.dibujar(gtx) })
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) dibujar(gtx layout.Context) {
	paint.Fill(gtx.Ops, fondo)
	switch a.vista {
	case pAlta:
		a.alta(gtx)
	case pPrincipal:
		a.principalUI(gtx)
	case pSala:
		a.sala(gtx)
	case pChat:
		a.chatUI(gtx)
	}
}

/* ---------- small pieces ---------- */

func (a *App) titulo(gtx layout.Context, s string) layout.Dimensions {
	l := material.H6(a.th, s)
	l.Color = tinta
	return l.Layout(gtx)
}

func (a *App) texto(gtx layout.Context, s string, c color.NRGBA) layout.Dimensions {
	l := material.Body1(a.th, s)
	l.Color = c
	return l.Layout(gtx)
}

func (a *App) chico(gtx layout.Context, s string) layout.Dimensions {
	l := material.Caption(a.th, s)
	l.Color = gris
	return l.Layout(gtx)
}

func (a *App) boton(gtx layout.Context, c *widget.Clickable, s string) layout.Dimensions {
	return material.Button(a.th, c, s).Layout(gtx)
}

func (a *App) botonSuave(gtx layout.Context, c *widget.Clickable, s string) layout.Dimensions {
	b := material.Button(a.th, c, s)
	b.Background, b.Color = panel, tinta
	return b.Layout(gtx)
}

func (a *App) campo(gtx layout.Context, e *widget.Editor, hint string) layout.Dimensions {
	return caja(gtx, panel, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, material.Editor(a.th, e, hint).Layout)
	})
}

func caja(gtx layout.Context, c color.NRGBA, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	r := clip.UniformRRect(image.Rectangle{Max: d.Size}, gtx.Dp(6))
	paint.FillShape(gtx.Ops, c, r.Op(gtx.Ops))
	call.Add(gtx.Ops)
	return d
}

func sep(dp float32) layout.FlexChild { return layout.Rigid(layout.Spacer{Width: unit.Dp(dp), Height: unit.Dp(dp)}.Layout) }

func fila(gtx layout.Context, hijos ...layout.FlexChild) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, hijos...)
}

func columna(gtx layout.Context, hijos ...layout.FlexChild) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, hijos...)
}

func margen(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, w)
}

func abrirURL(u string) { exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start() }

func (a *App) copiarTexto(gtx layout.Context, s string) {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(s))})
	a.aviso = "Copiado."
}

// imagen: a decoded photo by fingerprint, kept once decoded (the screen asks again on every frame).
func (a *App) imagen(h string, b []byte) (paint.ImageOp, bool) {
	if op, ok := a.imagenes[h]; ok {
		return op, true
	}
	if b == nil {
		return paint.ImageOp{}, false
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return paint.ImageOp{}, false
	}
	op := paint.NewImageOp(img)
	a.imagenes[h] = op
	return op, true
}

func (a *App) foto(gtx layout.Context, h string, b []byte, alto float32) layout.Dimensions {
	op, ok := a.imagen(h, b)
	if !ok {
		return a.chico(gtx, "🖼 cargando foto…")
	}
	gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(alto))
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(alto*1.6)))
	return widget.Image{Src: op, Fit: widget.Contain, Position: layout.W}.Layout(gtx)
}

// avatar: the profile photo in a circle, or the initial.
func (a *App) avatar(gtx layout.Context, key, nombre string, dp float32) layout.Dimensions {
	s := gtx.Dp(unit.Dp(dp))
	gtx.Constraints = layout.Exact(image.Pt(s, s))
	defer clip.Ellipse{Max: image.Pt(s, s)}.Push(gtx.Ops).Pop()
	if h := a.n.FotoDe(key); h != "" {
		if op, ok := a.imagen(h, a.n.Foto(h)); ok {
			return widget.Image{Src: op, Fit: widget.Cover}.Layout(gtx)
		}
		a.n.PedirFoto(h, key)
	}
	paint.Fill(gtx.Ops, acento)
	ini := "?"
	if nombre != "" {
		ini = strings.ToUpper(string([]rune(nombre)[:1]))
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Body1(a.th, ini)
		l.Color = color.NRGBA{0xff, 0xff, 0xff, 0xff}
		return l.Layout(gtx)
	})
}

func punto(gtx layout.Context, on bool) layout.Dimensions {
	s := gtx.Dp(8)
	c := gris
	if on {
		c = verde
	}
	paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(s, s)}.Op(gtx.Ops))
	return layout.Dimensions{Size: image.Pt(s, s)}
}

// elegirArchivo opens the system's file chooser on its own goroutine and hands the bytes to the node's loop.
func (a *App) elegirArchivo(exts []string, listo func(nombre string, b []byte)) {
	go func() {
		r, err := a.expl.ChooseFile(exts...)
		if err != nil {
			return
		}
		defer r.Close()
		nombre := "archivo"
		if f, ok := r.(interface{ Name() string }); ok {
			nombre = filepath.Base(f.Name())
		}
		b, err := io.ReadAll(io.LimitReader(r, 64<<20))
		if err != nil {
			return
		}
		a.n.Hacer(func() { listo(nombre, b); a.W.Invalidate() })
	}()
}

// guardarArchivo asks where to keep a received file and writes it there.
func (a *App) guardarArchivo(nombre string, b []byte) {
	go func() {
		w, err := a.expl.CreateFile(nombre)
		if err != nil {
			return
		}
		w.Write(b)
		w.Close()
	}()
}

func hora(ms int64) string { return time.UnixMilli(ms).Format("15:04") }

var _ = fmt.Sprint
