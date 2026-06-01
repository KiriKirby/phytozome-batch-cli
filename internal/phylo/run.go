package phylo

import (
	"fmt"
	"os"
	"path/filepath"
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

func findAlignedFASTA(dir string, base string) (string, string, error) {
	candidates := []string{
		base + ".fasta",
		base + ".fas",
		filepath.Join(dir, "aligned.fasta"),
		filepath.Join(dir, "aligned.fas"),
	}
	for _, path := range uniqueExistingCandidates(candidates...) {
		data, err := os.ReadFile(path)
		if err == nil {
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
	for _, path := range uniqueExistingCandidates(candidates...) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return path, strings.TrimSpace(string(data)), nil
	}
	return "", "", fmt.Errorf("tree runtime did not produce a Newick file in %s", dir)
}

func uniqueExistingCandidates(paths ...string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" {
			continue
		}
		key := strings.ToLower(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, path)
	}
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
