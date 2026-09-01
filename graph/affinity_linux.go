//go:build linux && !unsupported

package graph

import (
	"syscall"
	"unsafe"
)

type linuxAffinity struct{}

func newPlatformAffinity() affinityAdapter { return linuxAffinity{} }

func (linuxAffinity) bind(cpu int) error {
	const cpuSetBytes = 128
	if cpu/8 >= cpuSetBytes {
		return syscall.EINVAL
	}
	var set [cpuSetBytes]byte
	set[cpu/8] = 1 << uint(cpu%8)
	_, _, errno := syscall.RawSyscall(syscall.SYS_SCHED_SETAFFINITY, 0, uintptr(len(set)), uintptr(unsafe.Pointer(&set[0])))
	if errno != 0 {
		return errno
	}
	return nil
}
