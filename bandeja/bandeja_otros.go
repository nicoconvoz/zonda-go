//go:build !windows

package bandeja

// Only Windows has the tray as the app uses it; elsewhere the window closes as usual.

func Arrancar(abrir, salir func()) error { return nil }
func Quitar()                            {}
func Esconder(ventana uintptr)           {}
func Mostrar(ventana uintptr)            {}
func PonerIcono(ventana uintptr)         {}
