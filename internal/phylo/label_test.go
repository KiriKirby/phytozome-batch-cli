package phylo

import "testing"

func TestFormatPHgoLabelUsesSpeciesInitialsGeneIDAndLabelName(t *testing.T) {
	got := FormatPHgoLabel("Oryza sativa Japonica Group", "Os08g14760", "Os4CL1")
	if got != "Os-Os08g14760 (Os4CL1)" {
		t.Fatalf("PHgo label = %q", got)
	}
}

func TestFormatPHgoLabelFillsMissingPartsWithTilde(t *testing.T) {
	got := FormatPHgoLabel("Arabidopsis", "", "")
	if got != "~-~ (~)" {
		t.Fatalf("PHgo missing label = %q", got)
	}
}
