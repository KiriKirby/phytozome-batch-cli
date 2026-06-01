package phylo

import (
	"sort"
	"strings"
	"testing"
)

func TestAlignmentDefinitionsForKindUseKindSpecificDefaults(t *testing.T) {
	proteinDefs := AlignmentDefinitionsForKind(SequenceProtein)
	nucleotideDefs := AlignmentDefinitionsForKind(SequenceNucleotide)

	defs := map[string]MethodDefinition{}

	for _, def := range proteinDefs {
		defs[def.ID] = def
	}
	for _, def := range nucleotideDefs {
		defs[def.ID] = def
	}

	for _, id := range []string{"clustalw_protein", "muscle_protein", "clustalw", "clustalw_codons", "muscle", "muscle_codons"} {
		if defs[id].ID == "" {
			t.Fatalf("missing expected alignment definition %q", id)
		}
	}

	if !hasDefaultParam(defs["clustalw_protein"], "pairwise_gap_opening_penalty", "10") {
		t.Fatalf("protein ClustalW should keep protein default gap opening penalty")
	}
	if !hasDefaultParam(defs["clustalw"], "pairwise_gap_opening_penalty", "15") {
		t.Fatalf("nucleotide ClustalW should use nucleotide default gap opening penalty")
	}
	if !hasDefaultParam(defs["clustalw_codons"], "genetic_code", "Standard") {
		t.Fatalf("codon ClustalW should expose standard genetic code default")
	}
	if !hasDefaultParam(defs["clustalw"], "transition_weight", "0.5") {
		t.Fatalf("nucleotide ClustalW should expose transition weight")
	}
	if !hasDefaultParam(defs["clustalw_protein"], "protein_weight_matrix", "Gonnet") {
		t.Fatalf("protein ClustalW should use MEGA 12.1 HTML Gonnet as the default matrix")
	}
	if !hasDefaultParam(defs["clustalw_protein"], "residue_specific_penalties", "ON") {
		t.Fatalf("protein ClustalW should default residue-specific penalties to ON")
	}
	if !hasDefaultParam(defs["clustalw_protein"], "hydrophilic_penalties", "ON") {
		t.Fatalf("protein ClustalW should default hydrophilic penalties to ON")
	}
	if !hasDefaultParam(defs["clustalw_protein"], "end_gap_separation", "OFF") {
		t.Fatalf("protein ClustalW should default end gap separation to OFF")
	}
	if !hasDefaultParam(defs["clustalw_protein"], "use_negative_matrix", "OFF") {
		t.Fatalf("protein ClustalW should default use negative matrix to OFF")
	}
	if !hasDefaultParam(defs["clustalw_protein"], "keep_predefined_gaps", "False") {
		t.Fatalf("protein ClustalW should default keep predefined gaps to MEGA 12.1 HTML unchecked")
	}
	if !hasDefaultParam(defs["clustalw_codons"], "protein_weight_matrix", "BLOSUM") {
		t.Fatalf("codon ClustalW should use MEGA 12.1 HTML BLOSUM as the default matrix")
	}
	if !hasDefaultParam(defs["clustalw_codons"], "end_gap_separation", "ON") {
		t.Fatalf("codon ClustalW should default end gap separation to MEGA 12.1 HTML ON")
	}
	if !hasDefaultParam(defs["clustalw_codons"], "use_negative_matrix", "ON") {
		t.Fatalf("codon ClustalW should default use negative matrix to MEGA 12.1 HTML ON")
	}
	if !hasDefaultParam(defs["clustalw"], "use_negative_matrix", "OFF") {
		t.Fatalf("nucleotide ClustalW should default use negative matrix to MEGA 12.1 HTML OFF")
	}
	if !hasDefaultParam(defs["clustalw"], "keep_predefined_gaps", "False") {
		t.Fatalf("nucleotide ClustalW should default keep predefined gaps to MEGA 12.1 HTML unchecked")
	}
	if !hasDefaultParam(defs["muscle_protein"], "gap_open", "-2.90") {
		t.Fatalf("protein MUSCLE should default Gap Open to MEGA runtime/screenshot value -2.90")
	}
	if !hasDefaultParam(defs["muscle"], "gap_open", "-400.00") {
		t.Fatalf("nucleotide MUSCLE should default Gap Open to MEGA runtime/screenshot value -400.00")
	}
	if !hasDefaultParam(defs["muscle"], "max_iterations", "16") {
		t.Fatalf("nucleotide MUSCLE should default Max Iterations to 16")
	}
	if hasParam(defs["muscle"], "hydrophobicity_multiplier") {
		t.Fatalf("nucleotide MUSCLE should show Hydrophobicity Multiplier as not applicable, not an editable runtime parameter")
	}
	if !hasDefaultParam(defs["muscle_protein"], "hydrophobicity_multiplier", "1.20") {
		t.Fatalf("protein MUSCLE should expose Hydrophobicity Multiplier default 1.20")
	}
	if !hasDefaultParam(defs["muscle_codons"], "genetic_code", "Standard") {
		t.Fatalf("codon MUSCLE should expose the genetic code selector used by runtime codon translation")
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

func TestNormalizeTreeSettingsForKindFallsBackToDefaultTreeMethod(t *testing.T) {
	settings := NormalizeTreeSettingsForKind(TreeSettings{TreeMethod: TreeMethod("stale")}, SequenceProtein)
	if settings.TreeMethod != DefaultTreeMethod {
		t.Fatalf("stale protein tree method normalized to %q, want %q", settings.TreeMethod, DefaultTreeMethod)
	}
	settings = NormalizeTreeSettingsForKind(TreeSettings{TreeMethod: TreeMethod("stale")}, SequenceNucleotide)
	if settings.TreeMethod != DefaultTreeMethod {
		t.Fatalf("stale nucleotide tree method normalized to %q, want %q", settings.TreeMethod, DefaultTreeMethod)
	}
}

func TestNormalizeTreeSettingsDropsParametersOutsideCurrentMethodDefinition(t *testing.T) {
	settings := NormalizeTreeSettingsForKind(TreeSettings{
		AlignmentMethod: AlignmentMUSCLE,
		AlignmentParams: map[string]string{
			"gap_open":                      "-350.00",
			"cluster_method":                "UPGMA",
			"cluster_method_iterations_1_2": "Bad option",
			"unknown_runtime":               "kept by old snapshots",
		},
		TreeMethod: TreeNeighborJoining,
		TreeParams: map[string]string{
			"phylogeny_test": "Not a MEGA option",
			"model_method":   "Maximum Composite Likelihood",
			"extra":          "stale",
		},
	}, SequenceNucleotide)
	if settings.AlignmentParams["gap_open"] != "-350.00" {
		t.Fatalf("valid MUSCLE DNA params should be preserved, got %#v", settings.AlignmentParams)
	}
	if _, ok := settings.AlignmentParams["cluster_method"]; ok {
		t.Fatalf("stale MUSCLE params should be dropped, got %#v", settings.AlignmentParams)
	}
	if settings.AlignmentParams["cluster_method_iterations_1_2"] != "UPGMA" {
		t.Fatalf("invalid MUSCLE picklist values should fall back to MEGA default, got %#v", settings.AlignmentParams)
	}
	if _, ok := settings.TreeParams["extra"]; ok {
		t.Fatalf("tree params should drop stale keys outside the current method definition: %#v", settings.TreeParams)
	}
	if settings.TreeParams["phylogeny_test"] != "None" {
		t.Fatalf("invalid picklist values should fall back to MEGA default, got %#v", settings.TreeParams)
	}
	if settings.TreeParams["model_method"] != "Maximum Composite Likelihood" {
		t.Fatalf("valid current tree params should be preserved, got %#v", settings.TreeParams)
	}
}

func hasParam(def MethodDefinition, id string) bool {
	for _, param := range def.Parameters {
		if param.ID == id {
			return true
		}
	}
	return false
}

func TestNormalizeTreeSettingsPreservesSkipUnselectFalse(t *testing.T) {
	settings := DefaultTreeSettings()
	settings.ConversionSkipUnselect = false
	settings = NormalizeTreeSettings(settings)
	if settings.ConversionSkipUnselect {
		t.Fatalf("NormalizeTreeSettings should preserve explicit skip-unselect=false")
	}

	zero := NormalizeTreeSettings(TreeSettings{})
	if !zero.ConversionSkipUnselect {
		t.Fatalf("zero-value tree settings should still default skip-unselect to true")
	}
}

func TestTreeDefinitionsUseMegaTreeDefaults(t *testing.T) {
	for _, kind := range []SequenceKind{SequenceProtein, SequenceNucleotide} {
		t.Run(string(kind), func(t *testing.T) {
			defs := TreeDefinitionsForKind(kind)
			byID := map[string]MethodDefinition{}
			for _, def := range defs {
				byID[def.ID] = def
			}
			for _, id := range []string{
				string(TreeMaximumLikelihood),
				string(TreeNeighborJoining),
				string(TreeMinimumEvolution),
				string(TreeUPGMA),
				string(TreeMaximumParsimony),
			} {
				if byID[id].ID == "" {
					t.Fatalf("missing MEGA tree definition %q for %s", id, kind)
				}
			}
			if !hasDefaultParam(byID[string(TreeNeighborJoining)], "gaps_missing_data", "Pairwise deletion") {
				t.Fatalf("distance trees should default Gaps/Missing Data to MEGA Pairwise deletion")
			}
			if !hasOptionParam(byID[string(TreeNeighborJoining)], "phylogeny_test", "Bootstrap method") {
				t.Fatalf("distance trees should expose MEGA Bootstrap method")
			}
			if !hasDefaultParam(byID[string(TreeNeighborJoining)], "bootstrap_replicates", "500") {
				t.Fatalf("distance tree bootstrap replicate default should be MEGA 500")
			}
			if !hasDefaultParam(byID[string(TreeMaximumLikelihood)], "gaps_missing_data", "Use all sites") {
				t.Fatalf("ML trees should default Gaps/Missing Data to MEGA Use all sites")
			}
			if !hasDefaultParam(byID[string(TreeMaximumParsimony)], "gaps_missing_data", "Use all sites") {
				t.Fatalf("MP trees should default Gaps/Missing Data to MEGA Use all sites")
			}
			if !hasDefaultParam(byID[string(TreeMaximumLikelihood)], "bootstrap_replicates", "500") {
				t.Fatalf("ML bootstrap replicate default should be MEGA 500")
			}
			if !hasDefaultParam(byID[string(TreeMaximumLikelihood)], "site_coverage_cutoff", "95") {
				t.Fatalf("ML partial deletion site coverage default should be MEGA 95")
			}
			if !hasDefaultParam(byID[string(TreeMaximumLikelihood)], "discrete_gamma_categories", "5") {
				t.Fatalf("ML gamma categories default should be MEGA 5")
			}
			if !hasDefaultParam(byID[string(TreeMaximumLikelihood)], "branch_swap_filter", "None") {
				t.Fatalf("ML branch swap filter should default to MEGA None")
			}
			if !hasReadOnlyParam(byID[string(TreeNeighborJoining)], "statistical_method") {
				t.Fatalf("MEGA fixed Statistical Method row should be read-only")
			}
			if !hasReadOnlyParam(byID[string(TreeNeighborJoining)], "substitutions_type") {
				t.Fatalf("MEGA fixed Substitutions Type row should be read-only")
			}
			if !hasReadOnlyParam(byID[string(TreeMinimumEvolution)], "me_heuristic_method") {
				t.Fatalf("MEGA fixed ME heuristic row should be read-only")
			}
			if !hasOptionParam(byID[string(TreeMaximumParsimony)], "mp_search_method", "Min-Mini Heuristic") {
				t.Fatalf("MP search method should expose MEGA Min-Mini Heuristic option")
			}
			if !hasOptionParam(byID[string(TreeMaximumParsimony)], "mp_search_method", "Max-mini Branch-&-bound") {
				t.Fatalf("MP search method should expose MEGA Max-mini Branch-&-bound option")
			}
		})
	}
}

func TestTreeParameterDirectnessMatrixCoversEveryTreeParameter(t *testing.T) {
	consumedByRuntime := map[string]bool{
		"phylogeny_test":                true,
		"bootstrap_replicates":          true,
		"gaps_missing_data":             true,
		"site_coverage_cutoff":          true,
		"number_of_threads":             true,
		"model_method":                  true,
		"rates_among_sites":             true,
		"gamma_parameter":               true,
		"pattern_among_lineages":        true,
		"discrete_gamma_categories":     true,
		"ml_heuristic_method":           true,
		"initial_tree_for_ml":           true,
		"number_of_initial_trees":       true,
		"initial_tree_file":             true,
		"branch_swap_filter":            true,
		"mp_search_method":              true,
		"initial_trees_random_addition": true,
		"mp_search_level":               true,
		"max_trees_to_retain":           true,
	}
	readOnlyDisplayOnly := map[string]bool{
		"scope":              true,
		"statistical_method": true,
		"substitutions_type": true,
	}
	readOnlyFixedMegaRows := map[string]bool{
		"me_heuristic_method": true,
		"initial_tree_for_me": true,
	}
	megaRecordedNoDownstreamRead := map[string]bool{
		"me_search_level": true,
	}

	var missing []string
	seen := map[string]bool{}
	for _, def := range TreeDefinitions() {
		for _, param := range def.Parameters {
			id := strings.TrimSpace(param.ID)
			if id == "" || param.Kind == ParameterSection {
				continue
			}
			seen[id] = true
			if consumedByRuntime[id] || readOnlyDisplayOnly[id] || readOnlyFixedMegaRows[id] || megaRecordedNoDownstreamRead[id] {
				continue
			}
			missing = append(missing, def.ID+"."+id)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("tree parameters missing MEGA directness classification: %s", strings.Join(missing, ", "))
	}
	if seen["distance_model"] {
		t.Fatalf("distance_model must remain runtime compatibility-only and must not be exposed in the MEGA 12.1 tree UI registry")
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

func hasReadOnlyParam(def MethodDefinition, id string) bool {
	for _, param := range def.Parameters {
		if param.ID == id && param.ReadOnly {
			return true
		}
	}
	return false
}

func hasOptionParam(def MethodDefinition, id string, want string) bool {
	for _, param := range def.Parameters {
		if param.ID != id {
			continue
		}
		for _, option := range param.Options {
			if option == want {
				return true
			}
		}
	}
	return false
}
