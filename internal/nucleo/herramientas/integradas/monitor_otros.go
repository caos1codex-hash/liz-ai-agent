//go:build !linux

package integradas

// syscallStatvfs stub vacío para no-Linux.
type syscallStatvfs struct {
	f_bsize  uint64
	f_frsize uint64
	f_blocks uint64
	f_bfree  uint64
	f_bavail uint64
	f_files  uint64
	f_ffree  uint64
	f_favail uint64
	f_fsid   uint64
	f_flag   uint64
	f_namemax uint64
}

// statvfs stub para no-Linux.
func statvfs(path string, s *syscallStatvfs) error {
	return nil
}
