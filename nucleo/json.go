package nucleo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Msg is a mesh message or any JSON object, decoded with numbers kept exact (Dart ints are 64-bit; a float64 would
// turn a millisecond time into 1.8e+12 on the way out).
type Msg = map[string]any

// Decodificar reads a JSON object; false when the text is not one.
func Decodificar(s string) (Msg, bool) {
	d := json.NewDecoder(strings.NewReader(s))
	d.UseNumber()
	var m Msg
	if d.Decode(&m) != nil || m == nil {
		return nil, false
	}
	return m, true
}

// DecodificarValor reads any JSON value with exact numbers.
func DecodificarValor(b []byte) (any, bool) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return nil, false
	}
	return v, true
}

// Codificar writes JSON without HTML escaping, like Dart's jsonEncode.
func Codificar(v any) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		panic(err)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// Texto: m[k] when it is a string.
func Texto(m Msg, k string) (string, bool) {
	s, ok := m[k].(string)
	return s, ok
}

// Entero: m[k] when it is an integer (Dart's `is int`).
func Entero(m Msg, k string) (int64, bool) { return EnteroDe(m[k]) }

func EnteroDe(v any) (int64, bool) {
	switch x := v.(type) {
	case json.Number:
		if strings.ContainsAny(string(x), ".eE") {
			return 0, false
		}
		n, err := x.Int64()
		return n, err == nil
	case int:
		return int64(x), true
	case int64:
		return x, true
	}
	return 0, false
}

// Objeto: m[k] when it is a JSON object.
func Objeto(m Msg, k string) (Msg, bool) {
	o, ok := m[k].(map[string]any)
	return o, ok
}

// Lista: m[k] when it is a JSON array.
func Lista(m Msg, k string) ([]any, bool) {
	l, ok := m[k].([]any)
	return l, ok
}

// Textos: the strings of a list (`'$x'` for each element in Dart).
func Textos(v any) []string {
	l, _ := v.([]any)
	out := make([]string, 0, len(l))
	for _, x := range l {
		out = append(out, Interpolar(x))
	}
	return out
}

// Interpolar renders a value as Dart's string interpolation does (`'$x'`): null is "null", ints are decimal.
func Interpolar(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case json.Number:
		return string(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case bool:
		return strconv.FormatBool(x)
	}
	return fmt.Sprint(v)
}

// Copiar makes a shallow copy of m.
func Copiar(m Msg) Msg {
	out := make(Msg, len(m)+4)
	for k, v := range m {
		out[k] = v
	}
	return out
}

// canonico writes JSON exactly as Dart's jsonEncode does, for values whose bytes are hashed (Nostr event ids).
func canonico(b *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case string:
		cadenaDart(b, x)
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case int:
		b.WriteString(strconv.Itoa(x))
	case json.Number:
		b.WriteString(string(x))
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			canonico(b, e)
		}
		b.WriteByte(']')
	case [][]string:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteByte('[')
			for j, s := range e {
				if j > 0 {
					b.WriteByte(',')
				}
				cadenaDart(b, s)
			}
			b.WriteByte(']')
		}
		b.WriteByte(']')
	default:
		b.WriteString(Codificar(v))
	}
}

func cadenaDart(b *strings.Builder, s string) {
	const hexa = "0123456789abcdef"
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		i += n
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20:
			b.WriteString(`\u00`)
			b.WriteByte(hexa[r>>4])
			b.WriteByte(hexa[r&0xf])
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
}

// ComoObjeto and ComoLista read a stored or received value of a shape that may not be the expected one: anything else
// reads as empty, never a crash (a list saved empty comes back as null).
func ComoObjeto(v any) Msg {
	m, _ := v.(map[string]any)
	return m
}

func ComoLista(v any) []any {
	l, _ := v.([]any)
	return l
}
