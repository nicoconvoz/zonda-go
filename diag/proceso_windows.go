//go:build windows

package diag

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	getProcessTimes   = kernel32.NewProc("GetProcessTimes")
	getProcessMemInfo = kernel32.NewProc("K32GetProcessMemoryInfo")
	getCurrentProcess = kernel32.NewProc("GetCurrentProcess")
)

// cpuProceso: the processor time of the whole process (every thread, the DLLs' too), user plus kernel.
func cpuProceso() time.Duration {
	h, _, _ := getCurrentProcess.Call()
	var creado, salido, kern, usr syscall.Filetime
	getProcessTimes.Call(h, uintptr(unsafe.Pointer(&creado)), uintptr(unsafe.Pointer(&salido)), uintptr(unsafe.Pointer(&kern)), uintptr(unsafe.Pointer(&usr)))
	n := func(f syscall.Filetime) int64 { return int64(f.HighDateTime)<<32 | int64(f.LowDateTime) }
	return time.Duration((n(kern) + n(usr)) * 100)
}

// privadaProceso: the private bytes of the process, what Task Manager's "memory (private)" shows.
func privadaProceso() uint64 {
	h, _, _ := getCurrentProcess.Call()
	var c struct {
		cb                                                                 uint32
		pageFaults                                                         uint32
		peakWorkingSet, workingSet, quotaPeakPaged, quotaPaged             uintptr
		quotaPeakNonPaged, quotaNonPaged, pagefileUsage, peakPagefileUsage uintptr
		privateUsage                                                       uintptr
	}
	c.cb = uint32(unsafe.Sizeof(c))
	getProcessMemInfo.Call(h, uintptr(unsafe.Pointer(&c)), uintptr(c.cb))
	return uint64(c.privateUsage)
}
