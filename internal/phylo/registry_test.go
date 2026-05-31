package phylo

import "testing"

func TestAlignmentDefinitionsForKindUseKindSpecificDefaults(t *testing.T) {
	proteinDefs := AlignmentDefinitionsForKind(SequenceProtein)
	nucleotideDefs := AlignmentDefinitionsForKind(SequenceNucleotide)

	var proteinClustal, proteinMuscle, nucleotideClustal, nucleotideMuscle MethodDefinition
	var ok bool

	for _, def := range proteinDefs {
		switch def.ID {
		case "clustalw_protein":
			proteinClustal = def
		case "muscle_protein":
			proteinMuscle = def
		}
	}
	for _, def := range nucleotideDefs {
		switch def.ID {
		case "clustalw":
			nucleotideClustal = def
		case "muscle":
			nucleotideMuscle = def
		}
	}

	if proteinClustal.ID == "" || proteinMuscle.ID == "" || nucleotideClustal.ID == "" || nucleotideMuscle.ID == "" {
		t.Fatalf("missing expected kind-specific alignment definitions")
	}

	if ok = hasDefaultParam(proteinClustal, "pairwise_gap_opening_penalty", "10.00"); !ok {
		t.Fatalf("protein ClustalW should keep protein default gap opening penalty")
	}
	if ok = hasDefaultParam(nucleotideClustal, "pairwise_gap_opening_penalty", "15.00"); !ok {
		t.Fatalf("nucleotide ClustalW should use nucleotide default gap opening penalty")
	}
	if ok = hasDefaultParam(nucleotideClustal, "transition_weight", "0.50"); !ok {
		t.Fatalf("nucleotide ClustalW should expose transition weight")
	}
	if ok = hasDefaultParam(proteinMuscle, "max_iterations", "8"); !ok {
		t.Fatalf("protein MUSCLE should use MEGA default max iterations")
	}
	if ok = hasDefaultParam(nucleotideMuscle, "gap_open", "-400.00"); !ok {
		t.Fatalf("nucleotide MUSCLE should use nucleotide gap-open default")
	}
}

func TestNormalizeTreeSettingsForKindChoosesCompatibleMethodVariant(t *testing.T) {
	settings := NormalizeTreeSettingsForKind(DefaultTreeSettings(), SequenceProtein)
	if settings.AlignmentMethod != AlignmentMethod("clustalw_protein") {
		t.Fatalf("protein default alignment method = %q", settings.AlignmentMethod)
	}

	settings = NormalizeTreeSettingsForKind(TreeSettings{AlignmentMethod: AlignmentClustalW}, SequenceProtein)
	if settings.AlignmentMethod != AlignmentMethod("clustalw_protein") {
		t.Fatalf("protein normalization should switch base clustalw to protein variant, got %q", settings.AlignmentMethod)
	}

	settings = NormalizeTreeSettingsForKind(TreeSettings{AlignmentMethod: AlignmentMUSCLE}, SequenceProtein)
	if settings.AlignmentMethod != AlignmentMethod("muscle_protein") {
		t.Fatalf("protein normalization should switch base muscle to protein variant, got %q", settings.AlignmentMethod)
	}
}

func hasDefaultParam(def MethodDefinition, id string, want string) bool {
	for _, param := range def.Parameters {
		if param.ID == id && param.Default == want {
			return true
		}
	}
	return false
}
