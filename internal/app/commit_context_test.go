package app

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mygittu/internal/commitai"
)

func TestCompactCommitContextKeepsLaterFilesWithinBudget(t *testing.T) {
	patch := "diff --git a/first.go b/first.go\n+" + strings.Repeat("改", commitai.MaxPatch) +
		"\ndiff --git a/last.go b/last.go\n+last relevant change\n"
	context := compactCommitContext("2 files changed", patch)
	if len(context) > commitai.MaxPatch || !utf8.ValidString(context) {
		t.Fatal("compact context exceeded its byte budget or split a UTF-8 character")
	}
	for _, expected := range []string{"2 files changed", "first.go", "last.go", "last relevant change", "[Excerpt truncated]"} {
		if !strings.Contains(context, expected) {
			t.Errorf("compact context omitted %q", expected)
		}
	}
}
