package nucleo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Almacen keeps the device's data as key/value text in one JSON file (the KeyValueStore of the Dart core), and
// photos and files as their own files named by fingerprint. Writes are gathered and saved once a second at most.
type Almacen struct {
	dir     string
	mu      sync.Mutex
	datos   map[string]string
	sucio   bool
	timer   *time.Timer
}

func AbrirAlmacen(dir string) (*Almacen, error) {
	if err := os.MkdirAll(filepath.Join(dir, "fotos"), 0o755); err != nil {
		return nil, err
	}
	a := &Almacen{dir: dir, datos: map[string]string{}}
	if b, err := os.ReadFile(filepath.Join(dir, "datos.json")); err == nil {
		json.Unmarshal(b, &a.datos)
	}
	return a, nil
}

func (a *Almacen) Leer(k string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.datos[k]
}

func (a *Almacen) Escribir(k, v string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.datos[k] == v {
		return
	}
	a.datos[k] = v
	if !a.sucio {
		a.sucio = true
		a.timer = time.AfterFunc(time.Second, a.Guardar)
	}
}

// Guardar writes the file now (also on quitting).
func (a *Almacen) Guardar() {
	a.mu.Lock()
	if !a.sucio {
		a.mu.Unlock()
		return
	}
	a.sucio = false
	b, _ := json.Marshal(a.datos)
	a.mu.Unlock()
	tmp := filepath.Join(a.dir, "datos.json.tmp")
	if os.WriteFile(tmp, b, 0o600) == nil {
		os.Rename(tmp, filepath.Join(a.dir, "datos.json"))
	}
}

// LeerJSON decodes a stored value into v (exact numbers); false when absent or unreadable.
func (a *Almacen) LeerJSON(k string) (any, bool) {
	s := a.Leer(k)
	if s == "" {
		return nil, false
	}
	return DecodificarValor([]byte(s))
}

func (a *Almacen) EscribirJSON(k string, v any) { a.Escribir(k, Codificar(v)) }

func (a *Almacen) Foto(hash string) []byte {
	b, err := os.ReadFile(filepath.Join(a.dir, "fotos", hash))
	if err != nil {
		return nil
	}
	return b
}

func (a *Almacen) GuardarFoto(hash string, b []byte) {
	os.WriteFile(filepath.Join(a.dir, "fotos", hash), b, 0o600)
}

func (a *Almacen) Dir() string { return a.dir }
