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
	"gioui.org/font"
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

	"github.com/nicoconvoz/zonda-go/bandeja"
	"github.com/nicoconvoz/zonda-go/diag"
	"github.com/nicoconvoz/zonda-go/nucleo"
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
	Medidas() string
	Niveles() (sala, mio float32)
}

type App struct {
	W     *app.Window
	hwnd  uintptr // the window on Windows, to hide it in the tray and bring it back
	th    *material.Theme
	n     *nucleo.Nodo
	expl  *explorer.Explorer
	voz   Voz
	ops   op.Ops
	vista pantalla
	tab   int
	chat  string // the contact on screen

	// widgets
	nick, salaNombre, codigo, mensaje, canales           widget.Editor
	hombre, mujer, elegirFoto, entrar                    widget.Clickable
	tabs                                                 [3]widget.Clickable
	gotching, wenas                                      widget.Clickable
	crear, golpear, cancelarGolpe                        widget.Clickable
	volver, copiar, cerrarSala, enviar, adjFoto, archivo widget.Clickable
	hablar, salirVoz, compartir, fijarCanales            widget.Clickable
	lista, miembros, principal                           widget.List
	clicks                                               map[string]*widget.Clickable

	texturas       map[string]textura
	genero         string
	fotoAlta       []byte
	imagenes       map[string]paint.ImageOp
	aviso          string
	avisoVoz       string
	pantallaGrande string
	perfil         perfilEdicion

	// the room screen and the dialogs
	salaTab, menuDe, canalElegido, subAjustes string
	dialogo                                   string
	editorDialogo                             widget.Editor
	aceptarDialogo, cancelarDialogo           widget.Clickable
}

func Nueva(n *nucleo.Nodo, voz Voz) *App {
	a := &App{n: n, voz: voz, clicks: map[string]*widget.Clickable{}, imagenes: map[string]paint.ImageOp{}, texturas: map[string]textura{}, genero: "m"}
	a.W = new(app.Window)
	a.W.Option(app.Title("Zonda"), app.Size(unit.Dp(1100), unit.Dp(760)), app.MinSize(unit.Dp(420), unit.Dp(500)), app.Maximized.Option())
	a.expl = explorer.NewExplorer(a.W)
	a.th = material.NewTheme()
	a.th.Shaper = text.NewShaper(text.WithCollection(append(coleccionFuentes(), gofont.Collection()...)))
	a.aplicarPiel(n.Alm.Leer("piel"))
	for _, e := range []*widget.Editor{&a.nick, &a.salaNombre, &a.codigo, &a.canales} {
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

// The colors of the skin in use (aplicarPiel): every piece of the screen takes them from here.
var (
	piel   = pieles[0]
	fondo  color.NRGBA
	panel  color.NRGBA
	tinta  color.NRGBA
	gris   color.NRGBA
	acento color.NRGBA
	verde  = hx(0xFF2FBF4A)
)

func (a *App) aplicarPiel(id string) {
	piel = pielDe(id)
	fondo, panel, tinta, gris, acento = piel.AppBg, piel.Surface2, piel.Ink, piel.Muted, piel.On
	a.th.Palette = material.Palette{Bg: piel.Surface, Fg: piel.Ink, ContrastBg: piel.On, ContrastFg: piel.OnInk}
	a.th.Face = piel.Fuente
}

func (a *App) clic(id string) *widget.Clickable {
	c := a.clicks[id]
	if c == nil {
		c = new(widget.Clickable)
		a.clicks[id] = c
	}
	return c
}

// Mostrar brings the window back from the tray.
func (a *App) Mostrar() { bandeja.Mostrar(a.hwnd) }

// Correr is the window's event loop.
func (a *App) Correr() error {
	diag.Nombrar("interfaz")
	for {
		e := a.W.Event()
		a.expl.ListenEvents(e)
		switch e := e.(type) {
		case app.Win32ViewEvent:
			if e.HWND != 0 {
				a.hwnd = e.HWND
				bandeja.PonerIcono(e.HWND)
			}
		case *app.ClosingEvent:
			// As the app does: closing only hides the window; Zonda stays next to the clock, linked.
			if a.hwnd != 0 {
				e.Abort()
				bandeja.Esconder(a.hwnd)
			}
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
	// The clicks are read before drawing: a Gio button drawn first consumes its own clicks.
	if a.vista == pSala && a.pantallaGrande != "" {
		a.clicsPantallas(gtx)
	}
	if a.vista == pSala && a.pantallaCompleta(gtx) {
		return
	}
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
	a.dialogoUI(gtx)
}

/* ---------- small pieces ---------- */

func (a *App) titulo(gtx layout.Context, s string) layout.Dimensions {
	l := material.Label(a.th, unit.Sp(17), s)
	l.Color, l.Font.Weight = tinta, font.ExtraBold
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

// boton: the skin's filled button (its "on" color), bold label, small corners.
func (a *App) boton(gtx layout.Context, c *widget.Clickable, s string) layout.Dimensions {
	return a.botonDe(gtx, c, "", s, piel.On, piel.OnInk, nil)
}

// botonSuave: the skin's idle button, outlined.
func (a *App) botonSuave(gtx layout.Context, c *widget.Clickable, s string) layout.Dimensions {
	l := piel.BtnLine
	return a.botonDe(gtx, c, "", s, piel.Btn, piel.BtnInk, &l)
}

func (a *App) botonIcono(gtx layout.Context, c *widget.Clickable, ic, s string, lleno bool) layout.Dimensions {
	if lleno {
		return a.botonDe(gtx, c, ic, s, piel.On, piel.OnInk, nil)
	}
	l := piel.BtnLine
	return a.botonDe(gtx, c, ic, s, piel.Btn, piel.BtnInk, &l)
}

func (a *App) botonDe(gtx layout.Context, c *widget.Clickable, ic, s string, bg, ink color.NRGBA, linea *color.NRGBA) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		d := layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			var hijos []layout.FlexChild
			if ic != "" {
				hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icono(gtx, ic, 18, ink) }))
				if s != "" {
					hijos = append(hijos, sep(6))
				}
			}
			if s != "" {
				hijos = append(hijos, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(a.th, unit.Sp(14.5), s)
					l.Color, l.Font.Weight, l.MaxLines = ink, font.Bold, 1
					return l.Layout(gtx)
				}))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, hijos...)
		})
		call := m.Stop()
		fill := bg
		if c.Hovered() {
			fill = mezcla(bg, ink, 0.08)
		}
		rr := clip.UniformRRect(image.Rectangle{Max: d.Size}, gtx.Dp(unit.Dp(piel.RadioChico)))
		paint.FillShape(gtx.Ops, fill, rr.Op(gtx.Ops))
		if linea != nil {
			paint.FillShape(gtx.Ops, *linea, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
		}
		call.Add(gtx.Ops)
		return d
	})
}

func mezcla(a, b color.NRGBA, t float32) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(float32(x)*(1-t) + float32(y)*t) }
	return color.NRGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), a.A}
}

func (a *App) campo(gtx layout.Context, e *widget.Editor, hint string) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		ed := material.Editor(a.th, e, hint)
		ed.Color, ed.HintColor, ed.SelectionColor = piel.Ink, piel.Muted, mezcla(piel.Surface, piel.On, .35)
		return ed.Layout(gtx)
	})
	call := m.Stop()
	rr := clip.UniformRRect(image.Rectangle{Max: d.Size}, gtx.Dp(unit.Dp(piel.RadioChico)))
	paint.FillShape(gtx.Ops, piel.Surface, rr.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, piel.Line, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
	call.Add(gtx.Ops)
	return d
}

// caja: a card of the skin: its color, corners, border and drop shadow, under its content.
func caja(gtx layout.Context, c color.NRGBA, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	r := gtx.Dp(unit.Dp(piel.Radio))
	tarjeta := c == panel || c == piel.Surface
	if tarjeta {
		c = piel.Surface
		sh := op.Offset(image.Pt(0, gtx.Dp(unit.Dp(piel.SombraDY)))).Push(gtx.Ops)
		paint.FillShape(gtx.Ops, piel.Sombra, clip.UniformRRect(image.Rectangle{Max: d.Size}, r).Op(gtx.Ops))
		sh.Pop()
	}
	rr := clip.UniformRRect(image.Rectangle{Max: d.Size}, r)
	paint.FillShape(gtx.Ops, c, rr.Op(gtx.Ops))
	if piel.Borde != nil && tarjeta {
		paint.FillShape(gtx.Ops, *piel.Borde, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
	}
	call.Add(gtx.Ops)
	return d
}

func sep(dp float32) layout.FlexChild {
	return layout.Rigid(layout.Spacer{Width: unit.Dp(dp), Height: unit.Dp(dp)}.Layout)
}

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

// imagen: a decoded photo by fingerprint, kept once decoded. The screen asks again on every frame, so the bytes come
// from cargar, called only when the photo is not decoded yet: reading the file each frame cost 112 MB of reads in a
// few minutes of a call.
func (a *App) imagen(h string, cargar func() []byte) (paint.ImageOp, bool) {
	if op, ok := a.imagenes[h]; ok {
		return op, true
	}
	b := cargar()
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

func (a *App) foto(gtx layout.Context, h string, cargar func() []byte, alto float32) layout.Dimensions {
	op, ok := a.imagen(h, cargar)
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
		if op, ok := a.imagen(h, func() []byte { return a.n.Foto(h) }); ok {
			return widget.Image{Src: op, Fit: widget.Cover}.Layout(gtx)
		}
		a.n.PedirFoto(h, key)
	}
	paint.Fill(gtx.Ops, piel.Dark)
	ini := "?"
	if nombre != "" {
		ini = strings.ToUpper(string([]rune(nombre)[:1]))
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Body1(a.th, ini)
		l.Color, l.Font.Weight = piel.DarkInk, font.Bold
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
