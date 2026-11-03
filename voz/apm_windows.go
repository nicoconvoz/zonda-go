//go:build windows

package voz

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// APM is WebRTC's own audio processing module, the real one (webrtc-audio-processing, the same code libwebrtc runs on
// every call: high-pass, echo canceller AEC3, noise suppressor, AGC2), reached through apm/zapm.cc, a small C bridge
// compiled with MSYS2's gcc. The DLLs travel inside this program and are written out once to the user's folder; Go
// calls them with syscall, no cgo. When they cannot load, the translated pieces (Procesador) take over.
type APM struct {
	h         uintptr
	retardoMs int
}

//go:embed apm_dlls/*.dll
var dllsAPM embed.FS

// The load order matters only for the first one asked for; the rest are found next to it.
var ordenDLL = []string{"libwinpthread-1.dll", "libgcc_s_seh-1.dll", "libstdc++-6.dll", "libwebrtc-audio-processing-1-3.dll", "zapm.dll", "zvid.dll"}

// extraer writes the DLLs into a folder named by their fingerprint, so a new build never loads an old copy.
func extraer() (string, error) {
	h := sha256.New()
	for _, n := range ordenDLL {
		b, err := dllsAPM.ReadFile("apm_dlls/" + n)
		if err != nil {
			return "", err
		}
		h.Write(b)
	}
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "ZondaGo", "apm-"+hex.EncodeToString(h.Sum(nil))[:12])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for _, n := range ordenDLL {
		p := filepath.Join(dir, n)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		b, _ := dllsAPM.ReadFile("apm_dlls/" + n)
		if err := os.WriteFile(p+".tmp", b, 0o644); err != nil {
			return "", err
		}
		os.Rename(p+".tmp", p)
	}
	return dir, nil
}

var (
	loadLibraryEx = kernel.NewProc("LoadLibraryExW")
	getProc       = kernel.NewProc("GetProcAddress")
)

const loadWithAlteredSearchPath = 0x8

var cargadas = map[string]uintptr{}

// cargar loads every DLL of the folder once; the handle of the one asked for comes back.
func cargar(dir, cual string) (uintptr, error) {
	for _, n := range ordenDLL {
		if cargadas[n] != 0 {
			continue
		}
		p, _ := syscall.UTF16PtrFromString(filepath.Join(dir, n))
		r, _, err := loadLibraryEx.Call(uintptr(unsafe.Pointer(p)), 0, loadWithAlteredSearchPath)
		if r == 0 {
			return 0, errors.New("no cargó " + n + ": " + err.Error())
		}
		cargadas[n] = r
	}
	return cargadas[cual], nil
}

func funcion(dll uintptr, nombre string) (uintptr, error) {
	p, _ := syscall.BytePtrFromString(nombre)
	r, _, _ := getProc.Call(dll, uintptr(unsafe.Pointer(p)))
	if r == 0 {
		return 0, errors.New("falta " + nombre)
	}
	return r, nil
}

var apmCargada struct {
	crear, microfono, parlante, destruir uintptr
	err                                  error
	lista                                bool
}

func cargarAPM() error {
	if apmCargada.lista {
		return apmCargada.err
	}
	apmCargada.lista = true
	dir, err := extraer()
	if err == nil {
		var dll uintptr
		if dll, err = cargar(dir, "zapm.dll"); err == nil {
			for _, f := range []struct {
				n string
				p *uintptr
			}{{"zapm_crear", &apmCargada.crear}, {"zapm_microfono", &apmCargada.microfono}, {"zapm_parlante", &apmCargada.parlante}, {"zapm_destruir", &apmCargada.destruir}} {
				if *f.p, err = funcion(dll, f.n); err != nil {
					break
				}
			}
		}
	}
	apmCargada.err = err
	return err
}

// NuevoAPM opens WebRTC's processing for one call. retardoMs: how long the speaker's sound takes to come back into
// the microphone through this computer (its buffers); AEC3 refines it on its own.
func NuevoAPM(retardoMs int) (*APM, error) {
	if err := cargarAPM(); err != nil {
		return nil, err
	}
	h, _, _ := syscall.SyscallN(apmCargada.crear, uintptr(retardoMs))
	if h == 0 {
		return nil, errors.New("WebRTC no creó el procesador")
	}
	return &APM{h: h, retardoMs: retardoMs}, nil
}

const muestras10ms = 480

// Microfono processes captured audio in place (a multiple of 10 ms).
func (a *APM) Microfono(pcm []int16) {
	for i := 0; i+muestras10ms <= len(pcm); i += muestras10ms {
		syscall.SyscallN(apmCargada.microfono, a.h, uintptr(unsafe.Pointer(&pcm[i])), uintptr(a.retardoMs))
	}
}

// Parlante hands what the speaker plays to the echo canceller (a multiple of 10 ms), untouched.
func (a *APM) Parlante(pcm []int16) {
	for i := 0; i+muestras10ms <= len(pcm); i += muestras10ms {
		syscall.SyscallN(apmCargada.parlante, a.h, uintptr(unsafe.Pointer(&pcm[i])))
	}
}

func (a *APM) Cerrar() {
	if a != nil && a.h != 0 {
		syscall.SyscallN(apmCargada.destruir, a.h)
		a.h = 0
	}
}

// errorAPM: why WebRTC's DLLs did not load, for the voice line.
func errorAPM() string {
	if apmCargada.err == nil {
		return "sin intentar"
	}
	return apmCargada.err.Error()
}
