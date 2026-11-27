package main

import (
	"math"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/mkaand/allmy-remote/internal/telemetry"
	"golang.org/x/sys/windows"
)

type memoryStatus struct {
	Length, Load                uint32
	TotalPhysical, FreePhysical uint64
	TotalPageFile, FreePageFile uint64
	TotalVirtual, FreeVirtual   uint64
	FreeExtendedVirtual         uint64
}

func percentage(used, total uint64) *uint8 {
	if total == 0 || used > total {
		return nil
	}
	v := uint8(math.Round(100 * float64(used) / float64(total)))
	return &v
}

func ticks(t windows.Filetime) uint64 {
	return uint64(t.HighDateTime)<<32 | uint64(t.LowDateTime)
}

func windowsCollector() func() telemetry.Metrics {
	var lastIdle, lastKernel, lastUser uint64
	var baseline bool
	return func() telemetry.Metrics {
		var out telemetry.Metrics
		var idle, kernel, user windows.Filetime
		ok, _, _ := kernel32.NewProc("GetSystemTimes").Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
		groups, _, _ := kernel32.NewProc("GetActiveProcessorGroupCount").Call()
		if ok != 0 && groups == 1 {
			i, k, u := ticks(idle), ticks(kernel), ticks(user)
			if baseline && i >= lastIdle && k >= lastKernel && u >= lastUser {
				total, idleDelta := k-lastKernel+u-lastUser, i-lastIdle
				if idleDelta <= total {
					out.CPU = percentage(total-idleDelta, total)
				}
			}
			lastIdle, lastKernel, lastUser, baseline = i, k, u, true
		}
		mem := memoryStatus{Length: uint32(unsafe.Sizeof(memoryStatus{}))}
		ok, _, _ = kernel32.NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&mem)))
		if ok != 0 && mem.FreePhysical <= mem.TotalPhysical {
			out.RAM = percentage(mem.TotalPhysical-mem.FreePhysical, mem.TotalPhysical)
		}
		root := filepath.VolumeName(os.Getenv("SystemRoot"))
		if len(root) == 2 && root[1] == ':' {
			name, err := windows.UTF16PtrFromString(root + `\`)
			if err == nil {
				var available, total, free uint64
				ok, _, _ = kernel32.NewProc("GetDiskFreeSpaceExW").Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&available)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
				if ok != 0 && free <= total {
					out.Disk = percentage(total-free, total)
				}
			}
		}
		return out
	}
}
