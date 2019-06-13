//+build linux

package fs

import (
	"os"
	"syscall"
)

// canCow does not in fact refer to bovines in any way, it indicates whether we think
// we can do copy-on-write operations on the filesystem we're on. On Linux this means that
// we're running on btrfs, bcachefs, zfs or similar that support the FICLONE ioctl.
var canCow = true

const ficlone = 1074041865

func cowFile(from, to string, mode os.FileMode) error {
	fromfd, err := syscall.Open(from, os.O_RDONLY, uint32(mode))
	if err != nil {
		return err
	}
	defer syscall.Close(fromfd)
	tofd, err := syscall.Open(to, os.O_WRONLY|os.O_CREATE, uint32(mode))
	if err != nil {
		return err
	}
	defer syscall.Close(tofd)
	_, _, err = syscall.Syscall(ficlone, uintptr(tofd), uintptr(fromfd), 0)
	return err
}
