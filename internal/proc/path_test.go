package proc

import (
	"path/filepath"
	"testing"
)

func TestExtendedPath(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	missing := filepath.Join(a, "nope")
	got := extendedPath("/usr/bin"+string(filepath.ListSeparator)+a, []string{a, missing, b, b})
	want := "/usr/bin" + string(filepath.ListSeparator) + a + string(filepath.ListSeparator) + b
	if got != want {
		t.Errorf("extendedPath = %q, want %q", got, want)
	}
	if got := extendedPath("", []string{b}); got != b {
		t.Errorf("empty PATH: got %q, want %q", got, b)
	}
}
