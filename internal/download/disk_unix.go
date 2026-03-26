//go:build !windows

package download

import (
	"fmt"
	"syscall"
)

// AvailableSpace returns available bytes on the filesystem containing path.
func AvailableSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}
