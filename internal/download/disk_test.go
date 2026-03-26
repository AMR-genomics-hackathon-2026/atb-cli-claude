package download

import (
	"os"
	"testing"
)

func TestAvailableSpace(t *testing.T) {
	dir := os.TempDir()
	bytes, err := AvailableSpace(dir)
	if err != nil {
		t.Fatalf("AvailableSpace(%s) error: %v", dir, err)
	}
	if bytes == 0 {
		t.Errorf("expected available space > 0 for %s, got 0", dir)
	}
}

func TestCheckDiskSpace(t *testing.T) {
	dir := os.TempDir()

	t.Run("small download passes", func(t *testing.T) {
		err := CheckDiskSpace(dir, 1024, 0)
		if err != nil {
			t.Errorf("expected no error for 1024 bytes, got: %v", err)
		}
	})

	t.Run("huge download fails", func(t *testing.T) {
		const oneExabyte = uint64(1 << 60)
		err := CheckDiskSpace(dir, oneExabyte, 0)
		if err == nil {
			t.Error("expected error for 1 exabyte download, got nil")
		}
	})
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		input uint64
		want  string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tc := range cases {
		got := FormatBytes(tc.input)
		if got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
