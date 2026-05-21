package orchestrate

import (
	"testing"
)

func TestDetectInstallMethod_ReturnsValue(t *testing.T) {
	// This just verifies the function runs without panic and returns a valid method.
	method := DetectInstallMethod()
	if method < InstallHomebrew || method > InstallManual {
		t.Errorf("unexpected install method: %d", method)
	}
}

func TestInstallMethod_String(t *testing.T) {
	tests := []struct {
		method InstallMethod
		want   string
	}{
		{InstallHomebrew, "homebrew"},
		{InstallGoInstall, "go install"},
		{InstallManual, "manual"},
	}
	for _, tc := range tests {
		if got := tc.method.String(); got != tc.want {
			t.Errorf("InstallMethod(%d).String() = %q, want %q", tc.method, got, tc.want)
		}
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.13.1", "1.13.1"},
		{"1.13.1", "1.13.1"},
		{"v0.1.0", "0.1.0"},
		{"dev", "dev"},
	}
	for _, tc := range tests {
		if got := NormalizeVersion(tc.input); got != tc.want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
