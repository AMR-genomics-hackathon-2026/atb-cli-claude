package selfupdate

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current string
		remote  string
		want    bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"v0.3.0", "v0.2.0", false},
		{"0.1.0", "0.2.0", true},
		{"dev", "v1.0.0", false},
		{"", "v1.0.0", false},
		{"v0.1.0", "v0.1.1", true},
	}
	for _, tt := range tests {
		got := CompareVersions(tt.current, tt.remote)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %v, want %v", tt.current, tt.remote, got, tt.want)
		}
	}
}

func TestFindAsset(t *testing.T) {
	release := &Release{
		Assets: []Asset{
			{Name: "atb-cli_0.1.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64.tar.gz"},
			{Name: "atb-cli_0.1.0_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64.tar.gz"},
			{Name: "atb-cli_0.1.0_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows_amd64.zip"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
		},
	}

	asset := FindAsset(release)
	if asset == nil {
		t.Fatal("FindAsset returned nil for current platform")
	}
	if asset.BrowserDownloadURL == "" {
		t.Error("asset URL should not be empty")
	}
}
