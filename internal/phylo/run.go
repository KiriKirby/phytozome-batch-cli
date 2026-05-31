package phylo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func ReuseRunPlanArtifacts(plan RunPlan) (RunResult, error) {
	if strings.TrimSpace(plan.BaseDir) == "" {
		return RunResult{}, nil
	}
	manifest, err := LoadRunManifest(plan.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return RunResult{}, nil
		}
		return RunResult{}, err
	}
	if !ManifestMatchesPlan(manifest, plan) {
		return RunResult{}, nil
	}
	alignedPath := filepath.Join(plan.BaseDir, strings.TrimSpace(manifest.AlignedFASTA))
	newickPath := filepath.Join(plan.BaseDir, strings.TrimSpace(manifest.NewickPath))
	if strings.TrimSpace(manifest.AlignedFASTA) == "" || strings.TrimSpace(manifest.NewickPath) == "" {
		return RunResult{}, nil
	}
	aligned, err := os.ReadFile(alignedPath)
	if err != nil {
		return RunResult{}, nil
	}
	newick, err := os.ReadFile(newickPath)
	if err != nil {
		return RunResult{}, nil
	}
	if err := validateRuntimeAlignment(plan, string(aligned)); err != nil {
		return RunResult{}, err
	}
	finalPlan, err := BuildRunPlan(plan.SessionID, plan.RunID, plan.BaseDir, plan.Settings, plan.Kind, plan.Records, plan.Metadata, string(aligned), string(newick), time.Now())
	if err != nil {
		return RunResult{}, err
	}
	finalPlan = AttachRuntimeAudit(finalPlan, "mega-phgo-runtime/reused", time.Now())
	if err := finalPlan.ToArtifactSet().Write(); err != nil {
		return RunResult{}, err
	}
	return RunResult{
		Plan:           finalPlan,
		ArtifactDir:    finalPlan.BaseDir,
		SummaryPath:    findFirstExisting(finalPlan.BaseDir, []string{"runtime-summary.txt", "tree-result_summary.txt", "tree_summary.txt"}),
		SelectedNewick: newickPath,
		Reused:         true,
	}, nil
}

func ValidateRunPlanAlignment(plan RunPlan, alignedFASTA string) error {
	return validateRuntimeAlignment(plan, alignedFASTA)
}

func findAlignedFASTA(dir string, base string) (string, string, error) {
	candidates := []string{
		base + ".fasta",
		base + ".fas",
		filepath.Join(dir, "aligned.fasta"),
		filepath.Join(dir, "aligned.fas"),
	}
	for _, path := range append(candidates, filesWithExt(dir, ".fasta", ".fas")...) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), ">") {
			return path, string(data), nil
		}
	}
	return "", "", fmt.Errorf("tree runtime did not produce an aligned FASTA file in %s", dir)
}

func findNewick(dir string, base string) (string, string, error) {
	candidates := []string{
		base + ".nwk",
		base + ".newick",
		filepath.Join(dir, "tree.nwk"),
	}
	for _, path := range append(candidates, filesWithExt(dir, ".nwk", ".newick", ".tree")...) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(data))
		if looksLikeNewick(text) {
			return path, text, nil
		}
	}
	return "", "", fmt.Errorf("tree runtime did not produce a Newick file in %s", dir)
}

func filesWithExt(dir string, exts ...string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	allowed := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		allowed[strings.ToLower(ext)] = struct{}{}
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if _, ok := allowed[ext]; ok {
			out = append(out, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(out)
	return out
}

func findFirstExisting(dir string, names []string) string {
	for _, name := range names {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func looksLikeNewick(text string) bool {
	text = strings.TrimSpace(text)
	return strings.Contains(text, "(") && strings.Contains(text, ")") && strings.HasSuffix(text, ";")
}

func validateRuntimeAlignment(plan RunPlan, alignedFASTA string) error {
	records, err := parseAlignedFASTA(alignedFASTA)
	if err != nil {
		return err
	}
	if len(records) != len(plan.Records) {
		return fmt.Errorf("mega-phgo-runtime returned %d aligned sequence(s) for %d input record(s)", len(records), len(plan.Records))
	}
	for i, sequence := range records {
		kind := inferSequenceKind(strings.ReplaceAll(sequence, "-", ""))
		if kind == SequenceUnknown {
			return fmt.Errorf("mega-phgo-runtime returned an ambiguous aligned sequence for %s", plan.Records[i].TaxonID)
		}
		switch plan.Kind {
		case SequenceProtein:
			if kind == SequenceNucleotide {
				return fmt.Errorf("mega-phgo-runtime did not convert %s to protein as requested", plan.Records[i].TaxonID)
			}
		case SequenceNucleotide:
			if kind == SequenceProtein {
				return fmt.Errorf("mega-phgo-runtime did not convert %s to DNA as requested", plan.Records[i].TaxonID)
			}
		}
	}
	return nil
}

func parseAlignedFASTA(alignedFASTA string) ([]string, error) {
	lines := strings.Split(strings.ReplaceAll(alignedFASTA, "\r\n", "\n"), "\n")
	sequences := make([]string, 0)
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		sequences = append(sequences, current.String())
		current.Reset()
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ">") {
			flush()
			continue
		}
		current.WriteString(line)
	}
	flush()
	if len(sequences) == 0 {
		return nil, fmt.Errorf("mega-phgo-runtime returned an empty aligned FASTA")
	}
	return sequences, nil
}
