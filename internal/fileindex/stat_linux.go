//go:build linux

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
		CTime:  st.Ctim.Sec,
		CNTime: st.Ctim.Nsec,
		MTime:  st.Mtim.Sec,
		MNTime: st.Mtim.Nsec,
		Size:   uint64(st.Size),
	}, nil
}
