package interfaz

import (
	"image"
	"image/color"
	"math"
	"regexp"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// A small SVG reader for the app's icons: line drawings on a 24×24 grid, stroked in the ink color (path, circle,
// ellipse, rect; path commands M L H V C S Q T A Z, absolute and relative).

type trazo struct {
	segs    [][]f32.Point // each subpath as points (curves flattened)
	cerrado []bool
	ancho   float32
	relleno bool
}

var (
	rxElem  = regexp.MustCompile(`<(path|circle|ellipse|rect)\b([^>]*)/?>`)
	rxAttr  = regexp.MustCompile(`([a-z-]+)="([^"]*)"`)
	rxToken = regexp.MustCompile(`[A-Za-z]|-?(?:\d+\.?\d*|\.\d+)(?:e-?\d+)?`)
)

var cacheIconos = map[string][]trazo{}

func leerIcono(nombre string) []trazo {
	if t, ok := cacheIconos[nombre]; ok {
		return t
	}
	var out []trazo
	for _, m := range rxElem.FindAllStringSubmatch(iconos[nombre], -1) {
		at := map[string]string{}
		for _, a := range rxAttr.FindAllStringSubmatch(m[2], -1) {
			at[a[1]] = a[2]
		}
		num := func(k string) float32 { f, _ := strconv.ParseFloat(at[k], 32); return float32(f) }
		t := trazo{ancho: 2}
		if w, ok := at["stroke-width"]; ok {
			f, _ := strconv.ParseFloat(w, 32)
			t.ancho = float32(f)
		}
		t.relleno = at["fill"] != "" && at["fill"] != "none"
		switch m[1] {
		case "path":
			t.segs, t.cerrado = camino(at["d"])
		case "circle":
			t.segs, t.cerrado = [][]f32.Point{elipse(num("cx"), num("cy"), num("r"), num("r"))}, []bool{true}
		case "ellipse":
			t.segs, t.cerrado = [][]f32.Point{elipse(num("cx"), num("cy"), num("rx"), num("ry"))}, []bool{true}
		case "rect":
			t.segs, t.cerrado = [][]f32.Point{rectangulo(num("x"), num("y"), num("width"), num("height"), num("rx"))}, []bool{true}
		}
		out = append(out, t)
	}
	cacheIconos[nombre] = out
	return out
}

func elipse(cx, cy, rx, ry float32) []f32.Point {
	const n = 32
	pts := make([]f32.Point, n)
	for i := range pts {
		a := 2 * math.Pi * float64(i) / n
		pts[i] = f32.Pt(cx+rx*float32(math.Cos(a)), cy+ry*float32(math.Sin(a)))
	}
	return pts
}

func rectangulo(x, y, w, h, r float32) []f32.Point {
	if r <= 0 {
		return []f32.Point{{X: x, Y: y}, {X: x + w, Y: y}, {X: x + w, Y: y + h}, {X: x, Y: y + h}}
	}
	r = min(r, w/2, h/2)
	var pts []f32.Point
	esquina := func(cx, cy float32, a0 float64) {
		for i := 0; i <= 6; i++ {
			a := a0 + math.Pi/2*float64(i)/6
			pts = append(pts, f32.Pt(cx+r*float32(math.Cos(a)), cy+r*float32(math.Sin(a))))
		}
	}
	esquina(x+w-r, y+r, -math.Pi/2)
	esquina(x+w-r, y+h-r, 0)
	esquina(x+r, y+h-r, math.Pi/2)
	esquina(x+r, y+r, math.Pi)
	return pts
}

func cubica(p0, p1, p2, p3 f32.Point) []f32.Point {
	var out []f32.Point
	for i := 1; i <= 12; i++ {
		t := float32(i) / 12
		u := 1 - t
		out = append(out, p0.Mul(u*u*u).Add(p1.Mul(3*u*u*t)).Add(p2.Mul(3*u*t*t)).Add(p3.Mul(t*t*t)))
	}
	return out
}

// arco converts an SVG elliptical arc to points (the endpoint parameterization of the SVG spec, F.6.5).
func arco(p0 f32.Point, rx, ry, rot float64, grande, barrido bool, p f32.Point) []f32.Point {
	if rx == 0 || ry == 0 {
		return []f32.Point{p}
	}
	phi := rot * math.Pi / 180
	cs, sn := math.Cos(phi), math.Sin(phi)
	dx, dy := float64(p0.X-p.X)/2, float64(p0.Y-p.Y)/2
	x1, y1 := cs*dx+sn*dy, -sn*dx+cs*dy
	rx, ry = math.Abs(rx), math.Abs(ry)
	if l := x1*x1/(rx*rx) + y1*y1/(ry*ry); l > 1 {
		rx, ry = rx*math.Sqrt(l), ry*math.Sqrt(l)
	}
	num := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	den := rx*rx*y1*y1 + ry*ry*x1*x1
	co := 0.0
	if den != 0 && num > 0 {
		co = math.Sqrt(num / den)
	}
	if grande == barrido {
		co = -co
	}
	cx1, cy1 := co*rx*y1/ry, -co*ry*x1/rx
	cx := cs*cx1 - sn*cy1 + float64(p0.X+p.X)/2
	cy := sn*cx1 + cs*cy1 + float64(p0.Y+p.Y)/2
	ang := func(ux, uy, vx, vy float64) float64 { return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy) }
	t1 := ang(1, 0, (x1-cx1)/rx, (y1-cy1)/ry)
	dt := ang((x1-cx1)/rx, (y1-cy1)/ry, (-x1-cx1)/rx, (-y1-cy1)/ry)
	if !barrido && dt > 0 {
		dt -= 2 * math.Pi
	} else if barrido && dt < 0 {
		dt += 2 * math.Pi
	}
	n := max(4, int(math.Abs(dt)/(math.Pi/16)))
	var out []f32.Point
	for i := 1; i <= n; i++ {
		t := t1 + dt*float64(i)/float64(n)
		x, y := rx*math.Cos(t), ry*math.Sin(t)
		out = append(out, f32.Pt(float32(cs*x-sn*y+cx), float32(sn*x+cs*y+cy)))
	}
	return out
}

func camino(d string) ([][]f32.Point, []bool) {
	toks := rxToken.FindAllString(d, -1)
	var segs [][]f32.Point
	var cerr []bool
	var cur []f32.Point
	var p, inicio, ctrl f32.Point
	cmd := byte('M')
	i := 0
	num := func() float32 {
		if i >= len(toks) {
			return 0
		}
		f, _ := strconv.ParseFloat(toks[i], 32)
		i++
		return float32(f)
	}
	bandera := func() bool {
		// Arc flags may be glued to the next number ("a1 1 0 011 1"): take one digit.
		if i >= len(toks) {
			return false
		}
		t := toks[i]
		if len(t) > 1 && (t[0] == '0' || t[0] == '1') {
			toks[i] = t[1:]
			return t[0] == '1'
		}
		i++
		return t == "1"
	}
	cerrarSeg := func(c bool) {
		if len(cur) > 1 {
			segs, cerr = append(segs, cur), append(cerr, c)
		}
		cur = nil
	}
	for i < len(toks) {
		if t := toks[i]; len(t) == 1 && strings.ContainsAny(t, "MmLlHhVvCcSsQqTtAaZz") {
			cmd = t[0]
			i++
			if cmd == 'Z' || cmd == 'z' {
				cerrarSeg(true)
				p = inicio
				continue
			}
		}
		rel := cmd >= 'a'
		base := f32.Point{}
		if rel {
			base = p
		}
		switch cmd | 0x20 {
		case 'm':
			cerrarSeg(false)
			p = base.Add(f32.Pt(num(), num()))
			inicio = p
			cur = []f32.Point{p}
			if cmd == 'm' {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'l':
			p = base.Add(f32.Pt(num(), num()))
			cur = append(cur, p)
		case 'h':
			x := num()
			if rel {
				p.X += x
			} else {
				p.X = x
			}
			cur = append(cur, p)
		case 'v':
			y := num()
			if rel {
				p.Y += y
			} else {
				p.Y = y
			}
			cur = append(cur, p)
		case 'c':
			c1, c2, e := base.Add(f32.Pt(num(), num())), base.Add(f32.Pt(num(), num())), base.Add(f32.Pt(num(), num()))
			cur = append(cur, cubica(p, c1, c2, e)...)
			p, ctrl = e, c2
		case 's':
			c1 := p.Add(p.Sub(ctrl))
			c2, e := base.Add(f32.Pt(num(), num())), base.Add(f32.Pt(num(), num()))
			cur = append(cur, cubica(p, c1, c2, e)...)
			p, ctrl = e, c2
		case 'q':
			q, e := base.Add(f32.Pt(num(), num())), base.Add(f32.Pt(num(), num()))
			cur = append(cur, cubica(p, p.Add(q.Sub(p).Mul(2.0/3)), e.Add(q.Sub(e).Mul(2.0/3)), e)...)
			p, ctrl = e, q
		case 't':
			q, e := p.Add(p.Sub(ctrl)), base.Add(f32.Pt(num(), num()))
			cur = append(cur, cubica(p, p.Add(q.Sub(p).Mul(2.0/3)), e.Add(q.Sub(e).Mul(2.0/3)), e)...)
			p, ctrl = e, q
		case 'a':
			rx, ry, rot := num(), num(), num()
			g, b := bandera(), bandera()
			e := base.Add(f32.Pt(num(), num()))
			cur = append(cur, arco(p, float64(rx), float64(ry), float64(rot), g, b, e)...)
			p = e
		default:
			i++
		}
		if c := cmd | 0x20; c != 'c' && c != 's' && c != 'q' && c != 't' {
			ctrl = p
		}
	}
	cerrarSeg(false)
	return segs, cerr
}

// icono draws a named icon at dp size in color c.
func icono(gtx layout.Context, nombre string, dp float32, c color.NRGBA) layout.Dimensions {
	s := gtx.Dp(unit.Dp(dp))
	esc := float32(s) / 24
	for _, t := range leerIcono(nombre) {
		for k, seg := range t.segs {
			var pa clip.Path
			pa.Begin(gtx.Ops)
			pa.MoveTo(seg[0].Mul(esc))
			for _, q := range seg[1:] {
				pa.LineTo(q.Mul(esc))
			}
			if t.cerrado[k] {
				pa.Close()
			}
			spec := pa.End()
			if t.relleno {
				paint.FillShape(gtx.Ops, c, clip.Outline{Path: spec}.Op())
			} else {
				paint.FillShape(gtx.Ops, c, clip.Stroke{Path: spec, Width: t.ancho * esc}.Op())
			}
			// Round caps: a dot at each open end (the drawings are made with round line caps).
			if !t.cerrado[k] && !t.relleno {
				for _, q := range []f32.Point{seg[0], seg[len(seg)-1]} {
					r := t.ancho * esc / 2
					st := op.Offset(q.Mul(esc).Sub(f32.Pt(r, r)).Round()).Push(gtx.Ops)
					paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: f32.Pt(2*r, 2*r).Round()}.Op(gtx.Ops))
					st.Pop()
				}
			}
		}
	}
	return layout.Dimensions{Size: image.Pt(s, s)}
}
