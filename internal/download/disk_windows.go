//go:build windows

package download

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// AvailableSpace returns available bytes on the filesystem containing path.
func AvailableSpace(path string) (uint64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("invalid path %s: %w", path, err)
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	err = windows.GetDiskFreeSpaceEx(
		pathPtr,
		(*uint64)(unsafe.Pointer(&freeBytesAvailable)),
		(*uint64)(unsafe.Pointer(&totalBytes)),
		(*uint64)(unsafe.Pointer(&totalFreeBytes)),
	)
	if err != nil {
		return 0, fmt.Errorf("GetDiskFreeSpaceEx %s: %w", path, err)
	}
	return freeBytesAvailable, nil
}
