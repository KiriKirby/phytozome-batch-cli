package phylo

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMegaPHGORuntimeRealFASTAProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	fastaPath := strings.TrimSpace(os.Getenv("PHYTOZOME_MEGAPHGO_FASTA"))
	if fastaPath == "" {
		fastaPath = `C:\Users\wangsychn\Desktop\2026年5月14日\output\Monolignol_Biosynthesis\CCoAOMT1.fasta`
	}
	data, err := os.ReadFile(fastaPath)
	if err != nil {
		t.Fatalf("read real FASTA %s: %v", fastaPath, err)
	}
	sources := rowSourcesFromFASTAForTest(t, string(data))
	if len(sources) < 2 {
		t.Fatalf("real FASTA should contain at least two records, got %d", len(sources))
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()
	for _, method := range []AlignmentMethod{AlignmentClustalW, AlignmentMUSCLE} {
		t.Run(string(method), func(t *testing.T) {
			now := time.Now()
			records, meta, err := BuildInput(sources, "label_name", "real-probe", now)
			if err != nil {
				t.Fatalf("BuildInput returned error: %v", err)
			}
			settings := DefaultTreeSettings()
			settings.AlignmentMethod = method
			baseDir := filepath.Join(t.TempDir(), "tree")
			if strings.TrimSpace(os.Getenv("PHYTOZOME_KEEP_MEGAPHGO_PROBE")) != "" {
				baseDir = filepath.Join(os.TempDir(), "phgo-megaphgo-real-probe-"+string(method)+"-"+now.Format("20060102-150405.000000000"))
			}
			plan, err := BuildRunPlan("real-probe", "run1", baseDir, settings, SequenceProtein, records, meta, "", "", now)
			if err != nil {
				t.Fatalf("BuildRunPlan returned error: %v", err)
			}
			result, err := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})
			if err != nil {
				t.Fatalf("RunPlanWithRuntime returned error: %v\nartifacts: %s\n%s", err, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
			}
			if strings.TrimSpace(result.Plan.AlignedFASTA) == "" {
				t.Fatalf("aligned FASTA is empty")
			}
			newick := strings.TrimSpace(result.Plan.Newick)
			if newick == "" {
				t.Fatalf("runtime Newick output is empty")
			}
			if !strings.Contains(newick, "PHGOT000001") {
				t.Fatalf("Newick should use stable PHgo taxon IDs, got: %s", newick)
			}
			if strings.Contains(newick, "phgo://") {
				t.Fatalf("Newick must not contain raw PHgo FASTA headers: %s", newick)
			}
		})
	}
}

func TestMegaPHGORuntimeDesktopFASTAMatrixProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" || strings.TrimSpace(os.Getenv("PHYTOZOME_MEGAPHGO_DESKTOP_MATRIX")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 and PHYTOZOME_MEGAPHGO_DESKTOP_MATRIX=1 to run the desktop FASTA matrix probe")
	}
	paths := desktopMatrixFASTAPathsForTest()
	if env := strings.TrimSpace(os.Getenv("PHYTOZOME_MEGAPHGO_FASTA")); env != "" {
		paths = []string{env}
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()
	treeMethods := []TreeMethod{TreeNeighborJoining, TreeMinimumEvolution, TreeUPGMA, TreeMaximumLikelihood, TreeMaximumParsimony}
	cases := []struct {
		name           string
		kind           SequenceKind
		target         ConversionTarget
		alignments     []AlignmentMethod
		requireSuccess bool
	}{
		{name: "protein", kind: SequenceProtein, target: ConversionTargetProtein, alignments: []AlignmentMethod{AlignmentClustalW, AlignmentMUSCLE}, requireSuccess: true},
		{name: "dna", kind: SequenceNucleotide, target: ConversionTargetDNA, alignments: []AlignmentMethod{AlignmentClustalW, AlignmentMUSCLE, AlignmentClustalWCodons, AlignmentMUSCLECodons}},
	}
	for _, path := range paths {
		path := path
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read desktop FASTA %s: %v", path, err)
		}
		sources := rowSourcesFromFASTAForTest(t, string(data))
		if len(sources) < 2 {
			t.Fatalf("desktop FASTA %s should contain at least two records, got %d", path, len(sources))
		}
		if len(sources) > 4 {
			sources = append([]RowSource(nil), sources[:4]...)
		}
		sequencesAreDNA := true
		for _, source := range sources {
			sequencesAreDNA = sequencesAreDNA && looksNucleotideOnly(source.Sequence)
		}
		for _, tc := range cases {
			tc := tc
			requireSuccess := tc.requireSuccess || (tc.kind == SequenceNucleotide && sequencesAreDNA)
			t.Run(filepath.Base(path)+"/"+tc.name, func(t *testing.T) {
				for _, alignment := range tc.alignments {
					alignment := alignment
					for _, treeMethod := range treeMethods {
						treeMethod := treeMethod
						t.Run(string(alignment)+"/"+string(treeMethod), func(t *testing.T) {
							now := time.Now()
							records, meta, err := BuildInput(sources, "label_name", "desktop-matrix-probe", now)
							if err != nil {
								t.Fatalf("BuildInput returned error: %v", err)
							}
							settings := DefaultTreeSettings()
							settings.ConversionTarget = tc.target
							settings.AlignmentMethod = alignment
							settings.TreeMethod = treeMethod
							settings.TreeParams = map[string]string{
								"phylogeny_test":    "None",
								"number_of_threads": "1",
							}
							plan, err := BuildRunPlan("desktop-matrix-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, tc.kind, records, meta, "", "", now)
							if err != nil {
								t.Fatalf("BuildRunPlan returned error: %v", err)
							}
							result, err := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})
							if err != nil {
								if requireSuccess {
									t.Fatalf("RunPlanWithRuntime returned error: %v\nartifacts: %s\n%s", err, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
								}
								debug := runtimeProbeDebugText(result.ArtifactDir)
								if strings.TrimSpace(result.ArtifactDir) == "" || !strings.Contains(debug, `"error_text"`) {
									t.Fatalf("runtime error should keep artifacts and runtime error_text, err=%v\nartifacts: %s\n%s", err, result.ArtifactDir, debug)
								}
								return
							}
							if strings.TrimSpace(result.Plan.AlignedFASTA) == "" || strings.TrimSpace(result.Plan.Newick) == "" {
								t.Fatalf("successful runtime output should include aligned FASTA and Newick:\n%s", runtimeProbeDebugText(result.ArtifactDir))
							}
						})
					}
				}
			})
		}
	}
}

func TestMegaPHGORuntimeProteinStopCodonProbeDoesNotSanitizeInPHgo(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()

	sources := rowSourcesFromFASTAForTest(t, strings.Join([]string{
		">alpha",
		"MKTAYIAKQRQISFVKSHFSRQ*",
		">beta",
		"MKTAYIAKQRQISFVKSHFSR*QD*",
		">gamma",
		"GATAYIAKQRQISFVKSHFSRQD*",
	}, "\n"))
	for _, method := range []AlignmentMethod{AlignmentClustalW, AlignmentMUSCLE} {
		t.Run(string(method), func(t *testing.T) {
			now := time.Now()
			records, meta, err := BuildInput(sources, "label_name", "stop-probe", now)
			if err != nil {
				t.Fatalf("BuildInput returned error: %v", err)
			}
			settings := DefaultTreeSettings()
			settings.AlignmentMethod = method
			plan, err := BuildRunPlan("stop-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, SequenceProtein, records, meta, "", "", now)
			if err != nil {
				t.Fatalf("BuildRunPlan returned error: %v", err)
			}
			result, err := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})
			logData, readErr := os.ReadFile(filepath.Join(result.ArtifactDir, "runtime.log"))
			if readErr != nil {
				t.Fatalf("read runtime log: %v", readErr)
			}
			if strings.Contains(string(logData), "protein.sanitized") {
				t.Fatalf("runtime log must not record PHgo protein stop-codon sanitization, got:\n%s", logData)
			}
			if err != nil {
				return
			}
			if strings.TrimSpace(result.Plan.Newick) == "" {
				t.Fatalf("runtime Newick output is empty")
			}
		})
	}
}

func TestMegaPHGORuntimeProteinModeDoesNotConvertNucleotideRowsProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()

	sources := []RowSource{
		{
			ItemTitle:    "mixed protein probe",
			RowIndex:     0,
			Sequence:     "MPEPTIDEQ",
			SequenceKind: SequenceProtein,
			SourceType:   "fasta",
			OriginalHead: "protein-alpha",
			TableValues: map[string]string{
				"head":       "protein-alpha",
				"label_name": "protein-alpha",
			},
		},
		{
			ItemTitle:    "mixed protein probe",
			RowIndex:     1,
			Sequence:     "MPEPTLDEQ",
			SequenceKind: SequenceProtein,
			SourceType:   "fasta",
			OriginalHead: "protein-beta",
			TableValues: map[string]string{
				"head":       "protein-beta",
				"label_name": "protein-beta",
			},
		},
		{
			ItemTitle:    "mixed protein probe",
			RowIndex:     2,
			Sequence:     "ATGCCTGAACCTACTATTGATGAACAA",
			SequenceKind: SequenceNucleotide,
			SourceType:   "fasta",
			OriginalHead: "dna-alpha",
			TableValues: map[string]string{
				"head":       "dna-alpha",
				"label_name": "dna-alpha",
			},
		},
		{
			ItemTitle:    "mixed protein probe",
			RowIndex:     3,
			Sequence:     "ATGCCTGAACCTACTCTGGATGAACAA",
			SequenceKind: SequenceNucleotide,
			SourceType:   "fasta",
			OriginalHead: "dna-beta",
			TableValues: map[string]string{
				"head":       "dna-beta",
				"label_name": "dna-beta",
			},
		},
	}

	for _, method := range []AlignmentMethod{AlignmentClustalW, AlignmentMUSCLE} {
		t.Run(string(method), func(t *testing.T) {
			now := time.Now()
			records, meta, err := BuildInput(sources, "label_name", "mixed-probe", now)
			if err != nil {
				t.Fatalf("BuildInput returned error: %v", err)
			}
			settings := DefaultTreeSettings()
			settings.AlignmentMethod = method
			settings.ConversionTarget = ConversionTargetProtein
			plan, err := BuildRunPlan("mixed-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, SequenceProtein, records, meta, "", "", now)
			if err != nil {
				t.Fatalf("BuildRunPlan returned error: %v", err)
			}
			result, runErr := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})

			logData, err := os.ReadFile(filepath.Join(result.ArtifactDir, "runtime.log"))
			if err != nil {
				t.Fatalf("read runtime log: %v", err)
			}
			logText := string(logData)
			if strings.Contains(logText, "conversion.applied") || strings.Contains(logText, "converted_dna_to_protein=") {
				t.Fatalf("runtime log should not record PHgo-side DNA-to-protein conversion, got:\n%s", logText)
			}
			if !strings.Contains(result.Plan.InputFASTA, "ATGCCTGAACCTACTATTGATGAACAA") {
				t.Fatalf("protein-mode request should preserve nucleotide row content for MEGA instead of converting it:\n%s", result.Plan.InputFASTA)
			}
			if runErr != nil {
				if !strings.Contains(runtimeProbeDebugText(result.ArtifactDir), `"error_text"`) {
					t.Fatalf("RunPlanWithRuntime returned error without runtime error_text: %v\nartifacts: %s\n%s", runErr, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
				}
				return
			}
			if strings.TrimSpace(result.Plan.AlignedFASTA) == "" || strings.TrimSpace(result.Plan.Newick) == "" {
				t.Fatalf("successful runtime output should include aligned FASTA and Newick:\n%s", runtimeProbeDebugText(result.ArtifactDir))
			}
		})
	}
}

func TestMegaPHGORuntimeProteinTreeMethodsProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()

	sources := rowSourcesFromFASTAForTest(t, strings.Join([]string{
		">alpha",
		"MKTAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">beta",
		"MKTAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
		">gamma",
		"GATAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">delta",
		"GATAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
	}, "\n"))
	methods := []TreeMethod{
		TreeNeighborJoining,
		TreeMinimumEvolution,
		TreeUPGMA,
		TreeMaximumLikelihood,
		TreeMaximumParsimony,
	}
	for _, method := range methods {
		t.Run(string(method), func(t *testing.T) {
			now := time.Now()
			records, meta, err := BuildInput(sources, "label_name", "protein-tree-method-probe", now)
			if err != nil {
				t.Fatalf("BuildInput returned error: %v", err)
			}
			settings := DefaultTreeSettings()
			settings.AlignmentMethod = AlignmentClustalW
			settings.TreeMethod = method
			plan, err := BuildRunPlan("protein-tree-method-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, SequenceProtein, records, meta, "", "", now)
			if err != nil {
				t.Fatalf("BuildRunPlan returned error: %v", err)
			}
			result, err := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})
			if err != nil {
				t.Fatalf("RunPlanWithRuntime returned error: %v\nartifacts: %s\n%s", err, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
			}
			if strings.TrimSpace(result.Plan.Newick) == "" {
				t.Fatalf("runtime Newick output is empty")
			}
			summary, err := os.ReadFile(filepath.Join(result.ArtifactDir, "runtime-summary.txt"))
			if err != nil {
				t.Fatalf("read runtime summary: %v", err)
			}
			if !strings.Contains(string(summary), "tree_method="+string(method)) {
				t.Fatalf("runtime summary did not record expected tree method for %s:\n%s", method, summary)
			}
		})
	}
}

func TestMegaPHGORuntimeDNAModeAlignersProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()

	sources := []RowSource{
		{
			ItemTitle:    "dna aligner probe",
			RowIndex:     0,
			Sequence:     "ATGGCTGAGTTCAAAGGCTACGTTGCTGCTGAA",
			SequenceKind: SequenceNucleotide,
			SourceType:   "fasta",
			OriginalHead: "dna-alpha",
			TableValues:  map[string]string{"head": "dna-alpha", "label_name": "dna-alpha"},
		},
		{
			ItemTitle:    "dna aligner probe",
			RowIndex:     1,
			Sequence:     "ATGGCTGAATTTAAAGGCTATGTTGCTGCCGAA",
			SequenceKind: SequenceNucleotide,
			SourceType:   "fasta",
			OriginalHead: "dna-beta",
			TableValues:  map[string]string{"head": "dna-beta", "label_name": "dna-beta"},
		},
		{
			ItemTitle:    "dna aligner probe",
			RowIndex:     2,
			Sequence:     "ATGGCCGAGTTCAAAGGTTACGTAGCTGCTGAG",
			SequenceKind: SequenceNucleotide,
			SourceType:   "fasta",
			OriginalHead: "dna-gamma",
			TableValues:  map[string]string{"head": "dna-gamma", "label_name": "dna-gamma"},
		},
	}

	for _, method := range []AlignmentMethod{AlignmentClustalW, AlignmentMUSCLE, AlignmentClustalWCodons, AlignmentMUSCLECodons} {
		t.Run(string(method), func(t *testing.T) {
			now := time.Now()
			records, meta, err := BuildInput(sources, "label_name", "dna-aligner-probe", now)
			if err != nil {
				t.Fatalf("BuildInput returned error: %v", err)
			}
			settings := DefaultTreeSettings()
			settings.ConversionTarget = ConversionTargetDNA
			settings.AlignmentMethod = method
			plan, err := BuildRunPlan("dna-aligner-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, SequenceNucleotide, records, meta, "", "", now)
			if err != nil {
				t.Fatalf("BuildRunPlan returned error: %v", err)
			}
			result, err := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})
			if err != nil {
				t.Fatalf("RunPlanWithRuntime returned error: %v\nartifacts: %s\n%s", err, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
			}
			for _, seq := range alignedSequencesForTest(t, result.Plan.AlignedFASTA) {
				ungapped := strings.ReplaceAll(seq, "-", "")
				if !looksNucleotideOnly(ungapped) {
					t.Fatalf("DNA-mode aligned FASTA should stay nucleotide for %s: %s", method, result.Plan.AlignedFASTA)
				}
			}
			if strings.TrimSpace(result.Plan.Newick) == "" {
				t.Fatalf("runtime Newick output is empty")
			}
			summary, err := os.ReadFile(filepath.Join(result.ArtifactDir, "runtime-summary.txt"))
			if err != nil {
				t.Fatalf("read runtime summary: %v", err)
			}
			if !strings.Contains(string(summary), "alignment_method="+string(normalizedRuntimeAlignmentMethodForTest(method))) {
				t.Fatalf("runtime summary did not record expected alignment method for %s:\n%s", method, summary)
			}
		})
	}
}

func TestMegaPHGORuntimeDNATreeMethodsProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()

	sources := []RowSource{
		{
			ItemTitle:    "dna tree method probe",
			RowIndex:     0,
			Sequence:     "ATGGCTGAGTTCAAAGGCTACGTTGCTGCTGAA",
			SequenceKind: SequenceNucleotide,
			SourceType:   "fasta",
			OriginalHead: "dna-alpha",
			TableValues:  map[string]string{"head": "dna-alpha", "label_name": "dna-alpha"},
		},
		{
			ItemTitle:    "dna tree method probe",
			RowIndex:     1,
			Sequence:     "ATGGCTGAATTTAAAGGCTATGTTGCTGCCGAA",
			SequenceKind: SequenceNucleotide,
			SourceType:   "fasta",
			OriginalHead: "dna-beta",
			TableValues:  map[string]string{"head": "dna-beta", "label_name": "dna-beta"},
		},
		{
			ItemTitle:    "dna tree method probe",
			RowIndex:     2,
			Sequence:     "ATGGCCGAGTTCAAAGGTTACGTAGCTGCTGAG",
			SequenceKind: SequenceNucleotide,
			SourceType:   "fasta",
			OriginalHead: "dna-gamma",
			TableValues:  map[string]string{"head": "dna-gamma", "label_name": "dna-gamma"},
		},
		{
			ItemTitle:    "dna tree method probe",
			RowIndex:     3,
			Sequence:     "ATGGCCGAATTTAAAGGTTATGTAGCTGCCGAG",
			SequenceKind: SequenceNucleotide,
			SourceType:   "fasta",
			OriginalHead: "dna-delta",
			TableValues:  map[string]string{"head": "dna-delta", "label_name": "dna-delta"},
		},
	}

	for _, method := range []TreeMethod{TreeNeighborJoining, TreeMinimumEvolution, TreeUPGMA, TreeMaximumLikelihood, TreeMaximumParsimony} {
		t.Run(string(method), func(t *testing.T) {
			now := time.Now()
			records, meta, err := BuildInput(sources, "label_name", "dna-tree-method-probe", now)
			if err != nil {
				t.Fatalf("BuildInput returned error: %v", err)
			}
			settings := DefaultTreeSettings()
			settings.ConversionTarget = ConversionTargetDNA
			settings.AlignmentMethod = AlignmentClustalW
			settings.TreeMethod = method
			plan, err := BuildRunPlan("dna-tree-method-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, SequenceNucleotide, records, meta, "", "", now)
			if err != nil {
				t.Fatalf("BuildRunPlan returned error: %v", err)
			}
			result, err := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})
			if err != nil {
				t.Fatalf("RunPlanWithRuntime returned error: %v\nartifacts: %s\n%s", err, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
			}
			if strings.TrimSpace(result.Plan.Newick) == "" {
				t.Fatalf("runtime Newick output is empty")
			}
			summary, err := os.ReadFile(filepath.Join(result.ArtifactDir, "runtime-summary.txt"))
			if err != nil {
				t.Fatalf("read runtime summary: %v", err)
			}
			if !strings.Contains(string(summary), "tree_method="+string(method)) {
				t.Fatalf("runtime summary did not record expected tree method for %s:\n%s", method, summary)
			}
		})
	}
}

func TestMegaPHGORuntimeMLBranchSwapFilterProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()

	sources := rowSourcesFromFASTAForTest(t, strings.Join([]string{
		">alpha",
		"MKTAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYMKTAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">beta",
		"MKTAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYMKTAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
		">gamma",
		"GATAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYGATAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">delta",
		"GATAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYGATAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
		">epsilon",
		"MKTAYVAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYMKTAYVAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">zeta",
		"GATAYVAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYGATAYVAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
	}, "\n"))
	now := time.Now()
	records, meta, err := BuildInput(sources, "label_name", "ml-filter-probe", now)
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	settings := DefaultTreeSettings()
	settings.AlignmentMethod = AlignmentClustalW
	settings.TreeMethod = TreeMaximumLikelihood
	settings.TreeParams = map[string]string{"branch_swap_filter": "Strong"}
	plan, err := BuildRunPlan("ml-filter-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, SequenceProtein, records, meta, "", "", now)
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}
	result, err := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})
	if err != nil {
		t.Fatalf("RunPlanWithRuntime returned error: %v\nartifacts: %s\n%s", err, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
	}
	logData, err := os.ReadFile(filepath.Join(result.ArtifactDir, "runtime.log"))
	if err != nil {
		t.Fatalf("read runtime log: %v", err)
	}
	if !strings.Contains(string(logData), "search_filter=0.50000") {
		t.Fatalf("ML branch swap filter should be applied to MEGA SearchFilter, got:\n%s", logData)
	}
}

func TestMegaPHGORuntimeDistanceBootstrapProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()

	sources := rowSourcesFromFASTAForTest(t, strings.Join([]string{
		">alpha",
		"MKTAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYMKTAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">beta",
		"MKTAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYMKTAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
		">gamma",
		"GATAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYGATAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">delta",
		"GATAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYGATAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
		">epsilon",
		"MKTAYVAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYMKTAYVAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">zeta",
		"GATAYVAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYGATAYVAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
	}, "\n"))
	for _, method := range []TreeMethod{TreeNeighborJoining, TreeUPGMA} {
		t.Run(string(method), func(t *testing.T) {
			now := time.Now()
			records, meta, err := BuildInput(sources, "label_name", "distance-bootstrap-probe", now)
			if err != nil {
				t.Fatalf("BuildInput returned error: %v", err)
			}
			settings := DefaultTreeSettings()
			settings.AlignmentMethod = AlignmentClustalW
			settings.TreeMethod = method
			settings.TreeParams = map[string]string{
				"phylogeny_test":       "Bootstrap method",
				"bootstrap_replicates": "1",
				"number_of_threads":    "1",
			}
			plan, err := BuildRunPlan("distance-bootstrap-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, SequenceProtein, records, meta, "", "", now)
			if err != nil {
				t.Fatalf("BuildRunPlan returned error: %v", err)
			}
			result, err := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})
			if err != nil {
				t.Fatalf("RunPlanWithRuntime returned error: %v\nartifacts: %s\n%s", err, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
			}
			if strings.TrimSpace(result.Plan.Newick) == "" {
				t.Fatalf("runtime Newick output is empty")
			}
			logData, err := os.ReadFile(filepath.Join(result.ArtifactDir, "runtime.log"))
			if err != nil {
				t.Fatalf("read runtime log: %v", err)
			}
			if !strings.Contains(string(logData), "distance.bootstrap.complete") {
				t.Fatalf("distance bootstrap should run through MEGA bootstrap thread, got:\n%s", logData)
			}
		})
	}
}

func TestMegaPHGORuntimeMinimumEvolutionBootstrapProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()

	sources := rowSourcesFromFASTAForTest(t, strings.Join([]string{
		">alpha",
		"MKTAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYMKTAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">beta",
		"MKTAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYMKTAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
		">gamma",
		"GATAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYGATAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">delta",
		"GATAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYGATAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
	}, "\n"))
	now := time.Now()
	records, meta, err := BuildInput(sources, "label_name", "me-bootstrap-error-probe", now)
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	settings := DefaultTreeSettings()
	settings.AlignmentMethod = AlignmentClustalW
	settings.TreeMethod = TreeMinimumEvolution
	settings.TreeParams = map[string]string{
		"phylogeny_test":       "Bootstrap method",
		"bootstrap_replicates": "1",
		"number_of_threads":    "1",
	}
	plan, err := BuildRunPlan("me-bootstrap-error-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, SequenceProtein, records, meta, "", "", now)
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := RunPlanWithRuntime(ctx, plan, RuntimeOptions{})
	if err != nil {
		t.Fatalf("minimum-evolution bootstrap failed: %v\nartifacts: %s\n%s", err, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
	}
	debug := runtimeProbeDebugText(result.ArtifactDir)
	if !strings.Contains(debug, "distance.bootstrap.start method=minimum_evolution") || !strings.Contains(debug, "distance.bootstrap.complete") {
		t.Fatalf("minimum-evolution bootstrap log did not prove MEGA TBootstrapMEThread completion:\n%s", debug)
	}
	if strings.TrimSpace(result.Plan.Newick) == "" {
		t.Fatalf("minimum-evolution bootstrap produced an empty Newick; artifacts: %s\n%s", result.ArtifactDir, debug)
	}
}

func TestMegaPHGORuntimeMPBootstrapProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PHYTOZOME_RUN_MEGAPHGO_RUNTIME")) == "" {
		t.Skip("set PHYTOZOME_RUN_MEGAPHGO_RUNTIME=1 to run the real mega-phgo-runtime probe")
	}
	appRoot := repoRootForRuntimeProbeTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(appRoot); err != nil {
		t.Fatalf("switch to runtime app root %s: %v", appRoot, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir %s: %v", oldWD, err)
		}
	}()

	sources := rowSourcesFromFASTAForTest(t, strings.Join([]string{
		">alpha",
		"MKTAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYMKTAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">beta",
		"MKTAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYMKTAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
		">gamma",
		"GATAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYGATAYIAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">delta",
		"GATAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYGATAYIAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
		">epsilon",
		"MKTAYVAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNYMKTAYVAKQRQISFVKSHFSRQDILDLWIYHTQGYFPDWQNY",
		">zeta",
		"GATAYVAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNYGATAYVAKQRQISFVKSHFSRQNILDLWIYHTQGYFPDWQNY",
	}, "\n"))
	for _, tc := range []struct {
		method TreeMethod
		logKey string
	}{
		{method: TreeMaximumLikelihood, logKey: "maximum_likelihood.bootstrap complete=true"},
		{method: TreeMaximumParsimony, logKey: "maximum_parsimony.bootstrap complete=true"},
	} {
		t.Run(string(tc.method), func(t *testing.T) {
			now := time.Now()
			records, meta, err := BuildInput(sources, "label_name", "ml-mp-bootstrap-probe", now)
			if err != nil {
				t.Fatalf("BuildInput returned error: %v", err)
			}
			settings := DefaultTreeSettings()
			settings.AlignmentMethod = AlignmentClustalW
			settings.TreeMethod = tc.method
			settings.TreeParams = map[string]string{
				"phylogeny_test":       "Bootstrap method",
				"bootstrap_replicates": "3",
				"number_of_threads":    "1",
			}
			plan, err := BuildRunPlan("ml-mp-bootstrap-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, SequenceProtein, records, meta, "", "", now)
			if err != nil {
				t.Fatalf("BuildRunPlan returned error: %v", err)
			}
			result, err := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})
			if err != nil {
				t.Fatalf("RunPlanWithRuntime returned error: %v\nartifacts: %s\n%s", err, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
			}
			if strings.TrimSpace(result.Plan.Newick) == "" {
				t.Fatalf("runtime Newick output is empty")
			}
			logData, err := os.ReadFile(filepath.Join(result.ArtifactDir, "runtime.log"))
			if err != nil {
				t.Fatalf("read runtime log: %v", err)
			}
			if !strings.Contains(string(logData), tc.logKey) {
				t.Fatalf("%s bootstrap should run through MEGA bootstrap thread, got:\n%s", tc.method, logData)
			}
		})
	}
}

func normalizedRuntimeAlignmentMethodForTest(method AlignmentMethod) AlignmentMethod {
	switch method {
	case AlignmentClustalW, AlignmentMUSCLE, AlignmentClustalWCodons, AlignmentMUSCLECodons:
		return method
	default:
		return method
	}
}

func runtimeProbeDebugText(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	names := []string{"runtime-response.json", "runtime.stderr.txt", "runtime.stdout.txt", "runtime.log", "runtime-summary.txt"}
	var b strings.Builder
	b.WriteString("runtime=")
	b.WriteString(runtime.GOOS)
	b.WriteString("/")
	b.WriteString(runtime.GOARCH)
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		b.WriteString("\n--- ")
		b.WriteString(name)
		b.WriteString(" ---\n")
		text := string(data)
		if len(text) > 4096 {
			text = text[:4096] + "\n...[truncated]"
		}
		b.WriteString(text)
	}
	return b.String()
}

func repoRootForRuntimeProbeTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}

func desktopMatrixFASTAPathsForTest() []string {
	return []string{
		`C:\Users\wangsychn\Desktop\4CL_other_yt2_0.fasta`,
		`C:\Users\wangsychn\Desktop\4CL_other.fasta`,
	}
}

func rowSourcesFromFASTAForTest(t *testing.T, fasta string) []RowSource {
	t.Helper()
	var out []RowSource
	var head strings.Builder
	var seq strings.Builder
	flush := func() {
		header := strings.TrimSpace(head.String())
		sequence := strings.TrimSpace(seq.String())
		if header == "" && sequence == "" {
			return
		}
		label := header
		if fields := strings.Fields(header); len(fields) > 0 {
			label = fields[0]
		}
		out = append(out, RowSource{
			ItemTitle:    "real FASTA",
			ItemIndex:    0,
			RowIndex:     len(out),
			Sequence:     sequence,
			SourceType:   "fasta",
			OriginalHead: header,
			TableValues: map[string]string{
				"head":       header,
				"label_name": label,
			},
		})
		head.Reset()
		seq.Reset()
	}
	for _, line := range strings.Split(strings.ReplaceAll(fasta, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ">") {
			flush()
			head.WriteString(strings.TrimSpace(strings.TrimPrefix(line, ">")))
			continue
		}
		seq.WriteString(line)
	}
	flush()
	return out
}

func alignedSequencesForTest(t *testing.T, fasta string) []string {
	t.Helper()
	var seqs []string
	var seq strings.Builder
	flush := func() {
		if seq.Len() == 0 {
			return
		}
		seqs = append(seqs, seq.String())
		seq.Reset()
	}
	for _, line := range strings.Split(strings.ReplaceAll(fasta, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ">") {
			flush()
			continue
		}
		seq.WriteString(line)
	}
	flush()
	if len(seqs) == 0 {
		t.Fatalf("expected at least one aligned FASTA sequence")
	}
	return seqs
}

func looksNucleotideOnly(sequence string) bool {
	sequence = strings.TrimSpace(strings.ToUpper(sequence))
	if sequence == "" {
		return false
	}
	for _, ch := range sequence {
		if !strings.ContainsRune("ACGTUN", ch) {
			return false
		}
	}
	return true
}
