package ui

import "testing"

func TestBundledFontRegisters(t *testing.T) {
	if family := appFontFamily(); family != appFontName {
		t.Fatalf("expected bundled font %q, got %q", appFontName, family)
	}
}
