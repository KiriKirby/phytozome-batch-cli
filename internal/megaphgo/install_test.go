package megaphgo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallManagedUsesBundledRuntimeDirectory(t *testing.T) {
	requireBundledRuntimePlatform(t)
	t.Cleanup(withTempApplicationDir(t))
	toolsDir, err := ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	exe := filepath.Join(toolsDir, executableName(RuntimeExecutable))
	if err := writeProbeRuntime(exe); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, executableName(MuscleExecutable)), []byte("fake muscle"), 0o755); err != nil {
		t.Fatalf("write fake muscle: %v", err)
	}
	binDir, err := InstallManaged(context.Background(), nil)
	if err != nil {
		t.Fatalf("InstallManaged returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(binDir, executableName(RuntimeExecutable))); err != nil {
		t.Fatalf("installed runtime missing: %v", err)
	}
	toolsDir, err = ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	if binDir != toolsDir {
		t.Fatalf("binDir = %q, want exact tools dir %q", binDir, toolsDir)
	}
}

func TestInstallManagedUnsupportedPlatform(t *testing.T) {
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		t.Skip("bundled runtime is supported on windows/amd64")
	}
	t.Cleanup(withTempApplicationDir(t))
	_, err := InstallManaged(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "supported only in the Windows amd64 release") {
		t.Fatalf("InstallManaged error = %v, want unsupported-platform guidance", err)
	}
}

func TestManagedExecutableRequiresExactBundledRuntimeRoot(t *testing.T) {
	requireBundledRuntimePlatform(t)
	t.Cleanup(withTempApplicationDir(t))
	toolsDir, err := ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	nested := filepath.Join(toolsDir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	if err := writeProbeRuntime(filepath.Join(nested, executableName(RuntimeExecutable))); err != nil {
		t.Fatalf("write nested runtime: %v", err)
	}
	if exe, found, err := ManagedExecutable(); err != nil || found || exe != "" {
		t.Fatalf("ManagedExecutable = %q, %v, %v; want no nested runtime", exe, found, err)
	}
	if err := writeProbeRuntime(filepath.Join(toolsDir, executableName(RuntimeExecutable))); err != nil {
		t.Fatalf("write local runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, executableName(MuscleExecutable)), []byte("fake muscle"), 0o755); err != nil {
		t.Fatalf("write local muscle: %v", err)
	}
	if exe, found, err := ManagedExecutable(); err != nil || !found || filepath.Dir(exe) != toolsDir {
		t.Fatalf("ManagedExecutable = %q, %v, %v; want runtime in bundled runtime root", exe, found, err)
	}
}

func TestManagedExecutableRequiresRuntimeOwnedMuscle(t *testing.T) {
	requireBundledRuntimePlatform(t)
	t.Cleanup(withTempApplicationDir(t))
	toolsDir, err := ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	if err := writeProbeRuntime(filepath.Join(toolsDir, executableName(RuntimeExecutable))); err != nil {
		t.Fatalf("write runtime: %v", err)
	}
	if exe, found, err := ManagedExecutable(); err != nil || found || exe != "" {
		t.Fatalf("ManagedExecutable = %q, %v, %v; want missing runtime-owned MUSCLE to reject install", exe, found, err)
	}
}

func TestEnsureRuntimeAvailableIgnoresPathRuntime(t *testing.T) {
	requireBundledRuntimePlatform(t)
	t.Cleanup(withTempApplicationDir(t))
	pathDir := t.TempDir()
	if err := writeProbeRuntime(filepath.Join(pathDir, executableName(RuntimeExecutable))); err != nil {
		t.Fatalf("write PATH runtime: %v", err)
	}
	t.Setenv("PATH", pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	err := EnsureRuntimeAvailable(RuntimeExecutable)
	if err == nil || !IsMissingToolsError(err) {
		t.Fatalf("EnsureRuntimeAvailable error = %v, want MissingToolsError", err)
	}
}

func TestEnsureRuntimeAvailableRequiresProbeAndRuntimeOwnedMuscle(t *testing.T) {
	requireBundledRuntimePlatform(t)
	t.Cleanup(withTempApplicationDir(t))
	toolsDir, err := ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatalf("create tools dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, executableName(RuntimeExecutable)), []byte("not a PHgo runtime"), 0o755); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, executableName(MuscleExecutable)), []byte("fake muscle"), 0o755); err != nil {
		t.Fatalf("write fake muscle: %v", err)
	}
	if err := EnsureRuntimeAvailable(); err == nil || !IsMissingToolsError(err) {
		t.Fatalf("EnsureRuntimeAvailable error = %v, want MissingToolsError for failed probe", err)
	}
	if err := writeProbeRuntime(filepath.Join(toolsDir, executableName(RuntimeExecutable))); err != nil {
		t.Fatalf("write probe runtime: %v", err)
	}
	if err := os.Remove(filepath.Join(toolsDir, executableName(MuscleExecutable))); err != nil {
		t.Fatalf("remove muscle: %v", err)
	}
	if err := EnsureRuntimeAvailable(); err == nil || !IsMissingToolsError(err) {
		t.Fatalf("EnsureRuntimeAvailable error = %v, want MissingToolsError for missing muscle", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, executableName(MuscleExecutable)), []byte("fake muscle"), 0o755); err != nil {
		t.Fatalf("write fake muscle again: %v", err)
	}
	if err := EnsureRuntimeAvailable(); err != nil {
		t.Fatalf("EnsureRuntimeAvailable returned error for valid runtime: %v", err)
	}
}

func TestEnsureRuntimeAvailableRequiredListStillRequiresRuntimeOwnedMuscle(t *testing.T) {
	requireBundledRuntimePlatform(t)
	t.Cleanup(withTempApplicationDir(t))
	toolsDir, err := ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	if err := writeProbeRuntime(filepath.Join(toolsDir, executableName(RuntimeExecutable))); err != nil {
		t.Fatalf("write runtime: %v", err)
	}
	err = EnsureRuntimeAvailable(RuntimeExecutable, MuscleExecutable)
	if err == nil || !IsMissingToolsError(err) {
		t.Fatalf("EnsureRuntimeAvailable required-list error = %v, want MissingToolsError for missing muscle", err)
	}
	if !strings.Contains(err.Error(), MuscleExecutable) {
		t.Fatalf("error = %q, want runtime-owned MUSCLE guidance", err.Error())
	}
	if err := os.WriteFile(filepath.Join(toolsDir, executableName(MuscleExecutable)), []byte("fake muscle"), 0o755); err != nil {
		t.Fatalf("write muscle: %v", err)
	}
	if err := EnsureRuntimeAvailable(RuntimeExecutable, MuscleExecutable); err != nil {
		t.Fatalf("EnsureRuntimeAvailable required list returned error for complete runtime folder: %v", err)
	}
}

func TestPrepareExecutionCopiesRuntimeOwnedMuscleNameExpectedByRuntime(t *testing.T) {
	requireBundledRuntimePlatform(t)
	t.Cleanup(withTempApplicationDir(t))
	toolsDir, err := ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	runtimePath := filepath.Join(toolsDir, executableName(RuntimeExecutable))
	if err := writeProbeRuntime(runtimePath); err != nil {
		t.Fatalf("write runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, executableName(MuscleExecutable)), []byte("fake muscle"), 0o755); err != nil {
		t.Fatalf("write muscle: %v", err)
	}
	preparedRuntime, cleanup, err := PrepareExecution(runtimePath)
	if err != nil {
		t.Fatalf("PrepareExecution returned error: %v", err)
	}
	defer cleanup()
	if filepath.Base(preparedRuntime) != RuntimeExecutable+".exe" {
		t.Fatalf("prepared runtime = %q, want temporary .exe runtime", preparedRuntime)
	}
	preparedDir := filepath.Dir(preparedRuntime)
	if _, err := os.Stat(filepath.Join(preparedDir, "muscleWin64.bin")); err != nil {
		t.Fatalf("temporary runtime directory missing runtime-owned MUSCLE .bin: %v", err)
	}
	if _, err := os.Stat(filepath.Join(preparedDir, "muscleWin64.exe")); err == nil {
		t.Fatalf("temporary runtime directory should not use MEGA runtime-invisible MUSCLE .exe copy")
	}
}

func TestProbeExecutableUsesTemporaryExeForBundledWindowsBin(t *testing.T) {
	requireBundledRuntimePlatform(t)
	t.Cleanup(withTempApplicationDir(t))
	toolsDir, err := ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	runtimePath := filepath.Join(toolsDir, executableName(RuntimeExecutable))
	markerPath := filepath.Join(toolsDir, "probe-argv0.txt")
	if err := writeProbeRuntimeThatRecordsArgv0(t, runtimePath, markerPath); err != nil {
		t.Fatalf("write probe runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, executableName(MuscleExecutable)), []byte("fake muscle"), 0o755); err != nil {
		t.Fatalf("write muscle: %v", err)
	}
	if err := probeExecutable(runtimePath); err != nil {
		t.Fatalf("probeExecutable returned error: %v", err)
	}
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read probe marker: %v", err)
	}
	argv0 := strings.TrimSpace(string(data))
	if !strings.HasSuffix(strings.ToLower(argv0), RuntimeExecutable+".exe") {
		t.Fatalf("probe ran %q, want temporary .exe runtime instead of bundled .bin", argv0)
	}
	if strings.EqualFold(filepath.Clean(argv0), filepath.Clean(runtimePath)) {
		t.Fatalf("probe directly executed bundled .bin path %q", runtimePath)
	}
}

func TestManagedExecutableRejectsRenamedNonPHGORuntime(t *testing.T) {
	requireBundledRuntimePlatform(t)
	t.Cleanup(withTempApplicationDir(t))
	toolsDir, err := ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatalf("create tools dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, executableName(RuntimeExecutable)), []byte("not a PHgo runtime"), 0o755); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	if exe, found, err := ManagedExecutable(); err != nil || found || exe != "" {
		t.Fatalf("ManagedExecutable = %q, %v, %v; want no renamed non-runtime", exe, found, err)
	}
}

func TestExtractArchiveRejectsNativeInstallPackages(t *testing.T) {
	err := extractArchive(context.Background(), filepath.Join(t.TempDir(), "runtime.deb"), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "unsupported PHgo tree runtime archive format") {
		t.Fatalf("deb rejection error = %v", err)
	}
	err = extractArchive(context.Background(), filepath.Join(t.TempDir(), "runtime.pkg"), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "unsupported PHgo tree runtime archive format") {
		t.Fatalf("pkg rejection error = %v", err)
	}
}

func withTempApplicationDir(t *testing.T) func() {
	t.Helper()
	oldExecutable := executableFn
	oldGetwd := getwdFn
	oldTempDir := tempDirFn
	oldProbeExecutable := probeExecutableFn
	root := t.TempDir()
	executableFn = func() (string, error) {
		return filepath.Join(root, "phytozome-go-test"), nil
	}
	getwdFn = func() (string, error) {
		return root, nil
	}
	tempDirFn = func() string {
		return filepath.Join(os.TempDir(), "not-the-test-root")
	}
	probeExecutableFn = func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(data), RuntimeProbeToken) {
			return os.ErrInvalid
		}
		return nil
	}
	return func() {
		executableFn = oldExecutable
		getwdFn = oldGetwd
		tempDirFn = oldTempDir
		probeExecutableFn = oldProbeExecutable
	}
}

func writeProbeRuntime(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(probeRuntimeScript()), 0o755)
}

func writeProbeRuntimeThatRecordsArgv0(t *testing.T, path string, markerPath string) error {
	t.Helper()
	if runtime.GOOS != "windows" {
		return writeProbeRuntime(path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	source := filepath.Join(filepath.Dir(path), "probe-argv0-stub.go")
	code := `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--phgo-runtime-probe" {
		_ = os.WriteFile(os.Getenv("PHGO_PROBE_ARGV0_MARKER"), []byte(os.Args[0]), 0644)
		fmt.Println("mega-phgo-runtime probe ok")
		return
	}
}
`
	if err := os.WriteFile(source, []byte(code), 0o644); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-o", path, source)
	cmd.Env = append(os.Environ(), "PHGO_PROBE_ARGV0_MARKER="+markerPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	_ = out
	t.Setenv("PHGO_PROBE_ARGV0_MARKER", markerPath)
	return nil
}

func probeRuntimeScript() string {
	if runtime.GOOS == "windows" {
		return "@echo off\r\nif \"%1\"==\"--phgo-runtime-probe\" echo mega-phgo-runtime probe ok\r\n"
	}
	return "#!/bin/sh\nif [ \"$1\" = \"--phgo-runtime-probe\" ]; then echo mega-phgo-runtime probe ok; exit 0; fi\nexit 1\n"
}

func requireBundledRuntimePlatform(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("bundled PHgo runtime is supported only on windows/amd64")
	}
}
