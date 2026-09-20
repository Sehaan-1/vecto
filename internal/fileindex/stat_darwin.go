//go:build darwin

package fileindex

import "syscall"

func rawLstat(path string) (Stat, error) {
	var st syscall.Stat_t
	if err := syscall.Lstat(path, &st); err != nil {
		return Stat{}, err
	}
	return Stat{
		Dev:    uint64(st.Dev),
		Ino:    uint64(st.Ino),
		CTime:  st.Ctimespec.Sec,
		CNTime: st.Ctimespec.Nsec,
		MTime:  st.Mtimespec.Sec,
		MNTime: st.Mtimespec.Nsec,
		Size:   uint64(st.Size),
	}, nil
}
