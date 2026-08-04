package fastautil

import "testing"

func TestNormalizeGeneratedHeaderReplacesWhitespace(t *testing.T) {
	const input = ">Arabidopsis thaliana\tPAL1\nlabel"
	const want = ">Arabidopsis_thaliana_PAL1_label"
	if got := NormalizeGeneratedHeader(input); got != want {
		t.Fatalf("NormalizeGeneratedHeader(%q) = %q, want %q", input, got, want)
	}
}

func TestNormalizeGeneratedHeaderDoesNotChangeInputParsing(t *testing.T) {
	if IsIgnoredPHGONoteHeader(">phgo://note legacy header") == false {
		t.Fatal("legacy spaced header should remain readable by parsers")
	}
}
