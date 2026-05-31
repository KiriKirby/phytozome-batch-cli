package phylo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/megaphgo"
)

const RuntimeRequestFile = "runtime-request.json"
const RuntimeResponseFile = "runtime-response.json"

type MegaPHGORuntime struct {
	Executable string
}

func (MegaPHGORuntime) Name() string {
	return "mega-phgo-runtime"
}

func (r MegaPHGORuntime) Run(ctx context.Context, plan RunPlan) (RunResult, error) {
	if strings.TrimSpace(plan.BaseDir) == "" {
		return RunResult{}, fmt.Errorf("tree artifact directory is empty")
	}
	absBaseDir, err := filepath.Abs(filepath.Clean(plan.BaseDir))
	if err != nil {
		return RunResult{Plan: plan, ArtifactDir: plan.BaseDir, ErrorText: err.Error()}, err
	}
	plan.BaseDir = absBaseDir
	if reused, err := ReuseRunPlanArtifacts(plan); err != nil {
		return RunResult{}, err
	} else if reused.Reused {
		return reused, nil
	}
	if err := plan.ToArtifactSet().Write(); err != nil {
		return RunResult{}, err
	}
	requestPath, err := WriteMegaPHGORuntimeRequest(plan)
	if err != nil {
		return RunResult{}, err
	}
	exe := strings.TrimSpace(r.Executable)
	if exe == "" {
		if managed, found, err := megaphgo.ManagedExecutable(); err != nil {
			return RunResult{Plan: plan, ArtifactDir: plan.BaseDir, ErrorText: err.Error()}, err
		} else if found {
			exe = managed
		} else {
			toolsDir, _ := megaphgo.ToolsDir()
			err := &megaphgo.MissingToolsError{Tools: []string{megaphgo.RuntimeExecutable}, RuntimeDir: toolsDir}
			return RunResult{Plan: plan, ArtifactDir: plan.BaseDir, ErrorText: err.Error()}, err
		}
	} else if err := validateLocalRuntimeExecutable(exe); err != nil {
		return RunResult{Plan: plan, ArtifactDir: plan.BaseDir, ErrorText: err.Error()}, err
	}
	cmd := exec.CommandContext(ctx, exe, requestPath)
	cmd.Dir = plan.BaseDir
	stdoutPath, stderrPath, exitText, runErr := runMegaPHGORuntimeCommand(cmd, plan.BaseDir)
	responsePath := filepath.Join(plan.BaseDir, RuntimeResponseFile)
	response, readErr := readMegaPHGORuntimeResponse(responsePath)
	if readErr != nil {
		return RunResult{Plan: plan, ArtifactDir: plan.BaseDir, StdoutPath: stdoutPath, StderrPath: stderrPath, ErrorText: readErr.Error()}, readErr
	}
	if responseErr := strings.TrimSpace(response.ErrorText); responseErr != "" {
		err := fmt.Errorf("%s", responseErr)
		return RunResult{Plan: plan, ArtifactDir: plan.BaseDir, StdoutPath: stdoutPath, StderrPath: stderrPath, ErrorText: responseErr, SkippedRecords: append([]RuntimeSkippedRecord(nil), response.SkippedRecords...)}, err
	}
	if runErr != nil {
		return RunResult{Plan: plan, ArtifactDir: plan.BaseDir, StdoutPath: stdoutPath, StderrPath: stderrPath, ErrorText: strings.TrimSpace(exitText), SkippedRecords: append([]RuntimeSkippedRecord(nil), response.SkippedRecords...)}, runErr
	}
	_, alignedFASTA, err := findAlignedFASTA(plan.BaseDir, filepath.Join(plan.BaseDir, "aligned"))
	if err != nil {
		return RunResult{Plan: plan, ArtifactDir: plan.BaseDir, StdoutPath: stdoutPath, StderrPath: stderrPath, ErrorText: err.Error()}, err
	}
	if err := validateRuntimeAlignment(plan, alignedFASTA); err != nil {
		return RunResult{Plan: plan, ArtifactDir: plan.BaseDir, StdoutPath: stdoutPath, StderrPath: stderrPath, ErrorText: err.Error(), SkippedRecords: append([]RuntimeSkippedRecord(nil), response.SkippedRecords...)}, err
	}
	newickPath, newick, err := findNewick(plan.BaseDir, filepath.Join(plan.BaseDir, "tree-result"))
	if err != nil {
		return RunResult{Plan: plan, ArtifactDir: plan.BaseDir, StdoutPath: stdoutPath, StderrPath: stderrPath, ErrorText: err.Error()}, err
	}
	finalPlan, err := BuildRunPlan(plan.SessionID, plan.RunID, plan.BaseDir, plan.Settings, plan.Kind, plan.Records, plan.Metadata, alignedFASTA, newick, time.Now())
	if err != nil {
		return RunResult{}, err
	}
	requestJSON, _ := json.MarshalIndent(BuildMegaPHGORuntimeRequest(finalPlan), "", "  ")
	response.Artifacts = BuildMegaPHGORuntimeRequest(finalPlan).Artifacts
	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	finalPlan.RuntimeRequest = string(requestJSON)
	finalPlan.RuntimeResponse = string(responseJSON)
	if err := finalPlan.ToArtifactSet().Write(); err != nil {
		return RunResult{}, err
	}
	return RunResult{
		Plan:           finalPlan,
		ArtifactDir:    finalPlan.BaseDir,
		StdoutPath:     stdoutPath,
		StderrPath:     stderrPath,
		SummaryPath:    findFirstExisting(plan.BaseDir, []string{"runtime-summary.txt"}),
		SelectedNewick: newickPath,
		ErrorText:      strings.TrimSpace(exitText),
		SkippedRecords: append([]RuntimeSkippedRecord(nil), response.SkippedRecords...),
	}, runErr
}

func validateLocalRuntimeExecutable(exe string) error {
	if !filepath.IsAbs(exe) {
		return fmt.Errorf("PHgo tree runtime executable path must be absolute")
	}
	toolsDir, err := megaphgo.ToolsDir()
	if err != nil {
		return err
	}
	absExe, err := filepath.Abs(filepath.Clean(exe))
	if err != nil {
		return err
	}
	absToolsDir, err := filepath.Abs(filepath.Clean(toolsDir))
	if err != nil {
		return err
	}
	expected := filepath.Join(absToolsDir, runtimeExecutableName())
	if !samePath(absExe, expected) {
		return fmt.Errorf("PHgo tree runtime executable must be %s; installed MEGA or PATH runtimes are not used", expected)
	}
	if info, statErr := os.Stat(absExe); statErr != nil || info.IsDir() {
		return &megaphgo.MissingToolsError{Tools: []string{megaphgo.RuntimeExecutable}, RuntimeDir: absToolsDir}
	}
	return megaphgo.EnsureRuntimeAvailable(megaphgo.RuntimeExecutable, megaphgo.MuscleExecutable)
}

func runtimeExecutableName() string {
	if runtime.GOOS == "windows" {
		return megaphgo.RuntimeExecutable + ".exe"
	}
	return megaphgo.RuntimeExecutable
}

func samePath(a string, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

type RuntimePaths struct {
	BaseDir      string `json:"base_dir"`
	InputFASTA   string `json:"input_fasta"`
	MetadataJSON string `json:"metadata_json"`
	AlignedFASTA string `json:"aligned_fasta"`
	Newick       string `json:"newick"`
	Summary      string `json:"summary"`
	RuntimeLog   string `json:"runtime_log"`
}

type MegaPHGORuntimeRequest struct {
	SchemaVersion int           `json:"schema_version"`
	SessionID     string        `json:"session_id"`
	RunID         string        `json:"run_id"`
	CreatedAt     time.Time     `json:"created_at"`
	SequenceKind  SequenceKind  `json:"sequence_kind"`
	Settings      TreeSettings  `json:"settings"`
	Records       []InputRecord `json:"records"`
	InputFASTA    string        `json:"input_fasta"`
	Artifacts     RuntimePaths  `json:"artifacts"`
}

type MegaPHGORuntimeResponse struct {
	SchemaVersion  int                    `json:"schema_version"`
	Runtime        string                 `json:"runtime"`
	CompletedAt    time.Time              `json:"completed_at"`
	Artifacts      RuntimePaths           `json:"artifacts"`
	SkippedRecords []RuntimeSkippedRecord `json:"skipped_records,omitempty"`
	ErrorText      string                 `json:"error_text,omitempty"`
}

func AttachRuntimeAudit(plan RunPlan, runtimeName string, completedAt time.Time) RunPlan {
	request := BuildMegaPHGORuntimeRequest(plan)
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	runtimeName = strings.TrimSpace(runtimeName)
	if runtimeName == "" {
		runtimeName = "mega-phgo-runtime"
	}
	response := MegaPHGORuntimeResponse{
		SchemaVersion: 1,
		Runtime:       runtimeName,
		CompletedAt:   completedAt,
		Artifacts:     request.Artifacts,
	}
	requestJSON, _ := json.MarshalIndent(request, "", "  ")
	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	plan.RuntimeRequest = string(requestJSON)
	plan.RuntimeResponse = string(responseJSON)
	return plan
}

func BuildMegaPHGORuntimeRequest(plan RunPlan) MegaPHGORuntimeRequest {
	settings := normalizeRuntimeTreeSettings(NormalizeTreeSettingsForKind(plan.Settings, plan.Kind), plan.Kind)
	return MegaPHGORuntimeRequest{
		SchemaVersion: 1,
		SessionID:     strings.TrimSpace(plan.SessionID),
		RunID:         strings.TrimSpace(plan.RunID),
		CreatedAt:     plan.UpdatedAt,
		SequenceKind:  plan.Kind,
		Settings:      settings,
		Records:       append([]InputRecord(nil), plan.Records...),
		InputFASTA:    plan.InputFASTA,
		Artifacts: RuntimePaths{
			BaseDir:      plan.BaseDir,
			InputFASTA:   filepath.Join(plan.BaseDir, "input.fasta"),
			MetadataJSON: filepath.Join(plan.BaseDir, "input.meta.json"),
			AlignedFASTA: filepath.Join(plan.BaseDir, "aligned.fasta"),
			Newick:       filepath.Join(plan.BaseDir, "tree.nwk"),
			Summary:      filepath.Join(plan.BaseDir, "runtime-summary.txt"),
			RuntimeLog:   filepath.Join(plan.BaseDir, "runtime.log"),
		},
	}
}

func normalizeRuntimeTreeSettings(settings TreeSettings, kind SequenceKind) TreeSettings {
	settings = NormalizeTreeSettingsForKind(settings, kind)
	if def, ok := MethodDefinitionForAlignmentKind(settings.AlignmentMethod, kind); ok {
		if strings.TrimSpace(def.RuntimeMethod) != "" {
			settings.AlignmentMethod = AlignmentMethod(strings.TrimSpace(def.RuntimeMethod))
		}
	}
	if def, ok := MethodDefinitionForTree(settings.TreeMethod); ok {
		if strings.TrimSpace(def.RuntimeMethod) != "" {
			settings.TreeMethod = TreeMethod(strings.TrimSpace(def.RuntimeMethod))
		}
	}
	return settings
}

func WriteMegaPHGORuntimeRequest(plan RunPlan) (string, error) {
	if strings.TrimSpace(plan.BaseDir) == "" {
		return "", fmt.Errorf("source runtime request directory is empty")
	}
	if err := os.MkdirAll(plan.BaseDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(plan.BaseDir, RuntimeRequestFile)
	data, err := json.MarshalIndent(BuildMegaPHGORuntimeRequest(plan), "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o644)
}

func runMegaPHGORuntimeCommand(cmd *exec.Cmd, dir string) (stdoutPath string, stderrPath string, exitText string, err error) {
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		exitText = err.Error()
	}
	write := func(name string, value string) (string, error) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			return "", err
		}
		return path, nil
	}
	stdoutPath, err2 := write("runtime.stdout.txt", stdout.String())
	if err2 != nil {
		return "", "", exitText, err2
	}
	stderrPath, err2 = write("runtime.stderr.txt", stderr.String())
	if err2 != nil {
		return "", "", exitText, err2
	}
	return stdoutPath, stderrPath, exitText, err
}

func readMegaPHGORuntimeResponse(path string) (MegaPHGORuntimeResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MegaPHGORuntimeResponse{}, err
	}
	var response MegaPHGORuntimeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return MegaPHGORuntimeResponse{}, err
	}
	return response, nil
}
