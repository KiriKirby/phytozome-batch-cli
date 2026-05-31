package phylo

import "strings"

func AlignmentDefinitions() []MethodDefinition {
	return []MethodDefinition{
		alignmentClustalWDNA(),
		alignmentClustalWProtein(),
		alignmentClustalWCodons(),
		alignmentMUSCLEDNA(),
		alignmentMUSCLEProtein(),
		alignmentMUSCLECodons(),
	}
}

func TreeDefinitions() []MethodDefinition {
	return []MethodDefinition{
		{
			ID:            string(TreeNeighborJoining),
			Label:         "Neighbor-Joining",
			RuntimeMethod: "neighbor_joining",
			SequenceKinds: []SequenceKind{SequenceProtein, SequenceNucleotide},
			Parameters: []ParameterDefinition{
				stringParam("scope", "Scope", "All Selected Taxa"),
				pickParam("statistical_method", "Statistical Method", "Neighbor-joining", "Neighbor-joining"),
				pickParam("distance_model", "Model/Method", "p-distance", "p-distance", "No. of differences", "Jukes-Cantor model", "Kimura 2-parameter model"),
				pickParam("gaps_missing_treatment", "Gaps/Missing Data Treatment", "Pairwise deletion", "Pairwise deletion", "Complete deletion", "Partial deletion"),
				intParam("site_coverage_cutoff_percent", "Site Coverage Cutoff (%)", "95", "0", "100", "1"),
				pickParam("phylogeny_test", "Test of Phylogeny", "None", "None", "Bootstrap method"),
				intParam("bootstrap_replications", "No. of Bootstrap Replications", "500", "1", "100000", "100"),
			},
		},
	}
}

func MethodDefinitionForAlignment(method AlignmentMethod) (MethodDefinition, bool) {
	for _, def := range AlignmentDefinitions() {
		if def.ID == string(method) {
			return def, true
		}
	}
	return MethodDefinition{}, false
}

func MethodDefinitionForAlignmentKind(method AlignmentMethod, kind SequenceKind) (MethodDefinition, bool) {
	id := strings.TrimSpace(string(method))
	if id == "" {
		return MethodDefinition{}, false
	}
	if def, ok := MethodDefinitionForAlignment(method); ok && MethodSupportsKind(def, kind) {
		return def, true
	}
	if kind == SequenceProtein && !strings.Contains(strings.ToLower(id), "codons") && !strings.HasSuffix(strings.ToLower(id), "_protein") {
		if def, ok := MethodDefinitionForAlignment(AlignmentMethod(id + "_protein")); ok && MethodSupportsKind(def, kind) {
			return def, true
		}
	}
	if kind == SequenceNucleotide && strings.HasSuffix(strings.ToLower(id), "_protein") {
		baseID := strings.TrimSuffix(id, "_protein")
		if def, ok := MethodDefinitionForAlignment(AlignmentMethod(baseID)); ok && MethodSupportsKind(def, kind) {
			return def, true
		}
	}
	return MethodDefinition{}, false
}

func MethodDefinitionForTree(method TreeMethod) (MethodDefinition, bool) {
	for _, def := range TreeDefinitions() {
		if def.ID == string(method) {
			return def, true
		}
	}
	return MethodDefinition{}, false
}

func AlignmentDefinitionsForKind(kind SequenceKind) []MethodDefinition {
	return methodDefinitionsForKind(AlignmentDefinitions(), kind)
}

func TreeDefinitionsForKind(kind SequenceKind) []MethodDefinition {
	return methodDefinitionsForKind(TreeDefinitions(), kind)
}

func methodDefinitionsForKind(defs []MethodDefinition, kind SequenceKind) []MethodDefinition {
	out := make([]MethodDefinition, 0, len(defs))
	for _, def := range defs {
		if MethodSupportsKind(def, kind) {
			out = append(out, def)
		}
	}
	return out
}

func DefaultParams(def MethodDefinition) map[string]string {
	out := make(map[string]string)
	for _, param := range def.Parameters {
		if param.Kind == ParameterSection || !param.Applicable || param.ReadOnly {
			continue
		}
		if strings.TrimSpace(param.Default) != "" {
			out[param.ID] = param.Default
		}
	}
	return out
}

func NormalizeTreeSettings(settings TreeSettings) TreeSettings {
	if strings.TrimSpace(settings.DisplayNameSource) == "" {
		settings.DisplayNameSource = DefaultDisplayNameSource
	}
	hadConversionState := strings.TrimSpace(string(settings.ConversionTarget)) != "" ||
		strings.TrimSpace(string(settings.ConversionAction)) != "" ||
		settings.ConversionSkipUnselect
	switch strings.ToLower(strings.TrimSpace(string(settings.ConversionTarget))) {
	case "dna", "nucleotide":
		settings.ConversionTarget = ConversionTargetDNA
	case "protein", "aa", "amino_acid", "amino-acid":
		settings.ConversionTarget = ConversionTargetProtein
	default:
		settings.ConversionTarget = DefaultConversionTarget
	}
	switch strings.ToLower(strings.TrimSpace(string(settings.ConversionAction))) {
	case "skip":
		settings.ConversionAction = ConversionActionSkip
	case "convert", "":
		settings.ConversionAction = ConversionActionConvert
	default:
		settings.ConversionAction = DefaultConversionAction
	}
	if !hadConversionState {
		settings.ConversionSkipUnselect = true
	}
	if strings.TrimSpace(string(settings.AlignmentMethod)) == "" {
		settings.AlignmentMethod = DefaultAlignmentMethod
	}
	if strings.TrimSpace(string(settings.TreeMethod)) == "" {
		settings.TreeMethod = DefaultTreeMethod
	}
	if alignDef, ok := MethodDefinitionForAlignment(settings.AlignmentMethod); ok {
		settings.AlignmentParams = mergeDefaultParams(alignDef, settings.AlignmentParams)
	} else if settings.AlignmentParams == nil {
		settings.AlignmentParams = map[string]string{}
	}
	if treeDef, ok := MethodDefinitionForTree(settings.TreeMethod); ok {
		settings.TreeParams = mergeDefaultParams(treeDef, settings.TreeParams)
	} else if settings.TreeParams == nil {
		settings.TreeParams = map[string]string{}
	}
	return settings
}

func NormalizeTreeSettingsForKind(settings TreeSettings, kind SequenceKind) TreeSettings {
	hadAlignmentParams := len(settings.AlignmentParams) > 0
	settings = NormalizeTreeSettings(settings)
	if alignDef, ok := MethodDefinitionForAlignmentKind(settings.AlignmentMethod, kind); ok {
		previousMethod := settings.AlignmentMethod
		settings.AlignmentMethod = AlignmentMethod(strings.TrimSpace(alignDef.ID))
		if !hadAlignmentParams && !strings.EqualFold(strings.TrimSpace(string(previousMethod)), strings.TrimSpace(alignDef.ID)) {
			settings.AlignmentParams = DefaultParams(alignDef)
		} else {
			settings.AlignmentParams = mergeDefaultParams(alignDef, settings.AlignmentParams)
		}
		return settings
	}
	defs := AlignmentDefinitionsForKind(kind)
	if len(defs) > 0 {
		settings.AlignmentMethod = AlignmentMethod(strings.TrimSpace(defs[0].ID))
		if !hadAlignmentParams {
			settings.AlignmentParams = DefaultParams(defs[0])
		} else {
			settings.AlignmentParams = mergeDefaultParams(defs[0], settings.AlignmentParams)
		}
	}
	return settings
}

func mergeDefaultParams(def MethodDefinition, values map[string]string) map[string]string {
	out := DefaultParams(def)
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(value)
	}
	return out
}

func MethodSupportsKind(def MethodDefinition, kind SequenceKind) bool {
	if kind == SequenceUnknown {
		return !def.CodingRequired
	}
	for _, candidate := range def.SequenceKinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

func alignmentClustalWDNA() MethodDefinition {
	return MethodDefinition{
		ID:            string(AlignmentClustalW),
		Label:         "ClustalW (DNA)",
		RuntimeMethod: "clustalw",
		SequenceKinds: []SequenceKind{SequenceNucleotide},
		Parameters: []ParameterDefinition{
			sectionParam("Pairwise Alignment"),
			floatParam("pairwise_gap_opening_penalty", "Gap Openening Penalty", "15.00", "0", "100", "0.1", 2),
			floatParam("pairwise_gap_extension_penalty", "Gap Extension Penalty", "6.66", "0", "100", "0.1", 2),
			sectionParam("Multiple Alignment"),
			floatParam("multiple_gap_opening_penalty", "Gap Openening Penalty", "15.00", "0", "100", "0.1", 2),
			floatParam("multiple_gap_extension_penalty", "Gap Extension Penalty", "6.66", "0", "100", "0.1", 2),
			sectionParam("Global Options"),
			pickParam("dna_weight_matrix", "DNA Weight Matrix", "IUB", "IUB", "ClustalW (1.6)"),
			floatParam("transition_weight", "Transition Weight", "0.50", "0", "1", "0.1", 2),
			pickParam("use_negative_matrix", "Use Negative Matrix", "OFF", "ON", "OFF"),
			intParam("delay_divergent_cutoff", "Delay Divergence Cutoff(%)", "30", "0", "100", "1"),
			pickParam("keep_predefined_gaps", "Keep Predefined Gaps", "False", "True", "False"),
		},
	}
}

func alignmentClustalWProtein() MethodDefinition {
	return MethodDefinition{
		ID:            string(AlignmentClustalW) + "_protein",
		Label:         "ClustalW",
		RuntimeMethod: "clustalw",
		SequenceKinds: []SequenceKind{SequenceProtein},
		Parameters: []ParameterDefinition{
			sectionParam("Pairwise Alignment"),
			floatParam("pairwise_gap_opening_penalty", "Gap Openening Penalty", "10.00", "0", "100", "0.1", 2),
			floatParam("pairwise_gap_extension_penalty", "Gap Extension Penalty", "0.20", "0", "100", "0.1", 2),
			sectionParam("Multiple Alignment"),
			floatParam("multiple_gap_opening_penalty", "Gap Openening Penalty", "10.00", "0", "100", "0.1", 2),
			floatParam("multiple_gap_extension_penalty", "Gap Extension Penalty", "0.20", "0", "100", "0.1", 2),
			sectionParam("Global Options"),
			pickParam("protein_weight_matrix", "Protein Weight Matrix", "BLOSUM", "BLOSUM", "PAM", "Gonnet", "Identity"),
			pickParam("residue_specific_penalties", "Residue-specific Penalties", "OFF", "ON", "OFF"),
			pickParam("hydrophilic_penalties", "Hydrophilic Penalties", "OFF", "ON", "OFF"),
			intParam("gap_separation_distance", "Gap Separation Distance", "4", "0", "100", "1"),
			pickParam("end_gap_separation", "End Gap Separation", "OFF", "OFF", "ON"),
			pickParam("use_negative_matrix", "Use Negative Matrix", "OFF", "OFF", "ON"),
			intParam("delay_divergent_cutoff", "Delay Divergence Cutoff(%)", "30", "0", "100", "1"),
			pickParam("keep_predefined_gaps", "Keep Predefined Gaps", "False", "True", "False"),
		},
	}
}

func alignmentClustalWCodons() MethodDefinition {
	def := alignmentClustalWProtein()
	def.ID = string(AlignmentClustalWCodons)
	def.Label = "ClustalW (Codons)"
	def.RuntimeMethod = string(AlignmentClustalWCodons)
	def.SequenceKinds = []SequenceKind{SequenceNucleotide}
	def.CodingRequired = true
	def.Parameters = append(def.Parameters[:12], append([]ParameterDefinition{
		pickParam("genetic_code", "Genetic Code Table", "Standard", geneticCodeOptions()...),
	}, def.Parameters[12:]...)...)
	return def
}

func alignmentMUSCLEDNA() MethodDefinition {
	return MethodDefinition{
		ID:            string(AlignmentMUSCLE),
		Label:         "MUSCLE (DNA)",
		RuntimeMethod: "muscle",
		SequenceKinds: []SequenceKind{SequenceNucleotide},
		Parameters: []ParameterDefinition{
			sectionParam("Gap Penalties"),
			floatParam("gap_open", "Gap Open", "-400.00", "-2147483648", "0", "0.1", 2),
			floatParam("gap_extend", "Gap Extend", "0.00", "-2147483648", "0", "0.1", 2),
			sectionParam("Memory/Iterations"),
			intParam("max_memory_mb", "Max Memory in MB", "0", "0", "2147483647", "256"),
			intParam("max_iterations", "Max Iterations", "8", "1", "2147483647", "1"),
			sectionParam("Advanced Options"),
			pickParam("cluster_method_iterations_1_2", "Cluster Method (Iterations 1,2)", "UPGMB", "UPGMA", "UPGMB", "Neighbor Joining"),
			pickParam("cluster_method_other_iterations", "Cluster Method (Other Iterations)", "UPGMB", "UPGMA", "UPGMB", "Neighbor Joining"),
			intParam("min_diag_length_lambda", "Min Diag Length (Lambda)", "24", "0", "2147483647", "1"),
		},
	}
}

func alignmentMUSCLEProtein() MethodDefinition {
	return MethodDefinition{
		ID:            string(AlignmentMUSCLE) + "_protein",
		Label:         "MUSCLE",
		RuntimeMethod: "muscle",
		SequenceKinds: []SequenceKind{SequenceProtein},
		Parameters: []ParameterDefinition{
			sectionParam("Gap Penalties"),
			floatParam("gap_open", "Gap Open", "-2.90", "-2147483648", "0", "0.1", 2),
			floatParam("gap_extend", "Gap Extend", "0.00", "-2147483648", "0", "0.1", 2),
			floatParam("hydrophobicity_multiplier", "Hydrophobicity Multiplier", "1.20", "0", "2147483647", "0.1", 2),
			sectionParam("Memory/Iterations"),
			intParam("max_memory_mb", "Max Memory in MB", "0", "0", "2147483647", "256"),
			intParam("max_iterations", "Max Iterations", "8", "1", "2147483647", "1"),
			sectionParam("Advanced Options"),
			pickParam("cluster_method_iterations_1_2", "Cluster Method (Iterations 1,2)", "UPGMB", "UPGMA", "UPGMB", "Neighbor Joining"),
			pickParam("cluster_method_other_iterations", "Cluster Method (Other Iterations)", "UPGMB", "UPGMA", "UPGMB", "Neighbor Joining"),
			intParam("min_diag_length_lambda", "Min Diag Length (Lambda)", "24", "0", "2147483647", "1"),
		},
	}
}

func alignmentMUSCLECodons() MethodDefinition {
	def := alignmentMUSCLEProtein()
	def.ID = string(AlignmentMUSCLECodons)
	def.Label = "MUSCLE (Codons)"
	def.RuntimeMethod = string(AlignmentMUSCLECodons)
	def.SequenceKinds = []SequenceKind{SequenceNucleotide}
	def.CodingRequired = true
	return def
}

func sectionParam(label string) ParameterDefinition {
	return ParameterDefinition{ID: label, Label: label, Kind: ParameterSection, ReadOnly: true, Applicable: true}
}

func intParam(id, label, def, min, max, inc string) ParameterDefinition {
	return ParameterDefinition{ID: id, Label: label, Kind: ParameterInteger, Default: def, Applicable: true, Min: min, Max: max, Increment: inc}
}

func floatParam(id, label, def, min, max, inc string, precision int) ParameterDefinition {
	return ParameterDefinition{ID: id, Label: label, Kind: ParameterFloat, Default: def, Applicable: true, Min: min, Max: max, Increment: inc, Precision: precision}
}

func pickParam(id, label, def string, options ...string) ParameterDefinition {
	if len(options) == 0 {
		options = []string{def}
	}
	return ParameterDefinition{ID: id, Label: label, Kind: ParameterPicklist, Default: def, Options: append([]string(nil), options...), Applicable: true}
}

func stringParam(id, label, def string) ParameterDefinition {
	return ParameterDefinition{ID: id, Label: label, Kind: ParameterString, Default: def, Applicable: true}
}

func geneticCodeOptions() []string {
	return []string{
		"Standard",
		"Vertebrate Mitochondrial",
		"Invertebrate Mitochondrial",
		"Yeast Mitochondrial",
		"Mold Mitochondrial",
		"Protozoan Mitochondrial",
		"Coelenterate Mitochondrial",
		"Mycoplasma",
		"Spiroplasma",
		"Ciliate Nuclear",
		"Dasycladacean Nuclear",
		"Hexamita Nuclear",
		"Echinoderm Mitochondrial",
		"Euplotid Nuclear",
		"Bacterial Plastid",
		"Plant Plastid",
		"Alternative Yeast Nuclear",
		"Ascidian Mitochondrial",
		"Flatworm Mitochondrial",
		"Blepharisma Macronuclear",
		"Chlorophycean Mitochondrial",
		"Trematode Mitochondrial",
		"Scenedesmus obliquus Mitochondrial",
		"Thraustochytrium Mitochondrial",
	}
}
