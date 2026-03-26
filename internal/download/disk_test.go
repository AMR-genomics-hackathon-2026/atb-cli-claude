package download

import (
	"testing"
)

func TestAvailableSpace(t *testing.T) {
	bytes, err := AvailableSpace("/tmp")
	if err != nil {
		t.Fatalf("AvailableSpace(/tmp) error: %v", err)
	}
	if bytes == 0 {
		t.Error("expected available space > 0 for /tmp, got 0")
	}
}

func TestCheckDiskSpace(t *testing.T) {
	t.Run("small download passes", func(t *testing.T) {
		err := CheckDiskSpace("/tmp", 1024, 0)
		if err != nil {
			t.Errorf("expected no error for 1024 bytes on /tmp, got: %v", err)
		}
	})

	t.Run("huge download fails", func(t *testing.T) {
		const oneExabyte = uint64(1 << 60)
		err := CheckDiskSpace("/tmp", oneExabyte, 0)
		if err == nil {
			t.Error("expected error for 1 exabyte download on /tmp, got nil")
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
