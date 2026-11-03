package interfaz

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/op/paint"
	"golang.org/x/image/vector"
)

// The icons are drawn once per size and color as an image and reused: stroking the SVG lines with Gio on every frame
// rebuilt every stroke each time (about 640 MB of garbage in a few minutes of a call, most of the window's cost).

type claveIcono struct {
	nombre string
	lado   int
	c      color.NRGBA
}

var (
	iconosListos = map[claveIcono]paint.ImageOp{}
	rasterizados int // how many icons were drawn, for the test
)

// maxIconos: the combinations a screen uses are a few hundred (icons × sizes × skin colors); past this the cache
// starts over instead of growing when skins are changed many times.
const maxIconos = 1024

func iconoImagen(nombre string, lado int, c color.NRGBA) paint.ImageOp {
	k := claveIcono{nombre, lado, c}
	if op, ok := iconosListos[k]; ok {
		return op
	}
	if len(iconosListos) >= maxIconos {
		iconosListos = map[claveIcono]paint.ImageOp{}
	}
	op := paint.NewImageOp(rasterIcono(nombre, lado, c))
	iconosListos[k] = op
	return op
}

// rasterIcono draws the icon at lado pixels: each line as quads with a disc on every vertex (round joins and caps, as
// the drawings are made), each filled shape as itself.
func rasterIcono(nombre string, lado int, c color.NRGBA) *image.RGBA {
	rasterizados++
	img := image.NewRGBA(image.Rect(0, 0, lado, lado))
	esc := float32(lado) / 24
	src := image.NewUniform(c)
	z := vector.NewRasterizer(lado, lado)
	for _, t := range leerIcono(nombre) {
		z.Reset(lado, lado)
		for k, seg := range t.segs {
			pts := make([]f32.Point, len(seg))
			for i, p := range seg {
				pts[i] = p.Mul(esc)
			}
			if t.relleno {
				z.MoveTo(pts[0].X, pts[0].Y)
				for _, p := range pts[1:] {
					z.LineTo(p.X, p.Y)
				}
				z.ClosePath()
				continue
			}
			r := t.ancho * esc / 2
			if t.cerrado[k] && len(pts) > 1 && pts[0] != pts[len(pts)-1] {
				pts = append(pts, pts[0])
			}
			for i := 0; i+1 < len(pts); i++ {
				tramo(z, pts[i], pts[i+1], r)
			}
			for _, p := range pts {
				disco(z, p, r)
			}
		}
		z.Draw(img, img.Bounds(), src, image.Point{})
	}
	return img
}

// tramo and disco wind the same way: the rasterizer adds the shapes' signed areas, so shapes wound opposite ways
// would cancel where they overlap.
func tramo(z *vector.Rasterizer, p, q f32.Point, r float32) {
	d := q.Sub(p)
	l := float32(math.Hypot(float64(d.X), float64(d.Y)))
	if l == 0 {
		return
	}
	n := f32.Pt(-d.Y/l*r, d.X/l*r)
	a, b, c, e := p.Add(n), q.Add(n), q.Sub(n), p.Sub(n)
	z.MoveTo(a.X, a.Y)
	z.LineTo(b.X, b.Y)
	z.LineTo(c.X, c.Y)
	z.LineTo(e.X, e.Y)
	z.ClosePath()
}

func disco(z *vector.Rasterizer, p f32.Point, r float32) {
	// Enough sides that the edge stays within a tenth of a pixel of the circle: r(1-cos(π/n)) ≤ 0.1.
	n := 8
	for float64(r)*(1-math.Cos(math.Pi/float64(n))) > 0.1 && n < 64 {
		n *= 2
	}
	for i := 0; i <= n; i++ {
		a := -2 * math.Pi * float64(i) / float64(n)
		x, y := p.X+r*float32(math.Cos(a)), p.Y+r*float32(math.Sin(a))
		if i == 0 {
			z.MoveTo(x, y)
		} else {
			z.LineTo(x, y)
		}
	}
	z.ClosePath()
}
