package phylo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/megaphgo"
	"github.com/KiriKirby/phytozome-go/internal/model"
)

func TestWriteMegaPHGORuntimeRequest(t *testing.T) {
	records, meta, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		Sequence:     "MPEPTIDE",
		SourceType:   "keyword",
		OriginalHead: "PAL1",
		TableValues:  map[string]string{"label_name": "PAL1"},
	}}, "label_name", "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	plan, err := BuildRunPlan("session", "run1", t.TempDir(), DefaultTreeSettings(), SequenceProtein, records, meta, "", "", time.Now())
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}
	path, err := WriteMegaPHGORuntimeRequest(plan)
	if err != nil {
		t.Fatalf("WriteMegaPHGORuntimeRequest returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	if !strings.Contains(string(data), "\"session_id\": \"session\"") {
		t.Fatalf("request missing session id: %s", data)
	}
	if !strings.Contains(string(data), "\"sequence_kind\": \"protein\"") {
		t.Fatalf("request missing sequence kind: %s", data)
	}
}

func TestBuildMegaPHGORuntimeRequestNormalizesAlignmentMethodForRuntime(t *testing.T) {
	records, meta, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		Sequence:     "MPEPTIDE",
		SourceType:   "keyword",
		OriginalHead: "PAL1",
		TableValues:  map[string]string{"label_name": "PAL1"},
	}}, "label_name", "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	expectedRuntime := map[AlignmentMethod]string{
		AlignmentMethod("clustalw_protein"): "clustalw",
		AlignmentMethod("muscle_protein"):   "muscle",
		AlignmentClustalW:                   "clustalw",
		AlignmentClustalWCodons:             "clustalw_codons",
		AlignmentMUSCLE:                     "muscle",
		AlignmentMUSCLECodons:               "muscle_codons",
	}
	for method, want := range expectedRuntime {
		kind := SequenceProtein
		if strings.Contains(string(method), "codons") || method == AlignmentClustalW || method == AlignmentMUSCLE {
			kind = SequenceNucleotide
		}
		settings := DefaultTreeSettings()
		settings.AlignmentMethod = method
		plan, err := BuildRunPlan("session", "run1", t.TempDir(), settings, kind, records, meta, "", "", time.Now())
		if err != nil {
			t.Fatalf("BuildRunPlan(%s) returned error: %v", method, err)
		}
		request := BuildMegaPHGORuntimeRequest(plan)
		if got := strings.TrimSpace(string(request.Settings.AlignmentMethod)); got != want {
			t.Fatalf("runtime alignment method for %s = %q, want %s", method, got, want)
		}
	}
}

func TestBuildMegaPHGORuntimeRequestCarriesAllEditableMegaDefaults(t *testing.T) {
	records, meta, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		Sequence:     "MPEPTIDE",
		SourceType:   "fasta",
		OriginalHead: "PAL1",
		TableValues:  map[string]string{"label_name": "PAL1"},
	}}, "label_name", "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	for _, kind := range []SequenceKind{SequenceProtein, SequenceNucleotide} {
		t.Run(string(kind)+"/alignment", func(t *testing.T) {
			for _, def := range AlignmentDefinitionsForKind(kind) {
				settings := DefaultTreeSettings()
				if kind == SequenceNucleotide {
					settings.ConversionTarget = ConversionTargetDNA
				}
				settings.AlignmentMethod = AlignmentMethod(def.ID)
				plan, err := BuildRunPlan("session", "run1", t.TempDir(), settings, kind, records, meta, "", "", time.Now())
				if err != nil {
					t.Fatalf("BuildRunPlan(%s) returned error: %v", def.ID, err)
				}
				request := BuildMegaPHGORuntimeRequest(plan)
				assertEditableDefaultsInRequest(t, "alignment "+def.ID, def, request.Settings.AlignmentParams)
			}
		})
		t.Run(string(kind)+"/tree", func(t *testing.T) {
			for _, def := range TreeDefinitionsForKind(kind) {
				settings := DefaultTreeSettings()
				if kind == SequenceNucleotide {
					settings.ConversionTarget = ConversionTargetDNA
				}
				settings.TreeMethod = TreeMethod(def.ID)
				plan, err := BuildRunPlan("session", "run1", t.TempDir(), settings, kind, records, meta, "", "", time.Now())
				if err != nil {
					t.Fatalf("BuildRunPlan(%s) returned error: %v", def.ID, err)
				}
				request := BuildMegaPHGORuntimeRequest(plan)
				assertEditableDefaultsInRequest(t, "tree "+def.ID, def, request.Settings.TreeParams)
			}
		})
	}
}

func TestBuildMegaPHGORuntimeRequestPreservesTreeParameterOverrides(t *testing.T) {
	records, meta, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		Sequence:     "MPEPTIDE",
		SourceType:   "fasta",
		OriginalHead: "PAL1",
		TableValues:  map[string]string{"label_name": "PAL1"},
	}}, "label_name", "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	settings := DefaultTreeSettings()
	settings.TreeMethod = TreeMaximumLikelihood
	settings.TreeParams = map[string]string{
		"phylogeny_test":          "Bootstrap method",
		"bootstrap_replicates":    "37",
		"rates_among_sites":       "Gamma Distributed With Invariant Sites (G+I)",
		"initial_tree_for_ml":     "Make multiple initial trees automatically (Maximum Parsimony)",
		"number_of_initial_trees": "12",
		"branch_swap_filter":      "Strong",
		"number_of_threads":       "1",
	}
	plan, err := BuildRunPlan("session", "run1", t.TempDir(), settings, SequenceProtein, records, meta, "", "", time.Now())
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}
	request := BuildMegaPHGORuntimeRequest(plan)
	for key, want := range settings.TreeParams {
		if got := request.Settings.TreeParams[key]; got != want {
			t.Fatalf("runtime tree param %s = %q, want %q in request %#v", key, got, want, request.Settings.TreeParams)
		}
	}
}

func TestRunPlanWithRuntimeDefaultsToMegaPHGORuntimeStub(t *testing.T) {
	records, meta, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		Sequence:     "MPEPTIDE",
		SourceType:   "keyword",
		OriginalHead: "PAL1",
		TableValues:  map[string]string{"label_name": "PAL1"},
	}}, "label_name", "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	plan, err := BuildRunPlan("session", "run1", t.TempDir(), DefaultTreeSettings(), SequenceProtein, records, meta, "", "", time.Now())
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}
	result, err := RunPlanWithRuntime(context.Background(), plan, RuntimeOptions{})
	if err == nil {
		t.Fatalf("expected source runtime stub error")
	}
	if !strings.Contains(err.Error(), "PHgo runtime files") {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ArtifactDir == "" {
		t.Fatalf("expected artifact dir in result")
	}
}

func assertEditableDefaultsInRequest(t *testing.T, label string, def MethodDefinition, got map[string]string) {
	t.Helper()
	for _, param := range def.Parameters {
		if param.Kind == ParameterSection || param.ReadOnly || strings.TrimSpace(param.ID) == "" {
			continue
		}
		if got[param.ID] != param.Default {
			t.Fatalf("%s runtime request param %s = %q, want MEGA default %q in %#v", label, param.ID, got[param.ID], param.Default, got)
		}
	}
}

func TestMegaPHGORuntimeNormalizesRelativeArtifactDir(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()
	plan := smallRunPlan(t, filepath.Join("relative", "tree"))
	absBaseDir, err := filepath.Abs(filepath.Clean(plan.BaseDir))
	if err != nil {
		t.Fatalf("abs base dir: %v", err)
	}
	plan.BaseDir = absBaseDir
	request := BuildMegaPHGORuntimeRequest(plan)
	for label, path := range map[string]string{
		"base_dir":      request.Artifacts.BaseDir,
		"input_fasta":   request.Artifacts.InputFASTA,
		"metadata_json": request.Artifacts.MetadataJSON,
		"aligned_fasta": request.Artifacts.AlignedFASTA,
		"newick":        request.Artifacts.Newick,
	} {
		if !filepath.IsAbs(path) {
			t.Fatalf("%s path = %q, want absolute path", label, path)
		}
	}
}

func TestMegaPHGORuntimeRejectsPathExecutable(t *testing.T) {
	plan := testRuntimePlan(t)
	result, err := MegaPHGORuntime{Executable: megaphgo.RuntimeExecutable}.Run(context.Background(), plan)
	if err == nil {
		t.Fatalf("expected relative executable rejection")
	}
	if !strings.Contains(err.Error(), "must be absolute") {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ArtifactDir == "" {
		t.Fatalf("expected artifact dir in result")
	}
}

func TestMegaPHGORuntimeRejectsInstalledRuntimeOutsideMegaPHGOFolder(t *testing.T) {
	plan := testRuntimePlan(t)
	exe := filepath.Join(t.TempDir(), runtimeExecutableNameForTest())
	if err := os.WriteFile(exe, []byte("fake"), 0o755); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	_, err := MegaPHGORuntime{Executable: exe}.Run(context.Background(), plan)
	if err == nil || !strings.Contains(err.Error(), "installed MEGA or PATH runtimes are not used") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMegaPHGORuntimeCancellationBeforeRequestWriting(t *testing.T) {
	plan := testRuntimePlan(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := MegaPHGORuntime{}.Run(ctx, plan)
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("MegaPHGORuntime.Run error = %v, want context canceled", err)
	}
	if result.ArtifactDir == "" {
		t.Fatalf("expected artifact dir in cancellation result")
	}
	if _, statErr := os.Stat(filepath.Join(plan.BaseDir, RuntimeRequestFile)); !os.IsNotExist(statErr) {
		t.Fatalf("runtime request should not be written after pre-run cancellation, stat err=%v", statErr)
	}
}

func TestMegaPHGORuntimeCancellationWhileExecutingKeepsRuntimeLogs(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("bundled PHgo runtime execution is currently Windows-only")
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	testAppRoot := t.TempDir()
	if err := os.Chdir(testAppRoot); err != nil {
		t.Fatalf("switch to isolated test app root: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir: %v", err)
		}
	})
	plan := testRuntimePlan(t)
	exe := bundledRuntimePathForTest(t)
	if err := writeRuntimeSleepStub(t, exe); err != nil {
		t.Fatalf("write runtime sleep stub: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(2*time.Second, cancel)
	result, err := MegaPHGORuntime{Executable: exe}.Run(ctx, plan)
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("MegaPHGORuntime.Run error = %v, want context canceled", err)
	}
	if strings.TrimSpace(result.StdoutPath) == "" || strings.TrimSpace(result.StderrPath) == "" {
		t.Fatalf("canceled runtime should keep stdout/stderr artifacts: %#v", result)
	}
	for _, path := range []string{result.StdoutPath, result.StderrPath} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("expected canceled runtime log %s: %v", path, statErr)
		}
	}
}

func TestMegaPHGORuntimeSurfacesRuntimeErrorTextBeforeArtifacts(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	testAppRoot := t.TempDir()
	if err := os.Chdir(testAppRoot); err != nil {
		t.Fatalf("switch to isolated test app root: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working dir: %v", err)
		}
	})
	plan := testRuntimePlan(t)
	exe := bundledRuntimePathForTest(t)
	if err := writeRuntimeErrorTextStub(t, exe, "runtime says no"); err != nil {
		t.Fatalf("write runtime stub: %v", err)
	}
	result, err := MegaPHGORuntime{Executable: exe}.Run(context.Background(), plan)
	if err == nil || !strings.Contains(err.Error(), "runtime says no") {
		t.Fatalf("expected runtime error text, got err=%v result=%#v", err, result)
	}
	if strings.TrimSpace(result.ErrorText) != "runtime says no" {
		t.Fatalf("result error text = %q, want runtime error", result.ErrorText)
	}
}

func TestFindAlignedFASTADoesNotFallbackToInputFASTA(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "input.fasta"), []byte(">input\nAAAA\n"), 0o644); err != nil {
		t.Fatalf("write input fasta: %v", err)
	}
	if _, _, err := findAlignedFASTA(dir, filepath.Join(dir, "aligned")); err == nil {
		t.Fatalf("expected missing aligned FASTA error, got nil")
	}
}

func TestFindNewickDoesNotScanArbitraryTreeFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "old.tree"), []byte("(OLD);"), 0o644); err != nil {
		t.Fatalf("write old tree: %v", err)
	}
	if _, _, err := findNewick(dir, filepath.Join(dir, "tree-result")); err == nil {
		t.Fatalf("expected missing Newick error, got nil")
	}
}

func TestBuildRunPlanArtifactsRemainRuntimeOriented(t *testing.T) {
	records, meta, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		Sequence:     "MPEPTIDE",
		SourceType:   "keyword",
		OriginalHead: "PAL1",
		TableValues:  map[string]string{"label_name": "PAL1"},
	}}, "label_name", "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	plan, err := BuildRunPlan("session", "run1", filepath.Join(t.TempDir(), "tree"), DefaultTreeSettings(), SequenceProtein, records, meta, "aligned", "(PHGOT000001);", time.Now())
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}
	artifact := plan.ToArtifactSet()
	if strings.TrimSpace(artifact.RuntimeRequest) != "" {
		t.Fatalf("unexpected runtime request payload in artifact set")
	}
	if strings.TrimSpace(artifact.RuntimeResponse) != "" {
		t.Fatalf("unexpected runtime response payload in artifact set")
	}
}

func TestReuseRunPlanArtifactsAllowsDisplayNameOnlyRefresh(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	records, meta, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		CanvasRow:    model.CanvasRow{DisplayName: "PAL1"},
		Sequence:     "MPEPTIDE",
		SourceType:   "keyword",
		OriginalHead: "raw header",
		TableValues:  map[string]string{"label_name": "PAL1", "head": "raw header"},
	}}, "label_name", "session", now)
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	plan, err := BuildRunPlan("session", "run1", dir, DefaultTreeSettings(), SequenceProtein, records, meta, "aligned", "(PHGOT000001);", now)
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}
	if err := plan.ToArtifactSet().Write(); err != nil {
		t.Fatalf("write baseline artifacts: %v", err)
	}

	renamedRecords, renamedMeta, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		CanvasRow:    model.CanvasRow{DisplayName: "PAL display"},
		Sequence:     "MPEPTIDE",
		SourceType:   "keyword",
		OriginalHead: "raw header",
		TableValues:  map[string]string{"label_name": "PAL1", "head": "raw header"},
	}}, "label_name", "session", now)
	if err != nil {
		t.Fatalf("BuildInput renamed returned error: %v", err)
	}
	refreshPlan, err := BuildRunPlan("session", "run2", dir, DefaultTreeSettings(), SequenceProtein, renamedRecords, renamedMeta, "", "", now)
	if err != nil {
		t.Fatalf("BuildRunPlan refresh returned error: %v", err)
	}
	reused, err := ReuseRunPlanArtifacts(refreshPlan)
	if err != nil {
		t.Fatalf("ReuseRunPlanArtifacts returned error: %v", err)
	}
	if !reused.Reused {
		t.Fatalf("expected display-name-only refresh to reuse runtime artifacts")
	}
	if reused.Plan.Metadata.Records[0].DisplayName != "PAL display" {
		t.Fatalf("reused payload did not carry updated display name: %#v", reused.Plan.Metadata.Records)
	}
	if reused.Plan.Fingerprints.Alignment != plan.Fingerprints.Alignment || reused.Plan.Fingerprints.Tree != plan.Fingerprints.Tree {
		t.Fatalf("reused plan recompute fingerprints changed: %#v vs %#v", reused.Plan.Fingerprints, plan.Fingerprints)
	}
	if reused.Plan.Fingerprints.Preview == plan.Fingerprints.Preview {
		t.Fatalf("preview fingerprint should change after display-name edit")
	}
	requestPath := filepath.Join(refreshPlan.BaseDir, RuntimeRequestFile)
	if data, err := os.ReadFile(requestPath); err != nil {
		t.Fatalf("reused refresh should write runtime request audit file: %v", err)
	} else if !strings.Contains(string(data), `"session_id": "session"`) {
		t.Fatalf("runtime request audit file should describe the refresh plan: %s", data)
	}
	responsePath := filepath.Join(refreshPlan.BaseDir, RuntimeResponseFile)
	if data, err := os.ReadFile(responsePath); err != nil {
		t.Fatalf("reused refresh should write runtime response audit file: %v", err)
	} else if !strings.Contains(string(data), `"runtime": "mega-phgo-runtime/reused"`) {
		t.Fatalf("runtime response audit file should mark reuse: %s", data)
	}
}

func testRuntimePlan(t *testing.T) RunPlan {
	t.Helper()
	return smallRunPlan(t, t.TempDir())
}

func smallRunPlan(t *testing.T, dir string) RunPlan {
	t.Helper()
	records, meta, err := BuildInput([]RowSource{{
		ItemTitle:    "group 1",
		ItemIndex:    0,
		RowIndex:     0,
		Sequence:     "MPEPTIDE",
		SourceType:   "keyword",
		OriginalHead: "PAL1",
		TableValues:  map[string]string{"label_name": "PAL1"},
	}}, "label_name", "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput returned error: %v", err)
	}
	plan, err := BuildRunPlan("session", "run1", dir, DefaultTreeSettings(), SequenceProtein, records, meta, "", "", time.Now())
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}
	return plan
}

func runtimeExecutableNameForTest() string {
	if runtime.GOOS == "windows" {
		return megaphgo.RuntimeExecutable + ".bin"
	}
	return megaphgo.RuntimeExecutable
}

func bundledRuntimePathForTest(t *testing.T) string {
	t.Helper()
	toolsDir, err := megaphgo.ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	runtimePath := filepath.Join(toolsDir, runtimeExecutableNameForTest())
	musclePath := filepath.Join(toolsDir, muscleExecutableNameForTest())
	for _, path := range []string{runtimePath, musclePath} {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("refusing to overwrite existing bundled test runtime file: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat bundled test runtime file %s: %v", path, err)
		}
	}
	t.Cleanup(func() {
		_ = os.Remove(runtimePath)
		_ = os.Remove(musclePath)
		_ = os.Remove(filepath.Join(toolsDir, "stub.go"))
	})
	if err := os.WriteFile(musclePath, []byte("fake muscle"), 0o755); err != nil {
		t.Fatalf("write fake runtime-owned MUSCLE: %v", err)
	}
	return runtimePath
}

func muscleExecutableNameForTest() string {
	switch runtime.GOOS {
	case "windows":
		return "muscleWin64.bin"
	case "darwin":
		return "muscledarwin64"
	default:
		return "muscleUnix64.exe"
	}
}

func writeRuntimeErrorTextStub(t *testing.T, exe string, errorText string) error {
	t.Helper()
	if runtime.GOOS == "windows" {
		dir := filepath.Dir(exe)
		source := filepath.Join(dir, "stub.go")
		code := fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == %q {
		fmt.Println("mega-phgo-runtime probe ok")
		return
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var request struct {
		Artifacts struct {
			BaseDir string `+"`json:\"base_dir\"`"+`
		} `+"`json:\"artifacts\"`"+`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(request.Artifacts.BaseDir, 0755); err != nil {
		panic(err)
	}
	response := []byte(%q)
	if err := os.WriteFile(filepath.Join(request.Artifacts.BaseDir, "runtime-response.json"), response, 0644); err != nil {
		panic(err)
	}
}
`, megaphgo.RuntimeProbeArgument, fmt.Sprintf(`{"schema_version":1,"runtime":"mega-phgo-runtime","error_text":%q}`, errorText))
		if err := os.WriteFile(source, []byte(code), 0o644); err != nil {
			return err
		}
		cmd := exec.Command("go", "build", "-o", exe, source)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("build runtime stub: %w\n%s", err, out)
		}
		return nil
	}
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"%s\" ]; then echo mega-phgo-runtime probe ok; exit 0; fi\ndir=$(dirname \"$1\")\nprintf '{\"schema_version\":1,\"runtime\":\"mega-phgo-runtime\",\"error_text\":\"%s\"}' > \"$dir/runtime-response.json\"\nexit 0\n", megaphgo.RuntimeProbeArgument, errorText)
	return os.WriteFile(exe, []byte(script), 0o755)
}

func writeRuntimeSleepStub(t *testing.T, exe string) error {
	t.Helper()
	if runtime.GOOS == "windows" {
		dir := filepath.Dir(exe)
		source := filepath.Join(dir, "stub.go")
		code := fmt.Sprintf(`package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == %q {
		fmt.Println("mega-phgo-runtime probe ok")
		return
	}
	fmt.Println("runtime started")
	time.Sleep(30 * time.Second)
}
`, megaphgo.RuntimeProbeArgument)
		if err := os.WriteFile(source, []byte(code), 0o644); err != nil {
			return err
		}
		cmd := exec.Command("go", "build", "-o", exe, source)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("build runtime sleep stub: %w\n%s", err, out)
		}
		return nil
	}
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"%s\" ]; then echo mega-phgo-runtime probe ok; exit 0; fi\necho runtime started\nsleep 30\n", megaphgo.RuntimeProbeArgument)
	return os.WriteFile(exe, []byte(script), 0o755)
}
