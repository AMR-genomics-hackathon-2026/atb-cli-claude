package download

import "fmt"

// CheckDiskSpace verifies there's enough space for the download.
// It checks that available space on dir is at least neededBytes plus minFreeGB gigabytes.
func CheckDiskSpace(dir string, neededBytes uint64, minFreeGB int) error {
	available, err := AvailableSpace(dir)
	if err != nil {
		return err
	}

	minFreeBytes := uint64(minFreeGB) * 1024 * 1024 * 1024
	required := neededBytes + minFreeBytes

	if available < required {
		return fmt.Errorf(
			"insufficient disk space: need %s, have %s available",
			FormatBytes(required),
			FormatBytes(available),
		)
	}
	return nil
}

// FormatBytes formats bytes as a human-readable string using 1024-based units.
func FormatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
