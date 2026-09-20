//go:build windows

package fileindex

import "os"

// Windows syscall.Stat_t has no Dev/Ino/Ctim fields.
// Fall back to mtime + size only; this loses the inode-equality fast-path
// but is still correct (mismatched mtime/size triggers re-hash).
func rawLstat(path string) (Stat, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return Stat{}, err
	}
	mt := fi.ModTime()
	return Stat{
		Dev:    0,
		Ino:    0,
		CTime:  mt.Unix(),
		CNTime: int64(mt.Nanosecond()),
		MTime:  mt.Unix(),
		MNTime: int64(mt.Nanosecond()),
		Size:   uint64(fi.Size()),
	}, nil
}
