//go:build linux

package integradas

import (
	"syscall"
	"unsafe"
)

// syscallStatvfs es el struct compat entre plataformas.
type syscallStatvfs struct {
	f_bsize   uint64
	f_frsize  uint64
	f_blocks  uint64
	f_bfree   uint64
	f_bavail  uint64
	f_files   uint64
	f_ffree   uint64
	f_favail  uint64
	f_fsid    uint64
	f_flag    uint64
	f_namemax uint64
}

// statvfs llama a la syscall statfs(2) en Linux.
func statvfs(path string, s *syscallStatvfs) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return err
	}
	s.f_bsize = uint64(st.Bsize)
	s.f_frsize = uint64(st.Bsize)
	s.f_blocks = st.Blocks
	s.f_bfree = st.Bfree
	s.f_bavail = st.Bavail
	s.f_files = st.Files
	s.f_ffree = st.Ffree
	s.f_favail = st.Ffree
	return nil
}

// unsafe import evitado (no usado)
var _ = unsafe.Sizeof(0)
