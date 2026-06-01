package phylo

import (
	"runtime"
	"strconv"
	"strings"
)

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
		treeMaximumLikelihoodProtein(),
		treeNeighborJoiningProtein(),
		treeMinimumEvolutionProtein(),
		treeUPGMAProtein(),
		treeMaximumParsimonyProtein(),
		treeMaximumLikelihoodDNA(),
		treeNeighborJoiningDNA(),
		treeMinimumEvolutionDNA(),
		treeUPGMADNA(),
		treeMaximumParsimonyDNA(),
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

func MethodDefinitionForTreeKind(method TreeMethod, kind SequenceKind) (MethodDefinition, bool) {
	for _, def := range TreeDefinitions() {
		if def.ID == string(method) && MethodSupportsKind(def, kind) {
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

func DefaultParamsForAlignment(method AlignmentMethod) map[string]string {
	if def, ok := MethodDefinitionForAlignment(method); ok {
		return DefaultParams(def)
	}
	return map[string]string{}
}

func DefaultParamsForTree(method TreeMethod) map[string]string {
	if def, ok := MethodDefinitionForTree(method); ok {
		return DefaultParams(def)
	}
	return map[string]string{}
}

func NormalizeTreeSettings(settings TreeSettings) TreeSettings {
	settingsWasEmpty := strings.TrimSpace(settings.DisplayNameSource) == "" &&
		strings.TrimSpace(string(settings.ConversionTarget)) == "" &&
		!settings.ConversionSkipUnselect &&
		strings.TrimSpace(string(settings.AlignmentMethod)) == "" &&
		strings.TrimSpace(string(settings.TreeMethod)) == "" &&
		len(settings.AlignmentParams) == 0 &&
		len(settings.TreeParams) == 0
	if strings.TrimSpace(settings.DisplayNameSource) == "" {
		settings.DisplayNameSource = DefaultDisplayNameSource
	}
	switch strings.ToLower(strings.TrimSpace(string(settings.ConversionTarget))) {
	case "dna", "nucleotide":
		settings.ConversionTarget = ConversionTargetDNA
	case "protein", "aa", "amino_acid", "amino-acid":
		settings.ConversionTarget = ConversionTargetProtein
	default:
		settings.ConversionTarget = DefaultConversionTarget
	}
	if settingsWasEmpty {
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
	hadTreeParams := len(settings.TreeParams) > 0
	settings = NormalizeTreeSettings(settings)
	if alignDef, ok := MethodDefinitionForAlignmentKind(settings.AlignmentMethod, kind); ok {
		previousMethod := settings.AlignmentMethod
		settings.AlignmentMethod = AlignmentMethod(strings.TrimSpace(alignDef.ID))
		if !strings.EqualFold(strings.TrimSpace(string(previousMethod)), strings.TrimSpace(alignDef.ID)) {
			settings.AlignmentParams = DefaultParams(alignDef)
		} else {
			settings.AlignmentParams = mergeDefaultParams(alignDef, settings.AlignmentParams)
		}
	} else if defs := AlignmentDefinitionsForKind(kind); len(defs) > 0 {
		previousMethod := settings.AlignmentMethod
		settings.AlignmentMethod = AlignmentMethod(strings.TrimSpace(defs[0].ID))
		if !hadAlignmentParams || !strings.EqualFold(strings.TrimSpace(string(previousMethod)), strings.TrimSpace(defs[0].ID)) {
			settings.AlignmentParams = DefaultParams(defs[0])
		} else {
			settings.AlignmentParams = mergeDefaultParams(defs[0], settings.AlignmentParams)
		}
	}
	if treeDef, ok := MethodDefinitionForTreeKind(settings.TreeMethod, kind); ok {
		previousMethod := settings.TreeMethod
		settings.TreeMethod = TreeMethod(strings.TrimSpace(treeDef.ID))
		if !strings.EqualFold(strings.TrimSpace(string(previousMethod)), strings.TrimSpace(treeDef.ID)) || !paramsMatchDefinition(treeDef, settings.TreeParams) {
			settings.TreeParams = DefaultParams(treeDef)
		} else {
			settings.TreeParams = mergeDefaultParams(treeDef, settings.TreeParams)
		}
	} else if defs := TreeDefinitionsForKind(kind); len(defs) > 0 {
		previousMethod := settings.TreeMethod
		fallback := defaultTreeDefinitionForKind(kind, defs)
		settings.TreeMethod = TreeMethod(strings.TrimSpace(fallback.ID))
		if !hadTreeParams || !strings.EqualFold(strings.TrimSpace(string(previousMethod)), strings.TrimSpace(fallback.ID)) {
			settings.TreeParams = DefaultParams(fallback)
		} else {
			settings.TreeParams = mergeDefaultParams(fallback, settings.TreeParams)
		}
	}
	return settings
}

func defaultTreeDefinitionForKind(kind SequenceKind, defs []MethodDefinition) MethodDefinition {
	for _, def := range defs {
		if def.ID == string(DefaultTreeMethod) && MethodSupportsKind(def, kind) {
			return def
		}
	}
	return defs[0]
}

func treeMaximumLikelihoodProtein() MethodDefinition {
	return MethodDefinition{
		ID:            string(TreeMaximumLikelihood),
		Label:         "Maximum Likelihood",
		RuntimeMethod: "maximum_likelihood",
		SequenceKinds: []SequenceKind{SequenceProtein},
		Parameters: []ParameterDefinition{
			sectionParam("Analysis"),
			readOnlyPickParam("statistical_method", "Statistical Method", "Maximum Likelihood", "Maximum Likelihood"),
			sectionParam("Phylogeny Test"),
			pickParam("phylogeny_test", "Test of Phylogeny", "None", "None", "Bootstrap method"),
			intParam("bootstrap_replicates", "Bootstrap Replicates", "500", "1", "10000", "100"),
			sectionParam("Substitution Model"),
			readOnlyPickParam("substitutions_type", "Substitutions Type", "Amino acid", "Amino acid"),
			pickParam("model_method", "Model/Method", "Jones-Taylor-Thornton (JTT) model", megaMLAminoModels()...),
			sectionParam("Rates and Patterns"),
			pickParam("rates_among_sites", "Rates among Sites", "Uniform Rates", megaMLRatesAmongSites()...),
			intParam("discrete_gamma_categories", "No of Discrete Gamma Categories", "5", "2", "16", "1"),
			sectionParam("Data Subset To Use"),
			pickParam("gaps_missing_data", "Gaps/Missing Data", "Use all sites", "Use all sites", "Partial deletion", "Complete deletion"),
			intParam("site_coverage_cutoff", "Site Coverage Cutoff (%)", "95", "5", "100", "5"),
			sectionParam("Tree Inference Options"),
			pickParam("ml_heuristic_method", "ML Heuristic Method", "Nearest-Neighbor-Interchange (NNI)", megaMLSearchMethods()...),
			pickParam("initial_tree_for_ml", "Initial Tree for ML", "Make initial tree automatically (Default - NJ/MP)", megaMLInitialTrees()...),
			intParam("number_of_initial_trees", "No. of Initial Trees", "10", "1", "100000", "1"),
			stringParam("initial_tree_file", "Initial Tree File", ""),
			pickParam("branch_swap_filter", "Branch Swap Filter", "None", megaMLBranchSwapFilters()...),
			sectionParam("System Resource Usage"),
			intParam("number_of_threads", "Number of Threads", defaultMegaThreadCountString(), "1", strconv.Itoa(maxIntRegistry(1, runtime.NumCPU())), "1"),
		},
	}
}

func treeNeighborJoiningProtein() MethodDefinition {
	return MethodDefinition{
		ID:            string(TreeNeighborJoining),
		Label:         "Neighbor-Joining",
		RuntimeMethod: "neighbor_joining",
		SequenceKinds: []SequenceKind{SequenceProtein},
		Parameters:    distanceTreeProteinParams("Neighbor-joining", "Pairwise deletion", nil),
	}
}

func treeMinimumEvolutionProtein() MethodDefinition {
	extra := []ParameterDefinition{
		sectionParam("Tree Inference Options"),
		readOnlyPickParam("me_heuristic_method", "ME Heuristic Method", "Close-Neighbor-Interchange (CNI)", "Close-Neighbor-Interchange (CNI)"),
		readOnlyPickParam("initial_tree_for_me", "Initial Tree for ME", "Obtain initial tree by Neighbor-joining", "Obtain initial tree by Neighbor-joining"),
		intParam("me_search_level", "ME Search Level", "1", "1", "2", "1"),
	}
	return MethodDefinition{
		ID:            string(TreeMinimumEvolution),
		Label:         "Minimum Evolution",
		RuntimeMethod: "minimum_evolution",
		SequenceKinds: []SequenceKind{SequenceProtein},
		Parameters:    distanceTreeProteinParams("Minimum Evolution method", "Pairwise deletion", extra),
	}
}

func treeUPGMAProtein() MethodDefinition {
	return MethodDefinition{
		ID:            string(TreeUPGMA),
		Label:         "UPGMA",
		RuntimeMethod: "upgma",
		SequenceKinds: []SequenceKind{SequenceProtein},
		Parameters:    distanceTreeProteinParams("UPGMA", "Pairwise deletion", nil),
	}
}

func treeMaximumParsimonyProtein() MethodDefinition {
	return MethodDefinition{
		ID:            string(TreeMaximumParsimony),
		Label:         "Maximum Parsimony",
		RuntimeMethod: "maximum_parsimony",
		SequenceKinds: []SequenceKind{SequenceProtein},
		Parameters: []ParameterDefinition{
			sectionParam("Analysis"),
			readOnlyPickParam("statistical_method", "Statistical Method", "Maximum Parsimony", "Maximum Parsimony"),
			sectionParam("Phylogeny Test"),
			pickParam("phylogeny_test", "Test of Phylogeny", "None", "None", "Bootstrap method"),
			intParam("bootstrap_replicates", "Bootstrap Replicates", "500", "1", "10000", "100"),
			sectionParam("Substitution Model"),
			readOnlyPickParam("substitutions_type", "Substitutions Type", "Amino acid", "Amino acid"),
			sectionParam("Data Subset To Use"),
			pickParam("gaps_missing_data", "Gaps/Missing Data", "Use all sites", "Use all sites", "Partial deletion", "Complete deletion"),
			intParam("site_coverage_cutoff", "Site Coverage Cutoff (%)", "95", "5", "100", "5"),
			sectionParam("Tree Inference Options"),
			pickParam("mp_search_method", "MP Search Method", "Subtree-Pruning-Regrafting (SPR)", "Subtree-Pruning-Regrafting (SPR)", "Tree-Bisection-Reconnection (TBR)", "Min-Mini Heuristic", "Max-mini Branch-&-bound"),
			intParam("initial_trees_random_addition", "No. of Initial Trees (random addition)", "10", "1", "100000", "1"),
			intParam("mp_search_level", "MP Search Level", "1", "1", "5", "1"),
			intParam("max_trees_to_retain", "Max No. of Trees to Retain", "100", "1", "10000", "1"),
			sectionParam("System Resource Usage"),
			intParam("number_of_threads", "Number of Threads", defaultMegaThreadCountString(), "1", strconv.Itoa(maxIntRegistry(1, runtime.NumCPU())), "1"),
		},
	}
}

func distanceTreeProteinParams(statisticalMethod string, gapsMissingDefault string, extra []ParameterDefinition) []ParameterDefinition {
	params := []ParameterDefinition{
		sectionParam("Analysis"),
		readOnlyStringParam("scope", "Scope", "All Selected Taxa"),
		readOnlyPickParam("statistical_method", "Statistical Method", statisticalMethod, statisticalMethod),
		sectionParam("Phylogeny Test"),
		pickParam("phylogeny_test", "Test of Phylogeny", "None", "None", "Bootstrap method"),
		intParam("bootstrap_replicates", "Bootstrap Replicates", "500", "1", "10000", "100"),
		sectionParam("Substitution Model"),
		readOnlyPickParam("substitutions_type", "Substitutions Type", "Amino acid", "Amino acid"),
		pickParam("model_method", "Model/Method", "Poisson model", megaDistanceAminoModels()...),
		sectionParam("Rates and Patterns"),
		pickParam("rates_among_sites", "Rates among Sites", "Uniform Rates", megaDistanceRatesAmongSites()...),
		floatParam("gamma_parameter", "Gamma Parameter", "1.0", "0.05", "999", "0.1", 2),
		pickParam("pattern_among_lineages", "Pattern among Lineages", "Same (Homogeneous)", megaPatternAmongLineages()...),
		sectionParam("Data Subset To Use"),
		pickParam("gaps_missing_data", "Gaps/Missing Data", gapsMissingDefault, "Complete deletion", "Pairwise deletion", "Partial deletion"),
		intParam("site_coverage_cutoff", "Site Coverage Cutoff (%)", "95", "5", "100", "5"),
		sectionParam("System Resource Usage"),
		intParam("number_of_threads", "Number of Threads", defaultMegaThreadCountString(), "1", strconv.Itoa(maxIntRegistry(1, runtime.NumCPU())), "1"),
	}
	return append(params, extra...)
}

func treeMaximumLikelihoodDNA() MethodDefinition {
	def := treeMaximumLikelihoodProtein()
	def.SequenceKinds = []SequenceKind{SequenceNucleotide}
	for i := range def.Parameters {
		switch def.Parameters[i].ID {
		case "substitutions_type":
			def.Parameters[i] = readOnlyPickParam("substitutions_type", "Substitutions Type", "Nucleotide", "Nucleotide")
		case "model_method":
			def.Parameters[i] = pickParam("model_method", "Model/Method", "Tamura-Nei model", megaMLNucleotideModels()...)
		}
	}
	return def
}

func treeNeighborJoiningDNA() MethodDefinition {
	return MethodDefinition{
		ID:            string(TreeNeighborJoining),
		Label:         "Neighbor-Joining",
		RuntimeMethod: "neighbor_joining",
		SequenceKinds: []SequenceKind{SequenceNucleotide},
		Parameters:    distanceTreeDNAParams("Neighbor-joining", nil),
	}
}

func treeMinimumEvolutionDNA() MethodDefinition {
	extra := []ParameterDefinition{
		sectionParam("Tree Inference Options"),
		readOnlyPickParam("me_heuristic_method", "ME Heuristic Method", "Close-Neighbor-Interchange (CNI)", "Close-Neighbor-Interchange (CNI)"),
		readOnlyPickParam("initial_tree_for_me", "Initial Tree for ME", "Obtain initial tree by Neighbor-joining", "Obtain initial tree by Neighbor-joining"),
		intParam("me_search_level", "ME Search Level", "1", "1", "2", "1"),
	}
	return MethodDefinition{
		ID:            string(TreeMinimumEvolution),
		Label:         "Minimum Evolution",
		RuntimeMethod: "minimum_evolution",
		SequenceKinds: []SequenceKind{SequenceNucleotide},
		Parameters:    distanceTreeDNAParams("Minimum Evolution method", extra),
	}
}

func treeUPGMADNA() MethodDefinition {
	return MethodDefinition{
		ID:            string(TreeUPGMA),
		Label:         "UPGMA",
		RuntimeMethod: "upgma",
		SequenceKinds: []SequenceKind{SequenceNucleotide},
		Parameters:    distanceTreeDNAParams("UPGMA", nil),
	}
}

func distanceTreeDNAParams(statisticalMethod string, extra []ParameterDefinition) []ParameterDefinition {
	params := []ParameterDefinition{
		sectionParam("Analysis"),
		readOnlyStringParam("scope", "Scope", "All Selected Taxa"),
		readOnlyPickParam("statistical_method", "Statistical Method", statisticalMethod, statisticalMethod),
		sectionParam("Phylogeny Test"),
		pickParam("phylogeny_test", "Test of Phylogeny", "None", "None", "Bootstrap method"),
		intParam("bootstrap_replicates", "Bootstrap Replicates", "500", "1", "10000", "100"),
		sectionParam("Substitution Model"),
		readOnlyPickParam("substitutions_type", "Substitutions Type", "Nucleotide", "Nucleotide"),
		pickParam("model_method", "Model/Method", "Maximum Composite Likelihood", megaDistanceNucleotideModels()...),
		sectionParam("Rates and Patterns"),
		pickParam("rates_among_sites", "Rates among Sites", "Uniform Rates", megaDistanceRatesAmongSites()...),
		floatParam("gamma_parameter", "Gamma Parameter", "1.0", "0.05", "999", "0.1", 2),
		pickParam("pattern_among_lineages", "Pattern among Lineages", "Same (Homogeneous)", megaPatternAmongLineages()...),
		sectionParam("Data Subset To Use"),
		pickParam("gaps_missing_data", "Gaps/Missing Data", "Pairwise deletion", "Complete deletion", "Pairwise deletion", "Partial deletion"),
		intParam("site_coverage_cutoff", "Site Coverage Cutoff (%)", "95", "5", "100", "5"),
		sectionParam("System Resource Usage"),
		intParam("number_of_threads", "Number of Threads", defaultMegaThreadCountString(), "1", strconv.Itoa(maxIntRegistry(1, runtime.NumCPU())), "1"),
	}
	return append(params, extra...)
}

func treeMaximumParsimonyDNA() MethodDefinition {
	def := treeMaximumParsimonyProtein()
	def.SequenceKinds = []SequenceKind{SequenceNucleotide}
	for i := range def.Parameters {
		if def.Parameters[i].ID == "substitutions_type" {
			def.Parameters[i] = readOnlyPickParam("substitutions_type", "Substitutions Type", "Nucleotide", "Nucleotide")
		}
	}
	return def
}

func mergeDefaultParams(def MethodDefinition, values map[string]string) map[string]string {
	out := DefaultParams(def)
	allowed := editableParamsByID(def)
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		param, ok := allowed[key]
		if !ok {
			continue
		}
		if len(param.Options) > 0 && !optionValueAllowed(param, value) {
			continue
		}
		out[key] = strings.TrimSpace(value)
	}
	return out
}

func paramsMatchDefinition(def MethodDefinition, values map[string]string) bool {
	if len(values) == 0 {
		return true
	}
	allowed := editableParamsByID(def)
	for key, value := range values {
		key = strings.TrimSpace(key)
		param, ok := allowed[key]
		if !ok {
			return false
		}
		if len(param.Options) > 0 && !optionValueAllowed(param, value) {
			return false
		}
	}
	return true
}

func editableParamsByID(def MethodDefinition) map[string]ParameterDefinition {
	allowed := make(map[string]ParameterDefinition)
	for _, param := range def.Parameters {
		if param.Kind == ParameterSection || param.ReadOnly || strings.TrimSpace(param.ID) == "" {
			continue
		}
		allowed[strings.TrimSpace(param.ID)] = param
	}
	return allowed
}

func optionValueAllowed(param ParameterDefinition, value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, option := range param.Options {
		if strings.ToLower(strings.TrimSpace(option)) == value {
			return true
		}
	}
	return false
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
			floatParam("pairwise_gap_opening_penalty", "Gap Opening Penalty", "15", "0", "100", "0.1", 2),
			floatParam("pairwise_gap_extension_penalty", "Gap Extension Penalty", "6.66", "0", "100", "0.1", 2),
			sectionParam("Multiple Alignment"),
			floatParam("multiple_gap_opening_penalty", "Gap Opening Penalty", "15", "0", "100", "0.1", 2),
			floatParam("multiple_gap_extension_penalty", "Gap Extension Penalty", "6.66", "0", "100", "0.1", 2),
			sectionParam("Global Options"),
			pickParam("dna_weight_matrix", "DNA Weight Matrix", "IUB", "IUB", "ClustalW (1.6)"),
			floatParam("transition_weight", "Transition Weight", "0.5", "0", "100", "0.1", 2),
			pickParam("use_negative_matrix", "Use Negative Matrix", "OFF", "OFF", "ON"),
			intParam("delay_divergent_cutoff", "Delay Divergent Cutoff (%)", "30", "0", "100", "1"),
			pickParam("keep_predefined_gaps", "Keep Predefined Gaps", "False", "False", "True"),
			stringParam("guide_tree", "Guide Tree", ""),
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
			floatParam("pairwise_gap_opening_penalty", "Gap Opening Penalty", "10", "0", "100", "0.1", 2),
			floatParam("pairwise_gap_extension_penalty", "Gap Extension Penalty", "0.1", "0", "100", "0.1", 2),
			sectionParam("Multiple Alignment"),
			floatParam("multiple_gap_opening_penalty", "Gap Opening Penalty", "10", "0", "100", "0.1", 2),
			floatParam("multiple_gap_extension_penalty", "Gap Extension Penalty", "0.2", "0", "100", "0.1", 2),
			sectionParam("Global Options"),
			pickParam("protein_weight_matrix", "Protein Weight Matrix", "Gonnet", "BLOSUM", "PAM", "Gonnet", "Identity"),
			pickParam("residue_specific_penalties", "Residue-specific Penalties", "ON", "ON", "OFF"),
			pickParam("hydrophilic_penalties", "Hydrophilic Penalties", "ON", "ON", "OFF"),
			intParam("gap_separation_distance", "Gap Separation Matrix", "4", "0", "100", "1"),
			pickParam("end_gap_separation", "End Gap Separation", "OFF", "ON", "OFF"),
			pickParam("use_negative_matrix", "Use Negative Matrix", "OFF", "ON", "OFF"),
			intParam("delay_divergent_cutoff", "Delay Divergent Cutoff (%)", "30", "0", "100", "1"),
			pickParam("keep_predefined_gaps", "Keep Predefined Gaps", "False", "False", "True"),
			stringParam("guide_tree", "Guide Tree", ""),
		},
	}
}

func alignmentClustalWCodons() MethodDefinition {
	return MethodDefinition{
		ID:             string(AlignmentClustalWCodons),
		Label:          "ClustalW (Codons)",
		RuntimeMethod:  string(AlignmentClustalWCodons),
		SequenceKinds:  []SequenceKind{SequenceNucleotide},
		CodingRequired: true,
		Parameters: []ParameterDefinition{
			sectionParam("Pairwise Alignment"),
			floatParam("pairwise_gap_opening_penalty", "Gap Opening Penalty", "10", "0", "100", "0.1", 2),
			floatParam("pairwise_gap_extension_penalty", "Gap Extension Penalty", "0.1", "0", "100", "0.1", 2),
			sectionParam("Multiple Alignment"),
			floatParam("multiple_gap_opening_penalty", "Gap Opening Penalty", "10", "0", "100", "0.1", 2),
			floatParam("multiple_gap_extension_penalty", "Gap Extension Penalty", "0.2", "0", "100", "0.1", 2),
			sectionParam("Global Options"),
			pickParam("protein_weight_matrix", "Protein Weight Matrix", "BLOSUM", "BLOSUM", "PAM", "Gonnet", "Identity"),
			pickParam("residue_specific_penalties", "Residue-specific Penalties", "ON", "ON", "OFF"),
			pickParam("hydrophilic_penalties", "Hydrophilic Penalties", "ON", "ON", "OFF"),
			intParam("gap_separation_distance", "Gap Separation Matrix", "4", "0", "100", "1"),
			pickParam("end_gap_separation", "End Gap Separation", "ON", "ON", "OFF"),
			pickParam("genetic_code", "Genetic Code Table", "Standard", geneticCodeOptions()...),
			pickParam("use_negative_matrix", "Use Negative Matrix", "ON", "ON", "OFF"),
			intParam("delay_divergent_cutoff", "Delay Divergent Cutoff (%)", "30", "0", "100", "1"),
			pickParam("keep_predefined_gaps", "Keep Predefined Gaps", "False", "False", "True"),
			stringParam("guide_tree", "Guide Tree", ""),
		},
	}
}

func alignmentMUSCLEDNA() MethodDefinition {
	return MethodDefinition{
		ID:            string(AlignmentMUSCLE),
		Label:         "MUSCLE (DNA)",
		RuntimeMethod: "muscle",
		SequenceKinds: []SequenceKind{SequenceNucleotide},
		Parameters:    muscleAlignmentParams(false, true),
	}
}

func alignmentMUSCLEProtein() MethodDefinition {
	return MethodDefinition{
		ID:            string(AlignmentMUSCLE) + "_protein",
		Label:         "MUSCLE",
		RuntimeMethod: "muscle",
		SequenceKinds: []SequenceKind{SequenceProtein},
		Parameters:    muscleAlignmentParams(true, false),
	}
}

func alignmentMUSCLECodons() MethodDefinition {
	return MethodDefinition{
		ID:             string(AlignmentMUSCLECodons),
		Label:          "MUSCLE (Codons)",
		RuntimeMethod:  string(AlignmentMUSCLECodons),
		SequenceKinds:  []SequenceKind{SequenceNucleotide},
		CodingRequired: true,
		Parameters:     muscleAlignmentParams(true, true),
	}
}

func muscleAlignmentParams(includeHydrophobicity bool, includeGeneticCode bool) []ParameterDefinition {
	gapOpenDefault := "-400.00"
	if includeHydrophobicity {
		gapOpenDefault = "-2.90"
	}
	params := []ParameterDefinition{
		sectionParam("Gap Penalties"),
		floatParam("gap_open", "Gap Open", gapOpenDefault, "-2147483648", "0", "0.1", 2),
		floatParam("gap_extend", "Gap Extend", "0.00", "-2147483648", "0", "0.1", 2),
	}
	if includeHydrophobicity {
		params = append(params, floatParam("hydrophobicity_multiplier", "Hydrophobicity Multiplier", "1.20", "0", "2147483647", "0.1", 2))
	}
	params = append(params,
		sectionParam("Memory/Iterations"),
		intParam("max_memory_mb", "Max Memory in MB", "2048", "256", "2147483647", "256"),
		intParam("max_iterations", "Max Iterations", "16", "1", "2147483647", "1"),
		sectionParam("Advanced Options"),
	)
	if includeGeneticCode {
		params = append(params, pickParam("genetic_code", "Genetic Code", "Standard", geneticCodeOptions()...))
	}
	params = append(params,
		pickParam("cluster_method_iterations_1_2", "Cluster Method (Iterations 1,2)", "UPGMA", "UPGMA", "UPGMB", "Neighbor Joining"),
		pickParam("cluster_method_other_iterations", "Cluster Method (Other Iterations)", "UPGMA", "UPGMA", "UPGMB", "Neighbor Joining"),
		intParam("min_diag_length_lambda", "Min Diag Length (Lambda)", "24", "0", "2147483647", "1"),
	)
	return params
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

func readOnlyPickParam(id, label, def string, options ...string) ParameterDefinition {
	param := pickParam(id, label, def, options...)
	param.ReadOnly = true
	return param
}

func stringParam(id, label, def string) ParameterDefinition {
	return ParameterDefinition{ID: id, Label: label, Kind: ParameterString, Default: def, Applicable: true}
}

func readOnlyStringParam(id, label, def string) ParameterDefinition {
	param := stringParam(id, label, def)
	param.ReadOnly = true
	return param
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

func megaDistanceNucleotideModels() []string {
	return []string{
		"No. of differences",
		"p-distance",
		"Jukes-Cantor model",
		"Kimura 2-parameter model",
		"Tajima-Nei model",
		"Tamura 3-parameter model",
		"Tamura-Nei model",
		"Maximum Composite Likelihood",
		"LogDet (Tamura-Kumar)",
	}
}

func megaDistanceAminoModels() []string {
	return []string{
		"No. of differences",
		"p-distance",
		"Poisson model",
		"Equal input model",
		"Dayhoff model",
		"Jones-Taylor-Thornton (JTT) model",
	}
}

func megaMLNucleotideModels() []string {
	return []string{
		"Jukes-Cantor model",
		"Kimura 2-parameter model",
		"Tamura 3-parameter model",
		"Hasegawa-Kishino-Yano model",
		"Tamura-Nei model",
		"General Time Reversible model",
	}
}

func megaMLAminoModels() []string {
	return []string{
		"Poisson model",
		"Equal input model",
		"Dayhoff model",
		"Dayhoff model with Freqs. (+F)",
		"Jones-Taylor-Thornton (JTT) model",
		"JTT with Freqs. (+F) model",
		"WAG model",
		"WAG with Freqs. (+F) model",
		"LG model",
		"LG with Freqs. (+F) model",
		"General Reversible Mitochondrial (mtREV)",
		"mtREV with Freqs. (+F) model",
		"General Reversible Chloroplast (cpREV)",
		"cpREV with Freqs. (+F) model",
		"General Reverse Transcriptase model (rtREV)",
		"rtREV with Freqs. (+F) model",
	}
}

func megaDistanceRatesAmongSites() []string {
	return []string{
		"Uniform Rates",
		"Gamma Distributed (G)",
	}
}

func megaMLRatesAmongSites() []string {
	return []string{
		"Uniform Rates",
		"Gamma Distributed (G)",
		"Has Invariant Sites (I)",
		"Gamma Distributed With Invariant Sites (G+I)",
	}
}

func megaPatternAmongLineages() []string {
	return []string{
		"Same (Homogeneous)",
		"Different (Heterogeneous)",
	}
}

func megaMLSearchMethods() []string {
	return []string{
		"Nearest-Neighbor-Interchange (NNI)",
		"Subtree-Pruning-Regrafting - Fast (SPR level 3)",
		"Subtree-Pruning-Regrafting - Extensive (SPR level 5)",
	}
}

func megaMLInitialTrees() []string {
	return []string{
		"Make initial tree automatically (Default - NJ/MP)",
		"Make initial tree automatically (Maximum Parsimony)",
		"Make multiple initial trees automatically (Maximum Parsimony)",
		"Make initial tree automatically (Neighbor Joining)",
		"Use tree from file",
	}
}

func megaMLBranchSwapFilters() []string {
	return []string{
		"None",
		"Weak",
		"Moderate",
		"Strong",
	}
}

func defaultMegaThreadCountString() string {
	n := runtime.NumCPU()
	if n > 1 {
		n--
	}
	if n < 1 {
		n = 1
	}
	return strconv.Itoa(n)
}

func maxIntRegistry(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
