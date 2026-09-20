//go:build !linux && !darwin && !windows

package fileindex

import "os"

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
