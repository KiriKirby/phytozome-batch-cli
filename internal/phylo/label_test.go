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

func TestFormatYTLabelUsesGeneLocusSpeciesInitialsAndLabelName(t *testing.T) {
	tests := []struct {
		name      string
		species   string
		geneLocus string
		labelName string
		want      string
	}{
		{
			name:      "Panicum virgatum",
			species:   "Panicum virgatum",
			geneLocus: "Pavir.6KG154400",
			labelName: "Pv4CL1",
			want:      "Pavir.6KG154400_Pv4CL1",
		},
		{
			name:      "Arabidopsis thaliana",
			species:   "Arabidopsis thaliana",
			geneLocus: "At1G51680",
			labelName: "At4CL1",
			want:      "At1G51680_At4CL1",
		},
		{
			name:      "missing label",
			species:   "Arabidopsis thaliana",
			geneLocus: "At1G51680",
			labelName: "",
			want:      "At1G51680",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatYTLabel(tt.species, tt.geneLocus, tt.labelName)
			if got != tt.want {
				t.Fatalf("YT label = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatYTV2LabelUsesGeneLocusAndLabelName(t *testing.T) {
	if got := FormatYTV2Label("Arabidopsis thaliana", "At1G51680", "At4CL1"); got != "At1G51680_4CL1" {
		t.Fatalf("YT v2 label = %q", got)
	}
	if got := FormatYTV2Label("Arabidopsis thaliana", "At1G51680", ""); got != "At1G51680" {
		t.Fatalf("YT v2 label without label = %q", got)
	}
}
