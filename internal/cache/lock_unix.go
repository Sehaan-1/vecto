//go:build !windows

package cache

import (
	"os"
	"syscall"
)

func lockFile(f *os.File, lockType LockType) error {
	how := syscall.LOCK_SH
	if lockType == LockExclusive {
		how = syscall.LOCK_EX
	}
	return syscall.Flock(int(f.Fd()), how)
}

func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
