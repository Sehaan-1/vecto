package cache

import (
	"fmt"
	"os"
	"path/filepath"
)

// LockType indicates shared or exclusive lock.
type LockType int

const (
	LockShared LockType = iota
	LockExclusive
)

// FileLock represents an active OS-level file lock.
type FileLock interface {
	Unlock() error
}

// AcquireLock acquires an OS-level file lock on lockPath.
// If lockPath directory doesn't exist, it creates it.
func AcquireLock(lockPath string, lockType LockType) (FileLock, error) {
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating lock dir: %w", err)
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, fmt.Errorf("opening lock file %s: %w", lockPath, err)
	}

	if err := lockFile(f, lockType); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquiring lock on %s: %w", lockPath, err)
	}

	return &osFileLock{file: f}, nil
}

type osFileLock struct {
	file *os.File
}

func (l *osFileLock) Unlock() error {
	if l.file == nil {
		return nil
	}
	defer func() {
		_ = l.file.Close()
		l.file = nil
	}()
	return unlockFile(l.file)
}
