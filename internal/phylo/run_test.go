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
	if !strings.Contains(err.Error(), "mega-phgo-runtime folder") {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ArtifactDir == "" {
		t.Fatalf("expected artifact dir in result")
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

func TestValidateRuntimeAlignmentRejectsWrongOutputKind(t *testing.T) {
	recordsProtein, metaProtein, err := BuildInput([]RowSource{{
		ItemTitle:    "protein",
		ItemIndex:    0,
		RowIndex:     0,
		Sequence:     "MPEPTIDE",
		SequenceKind: SequenceProtein,
		SourceType:   "fasta",
		OriginalHead: "protein-1",
		TableValues:  map[string]string{"label_name": "protein-1"},
	}}, "label_name", "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput protein returned error: %v", err)
	}
	proteinPlan, err := BuildRunPlan("session", "run1", t.TempDir(), DefaultTreeSettings(), SequenceProtein, recordsProtein, metaProtein, "", "", time.Now())
	if err != nil {
		t.Fatalf("BuildRunPlan protein returned error: %v", err)
	}
	if err := validateRuntimeAlignment(proteinPlan, ">PHGOT000001\nATGCATGC\n"); err == nil || !strings.Contains(err.Error(), "did not convert PHGOT000001 to protein") {
		t.Fatalf("protein validation error = %v", err)
	}

	recordsDNA, metaDNA, err := BuildInput([]RowSource{{
		ItemTitle:    "dna",
		ItemIndex:    0,
		RowIndex:     0,
		Sequence:     "ATGCATGC",
		SequenceKind: SequenceNucleotide,
		SourceType:   "fasta",
		OriginalHead: "dna-1",
		TableValues:  map[string]string{"label_name": "dna-1"},
	}}, "label_name", "session", time.Now())
	if err != nil {
		t.Fatalf("BuildInput dna returned error: %v", err)
	}
	settings := DefaultTreeSettings()
	settings.ConversionTarget = ConversionTargetDNA
	dnaPlan, err := BuildRunPlan("session", "run1", t.TempDir(), settings, SequenceNucleotide, recordsDNA, metaDNA, "", "", time.Now())
	if err != nil {
		t.Fatalf("BuildRunPlan dna returned error: %v", err)
	}
	if err := validateRuntimeAlignment(dnaPlan, ">PHGOT000001\nMPEPTIDE\n"); err == nil || !strings.Contains(err.Error(), "did not convert PHGOT000001 to DNA") {
		t.Fatalf("dna validation error = %v", err)
	}
}

func TestMegaPHGORuntimeRejectsWrongKindAfterRuntimeExecution(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir app root: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()
	runtimeDir := filepath.Join(root, "mega-phgo-runtime")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatalf("mkdir runtime dir: %v", err)
	}
	exe := filepath.Join(runtimeDir, runtimeExecutableNameForTest())
	if err := writeRuntimeWrongKindStub(t, exe); err != nil {
		t.Fatalf("write runtime stub: %v", err)
	}
	muscle := "muscleUnix64.exe"
	if runtime.GOOS == "windows" {
		muscle = "muscleWin64.exe"
	} else if runtime.GOOS == "darwin" {
		muscle = "muscledarwin64"
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, muscle), []byte("stub"), 0o755); err != nil {
		t.Fatalf("write muscle stub: %v", err)
	}

	plan := smallRunPlan(t, filepath.Join(root, "tree"))
	result, err := MegaPHGORuntime{}.Run(context.Background(), plan)
	if err == nil {
		t.Fatalf("expected wrong-kind runtime output to be rejected")
	}
	if !strings.Contains(err.Error(), "did not convert PHGOT000001 to protein") {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if result.ArtifactDir == "" || result.ErrorText == "" {
		t.Fatalf("expected rejected runtime result to keep artifact/error context: %#v", result)
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

func TestMegaPHGORuntimeReturnsRuntimeErrorTextBeforeArtifactLookup(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir app root: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()
	runtimeDir := filepath.Join(root, "mega-phgo-runtime")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatalf("mkdir runtime dir: %v", err)
	}
	exe := filepath.Join(runtimeDir, runtimeExecutableNameForTest())
	if err := writeRuntimeErrorTextStub(t, exe, "runtime-specific failure"); err != nil {
		t.Fatalf("write runtime stub: %v", err)
	}
	muscle := "muscleUnix64.exe"
	if runtime.GOOS == "windows" {
		muscle = "muscleWin64.exe"
	} else if runtime.GOOS == "darwin" {
		muscle = "muscledarwin64"
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, muscle), []byte("stub"), 0o755); err != nil {
		t.Fatalf("write muscle stub: %v", err)
	}

	plan := smallRunPlan(t, filepath.Join(root, "tree"))
	result, err := MegaPHGORuntime{}.Run(context.Background(), plan)
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if !strings.Contains(err.Error(), "runtime-specific failure") {
		t.Fatalf("expected runtime error text, got: %v", err)
	}
	if strings.Contains(err.Error(), "Newick") || strings.Contains(err.Error(), "aligned") {
		t.Fatalf("runtime error should not be masked by artifact lookup: %v", err)
	}
	if result.ErrorText != "runtime-specific failure" {
		t.Fatalf("result.ErrorText = %q", result.ErrorText)
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
		return megaphgo.RuntimeExecutable + ".exe"
	}
	return megaphgo.RuntimeExecutable
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

func writeRuntimeWrongKindStub(t *testing.T, exe string) error {
	t.Helper()
	if runtime.GOOS == "windows" {
		dir := filepath.Dir(exe)
		source := filepath.Join(dir, "stub_wrong_kind.go")
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
			BaseDir      string `+"`json:\"base_dir\"`"+`
			AlignedFASTA string `+"`json:\"aligned_fasta\"`"+`
			Newick       string `+"`json:\"newick\"`"+`
		} `+"`json:\"artifacts\"`"+`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(request.Artifacts.BaseDir, 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(request.Artifacts.BaseDir, "runtime-response.json"), []byte(`+"`"+`{"schema_version":1,"runtime":"mega-phgo-runtime"}`+"`"+`), 0644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(request.Artifacts.AlignedFASTA, []byte(">PHGOT000001\nATGCATGC\n"), 0644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(request.Artifacts.Newick, []byte("(PHGOT000001);"), 0644); err != nil {
		panic(err)
	}
}
`, megaphgo.RuntimeProbeArgument)
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
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"%s\" ]; then echo mega-phgo-runtime probe ok; exit 0; fi\ndir=$(dirname \"$1\")\nprintf '{\"schema_version\":1,\"runtime\":\"mega-phgo-runtime\"}' > \"$dir/runtime-response.json\"\nprintf '>PHGOT000001\\nATGCATGC\\n' > \"$dir/aligned.fasta\"\nprintf '(PHGOT000001);' > \"$dir/tree.nwk\"\nexit 0\n", megaphgo.RuntimeProbeArgument)
	return os.WriteFile(exe, []byte(script), 0o755)
}
