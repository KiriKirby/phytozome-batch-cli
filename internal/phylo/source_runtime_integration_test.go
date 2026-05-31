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
			if !looksLikeNewick(newick) {
				t.Fatalf("Newick output does not look valid: %q", newick)
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

func TestMegaPHGORuntimeProteinStopCodonProbe(t *testing.T) {
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
			if err != nil {
				t.Fatalf("RunPlanWithRuntime returned error: %v\nartifacts: %s\n%s", err, result.ArtifactDir, runtimeProbeDebugText(result.ArtifactDir))
			}
			if strings.Contains(result.Plan.AlignedFASTA, "*") {
				t.Fatalf("runtime-aligned FASTA should not contain protein stop codons: %s", result.Plan.AlignedFASTA)
			}
			if !looksLikeNewick(strings.TrimSpace(result.Plan.Newick)) {
				t.Fatalf("Newick output does not look valid: %q", result.Plan.Newick)
			}
			logData, err := os.ReadFile(filepath.Join(result.ArtifactDir, "runtime.log"))
			if err != nil {
				t.Fatalf("read runtime log: %v", err)
			}
			if !strings.Contains(string(logData), "protein.sanitized") {
				t.Fatalf("runtime log should record protein stop-codon sanitization, got:\n%s", logData)
			}
		})
	}
}

func TestMegaPHGORuntimeProteinModeConvertsNucleotideRowsProbe(t *testing.T) {
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
			settings.ConversionAction = ConversionActionConvert
			plan, err := BuildRunPlan("mixed-probe", "run1", filepath.Join(t.TempDir(), "tree"), settings, SequenceProtein, records, meta, "", "", now)
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
			logText := string(logData)
			if !strings.Contains(logText, "conversion.applied") {
				t.Fatalf("runtime log should record conversion.applied, got:\n%s", logText)
			}
			if !strings.Contains(logText, "converted_dna_to_protein=2") {
				t.Fatalf("runtime log should report two DNA-to-protein conversions, got:\n%s", logText)
			}

			maxUngapped := 0
			for _, seq := range alignedSequencesForTest(t, result.Plan.AlignedFASTA) {
				ungapped := strings.ReplaceAll(seq, "-", "")
				if len(ungapped) > maxUngapped {
					maxUngapped = len(ungapped)
				}
				if looksNucleotideOnly(ungapped) {
					t.Fatalf("protein-mode aligned FASTA should not contain nucleotide-only sequences after conversion: %s", result.Plan.AlignedFASTA)
				}
			}
			if maxUngapped > 12 {
				t.Fatalf("converted protein alignment should stay peptide-sized, max ungapped length=%d\n%s", maxUngapped, result.Plan.AlignedFASTA)
			}
			if !looksLikeNewick(strings.TrimSpace(result.Plan.Newick)) {
				t.Fatalf("Newick output does not look valid: %q", result.Plan.Newick)
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
			settings.ConversionAction = ConversionActionConvert
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
			if !looksLikeNewick(strings.TrimSpace(result.Plan.Newick)) {
				t.Fatalf("Newick output does not look valid: %q", result.Plan.Newick)
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
