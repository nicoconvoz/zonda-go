//go:build windows

package bandeja

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")
	kernel  = syscall.NewLazyDLL("kernel32.dll")

	registerClassEx       = user32.NewProc("RegisterClassExW")
	createWindowEx        = user32.NewProc("CreateWindowExW")
	defWindowProc         = user32.NewProc("DefWindowProcW")
	getMessage            = user32.NewProc("GetMessageW")
	translateMessage      = user32.NewProc("TranslateMessage")
	dispatchMessage       = user32.NewProc("DispatchMessageW")
	registerWindowMessage = user32.NewProc("RegisterWindowMessageW")
	createPopupMenu       = user32.NewProc("CreatePopupMenu")
	appendMenu            = user32.NewProc("AppendMenuW")
	trackPopupMenu        = user32.NewProc("TrackPopupMenu")
	destroyMenu           = user32.NewProc("DestroyMenu")
	setForegroundWindow   = user32.NewProc("SetForegroundWindow")
	postMessage           = user32.NewProc("PostMessageW")
	getCursorPos          = user32.NewProc("GetCursorPos")
	getSystemMetrics      = user32.NewProc("GetSystemMetrics")
	createIconFromRes     = user32.NewProc("CreateIconFromResourceEx")
	showWindowAsync       = user32.NewProc("ShowWindowAsync")
	isIconic              = user32.NewProc("IsIconic")
	shellNotifyIcon       = shell32.NewProc("Shell_NotifyIconW")
	getModuleHandle       = kernel.NewProc("GetModuleHandleW")
)

const (
	wmNull       = 0x0000
	wmSetIcon    = 0x0080
	wmLButtonUp  = 0x0202
	wmRButtonUp  = 0x0205
	wmAvisoIcono = 0x8001 // WM_APP+1: what the tray icon sends this window
	nimAdd       = 0
	nimDelete    = 2
	nifMessage   = 1
	nifIcon      = 2
	nifTip       = 4
	mfString     = 0
	mfSeparator  = 0x800
	tpmReturnCmd = 0x100
	tpmRightBtn  = 0x2
	smCxIcon     = 11
	smCxSmIcon   = 49
	swHide       = 0
	swShow       = 5
	swRestore    = 9
	hwndMessage  = ^uintptr(2) // HWND_MESSAGE (-3): a window that only receives messages
	cmdAbrir     = 1
	cmdSalir     = 2
)

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

var estado struct {
	hwnd           uintptr
	datos          notifyIconData
	abrir, salir   func()
	taskbarCreated uintptr
	iconoChico     uintptr
	iconoGrande    uintptr
}

// icono makes a Windows icon from the .ico image nearest lado pixels.
func icono(ico []byte, lado int) uintptr {
	img, l, err := elegirImagen(ico, lado)
	if err != nil {
		return 0
	}
	h, _, _ := createIconFromRes.Call(uintptr(unsafe.Pointer(&img[0])), uintptr(len(img)), 1, 0x00030000, uintptr(l), uintptr(l), 0)
	return h
}

func metrica(i int) int {
	v, _, _ := getSystemMetrics.Call(uintptr(i))
	return int(v)
}

// Arrancar puts the icon next to the clock. abrir runs on a click on it or on "Abrir Zonda"; salir on "Salir de
// Zonda". Its window and message loop live on their own OS thread, asleep in GetMessage until something happens.
func Arrancar(abrir, salir func()) error {
	estado.abrir, estado.salir = abrir, salir
	listo := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		err := crear()
		listo <- err
		if err != nil {
			return
		}
		var m msg
		for {
			r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			translateMessage.Call(uintptr(unsafe.Pointer(&m)))
			dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	return <-listo
}

func crear() error {
	inst, _, _ := getModuleHandle.Call(0)
	clase, _ := syscall.UTF16PtrFromString("ZondaBandeja")
	wc := wndClassEx{lpfnWndProc: syscall.NewCallback(proc), hInstance: inst, lpszClassName: clase}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if r, _, err := registerClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return errors.New("no se registró la ventana de la bandeja: " + err.Error())
	}
	h, _, err := createWindowEx.Call(0, uintptr(unsafe.Pointer(clase)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, inst, 0)
	if h == 0 {
		return errors.New("no se creó la ventana de la bandeja: " + err.Error())
	}
	estado.hwnd = h
	tb, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	estado.taskbarCreated, _, _ = registerWindowMessage.Call(uintptr(unsafe.Pointer(tb)))
	estado.iconoChico = icono(icoBandeja, metrica(smCxSmIcon))
	estado.iconoGrande = icono(icoVentana, metrica(smCxIcon))
	d := &estado.datos
	d.cbSize = uint32(unsafe.Sizeof(*d))
	d.hWnd, d.uID, d.uFlags, d.uCallbackMessage, d.hIcon = h, 1, nifMessage|nifIcon|nifTip, wmAvisoIcono, estado.iconoChico
	copy(d.szTip[:], syscall.StringToUTF16("Zonda"))
	return agregar()
}

func agregar() error {
	if r, _, _ := shellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&estado.datos))); r == 0 {
		return errors.New("Windows no aceptó el ícono de la bandeja")
	}
	return nil
}

// Quitar takes the icon away before quitting; otherwise it stays next to the clock until the mouse passes over it.
func Quitar() {
	if estado.hwnd != 0 {
		shellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&estado.datos)))
	}
}

func proc(hwnd, m, wp, lp uintptr) uintptr {
	switch {
	case m == wmAvisoIcono && lp == wmLButtonUp:
		go estado.abrir()
		return 0
	case m == wmAvisoIcono && lp == wmRButtonUp:
		menu(hwnd)
		return 0
	case estado.taskbarCreated != 0 && m == estado.taskbarCreated:
		// Explorer restarted and the taskbar came back empty: the icon goes in again.
		agregar()
		return 0
	}
	r, _, _ := defWindowProc.Call(hwnd, m, wp, lp)
	return r
}

func menu(hwnd uintptr) {
	mn, _, _ := createPopupMenu.Call()
	if mn == 0 {
		return
	}
	defer destroyMenu.Call(mn)
	abrir, _ := syscall.UTF16PtrFromString("Abrir Zonda")
	salir, _ := syscall.UTF16PtrFromString("Salir de Zonda")
	appendMenu.Call(mn, mfString, cmdAbrir, uintptr(unsafe.Pointer(abrir)))
	appendMenu.Call(mn, mfSeparator, 0, 0)
	appendMenu.Call(mn, mfString, cmdSalir, uintptr(unsafe.Pointer(salir)))
	var p struct{ x, y int32 }
	getCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	// Windows' rule for tray menus: the window comes forward first, so the menu closes on a click elsewhere.
	setForegroundWindow.Call(hwnd)
	cmd, _, _ := trackPopupMenu.Call(mn, tpmReturnCmd|tpmRightBtn, uintptr(p.x), uintptr(p.y), 0, hwnd, 0)
	postMessage.Call(hwnd, wmNull, 0, 0)
	switch cmd {
	case cmdAbrir:
		go estado.abrir()
	case cmdSalir:
		go estado.salir()
	}
}

// Esconder hides the program's window (the program keeps running). Asynchronous, so it is safe to call from inside
// the window's own events.
func Esconder(ventana uintptr) {
	if ventana != 0 {
		showWindowAsync.Call(ventana, swHide)
	}
}

// Mostrar brings the window back, restored if it was minimized, and in front.
func Mostrar(ventana uintptr) {
	if ventana == 0 {
		return
	}
	if r, _, _ := isIconic.Call(ventana); r != 0 {
		showWindowAsync.Call(ventana, swRestore)
	} else {
		showWindowAsync.Call(ventana, swShow)
	}
	setForegroundWindow.Call(ventana)
}

// PonerIcono gives the window Zonda's icon in its title bar and the taskbar; posted, so it never waits on the
// window's thread.
func PonerIcono(ventana uintptr) {
	if ventana == 0 {
		return
	}
	if estado.iconoChico != 0 {
		postMessage.Call(ventana, wmSetIcon, 0, estado.iconoChico)
	}
	if estado.iconoGrande != 0 {
		postMessage.Call(ventana, wmSetIcon, 1, estado.iconoGrande)
	}
}
