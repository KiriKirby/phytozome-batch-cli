package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveMainProgramPathFromFindsSiblingBinary(t *testing.T) {
	tmp := t.TempDir()
	cleanerPath := filepath.Join(tmp, "phgohelper.bin")
	mainPath := filepath.Join(tmp, mainProgramName)

	if err := os.WriteFile(cleanerPath, []byte("cleaner"), 0o644); err != nil {
		t.Fatalf("write cleaner: %v", err)
	}
	if err := os.WriteFile(mainPath, []byte("main"), 0o644); err != nil {
		t.Fatalf("write main binary: %v", err)
	}

	got, err := resolveMainProgramPathFrom(cleanerPath)
	if err != nil {
		t.Fatalf("resolveMainProgramPathFrom returned error: %v", err)
	}
	if got != mainPath {
		t.Fatalf("resolveMainProgramPathFrom = %q, want %q", got, mainPath)
	}
}

func TestResolveMainProgramPathFromErrorsWhenSiblingBinaryMissing(t *testing.T) {
	tmp := t.TempDir()
	cleanerPath := filepath.Join(tmp, "phgohelper.bin")

	if err := os.WriteFile(cleanerPath, []byte("cleaner"), 0o644); err != nil {
		t.Fatalf("write cleaner: %v", err)
	}

	_, err := resolveMainProgramPathFrom(cleanerPath)
	if err == nil {
		t.Fatal("resolveMainProgramPathFrom returned nil error, want missing binary error")
	}
}

func TestResolveCacheTargetsFromIncludesBundleAndWorkingDirectory(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	bundleDir := filepath.Join(repoRoot, "bin", "phytozome-go_windows_amd64_wezterm")
	cleanerPath := filepath.Join(bundleDir, "phgohelper.bin")
	workingDir := filepath.Join(tmp, "separate-workdir")

	for _, dir := range []string{repoRoot, bundleDir, workingDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	got := resolveCacheTargetsFrom(cleanerPath, workingDir)
	want := []string{
		filepath.Join(repoRoot, ".cache"),
		filepath.Join(bundleDir, ".cache"),
		filepath.Join(workingDir, ".cache"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveCacheTargetsFrom = %#v, want %#v", got, want)
	}
}

func TestResolveCacheTargetsFromDeduplicatesSameDirectory(t *testing.T) {
	tmp := t.TempDir()
	cleanerPath := filepath.Join(tmp, "phgohelper.bin")

	got := resolveCacheTargetsFrom(cleanerPath, tmp)
	want := []string{filepath.Join(tmp, ".cache")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveCacheTargetsFrom = %#v, want %#v", got, want)
	}
}

func TestResolveWezTermCLIPathFromFindsSiblingCLI(t *testing.T) {
	tmp := t.TempDir()
	cleanerPath := filepath.Join(tmp, "phgohelper.bin")
	cliPath := filepath.Join(tmp, windowsWezTermCLIName)

	if err := os.WriteFile(cleanerPath, []byte("cleaner"), 0o644); err != nil {
		t.Fatalf("write cleaner: %v", err)
	}
	if err := os.WriteFile(cliPath, []byte("cli"), 0o644); err != nil {
		t.Fatalf("write cli: %v", err)
	}

	got, err := resolveWezTermCLIPathFrom(cleanerPath)
	if err != nil {
		t.Fatalf("resolveWezTermCLIPathFrom returned error: %v", err)
	}
	if got != cliPath {
		t.Fatalf("resolveWezTermCLIPathFrom = %q, want %q", got, cliPath)
	}
}

func TestBuildWezTermSpawnArgsUsesMainPathAndAppArgs(t *testing.T) {
	mainPath := filepath.Join(`C:\bundle`, mainProgramName)
	got := buildWezTermSpawnArgs(mainPath, []string{"--instance-id", "1", "", "blast"})
	want := []string{
		"cli",
		"spawn",
		"--cwd", filepath.Dir(mainPath),
		"--",
		mainPath,
		"--instance-id",
		"1",
		"blast",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildWezTermSpawnArgs = %#v, want %#v", got, want)
	}
}

func TestShouldSkipBundleMaintenancePreflight(t *testing.T) {
	t.Setenv(skipBundlePreflightEnv, "")
	if shouldSkipBundleMaintenancePreflight() {
		t.Fatal("shouldSkipBundleMaintenancePreflight returned true for empty env")
	}
	t.Setenv(skipBundlePreflightEnv, "1")
	if !shouldSkipBundleMaintenancePreflight() {
		t.Fatal("shouldSkipBundleMaintenancePreflight returned false for set env")
	}
}

func TestShouldSpawnMainProgramInNewTabOnlyInsideWezTerm(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "1")
	if !shouldSpawnMainProgramInNewTab() {
		t.Fatal("expected helper to detect WezTerm pane")
	}
	t.Setenv("WEZTERM_PANE", "")
	if shouldSpawnMainProgramInNewTab() {
		t.Fatal("expected direct launch outside WezTerm")
	}
}

func TestConsoleProgressFinishKeepsLastDetailedLineOnFailure(t *testing.T) {
	var out bytes.Buffer
	progress := newConsoleProgress(&out)
	progress.Update("Downloading prebuilt symbol name database... | 42.1% | 1.2 GiB/2.8 GiB | 25.8 MiB/s | 3 workers", false)
	progress.Finish("done", false)
	text := out.String()
	if !strings.Contains(text, "42.1%") || !strings.Contains(text, "25.8 MiB/s") || !strings.Contains(text, "3 workers") {
		t.Fatalf("console progress output lost detailed line:\n%s", text)
	}
	if strings.Contains(text, "done") {
		t.Fatalf("console progress unexpectedly wrote success message on failure:\n%s", text)
	}
}

func TestStartupDownloadPromptTextIncludesDownloadingStatusAndPrompt(t *testing.T) {
	got := startupDownloadPromptText("Open phytozome GO while the symbol name library downloads? [y/N]:")
	if !strings.Contains(got, "Downloading in background.") {
		t.Fatalf("startupDownloadPromptText missing background status: %q", got)
	}
	if !strings.Contains(got, "Open phytozome GO while the symbol name library downloads? [y/N]:") {
		t.Fatalf("startupDownloadPromptText missing prompt: %q", got)
	}
}

func TestConsoleProgressSingleLineRefreshUsesCarriageReturn(t *testing.T) {
	var out bytes.Buffer
	progress := newConsoleProgress(&out)
	progress.Update("Downloading prebuilt symbol name database... | 68.8% | 1.9 GiB/2.8 GiB | 89.1 MiB/s | 3 workers", false)
	progress.Update("Downloading prebuilt symbol name database... | 69.7% | 2.0 GiB/2.8 GiB | 88.9 MiB/s | 3 workers", false)
	text := out.String()
	if strings.Count(text, "\n") != 0 {
		t.Fatalf("console progress unexpectedly wrote newline during refresh:\n%s", text)
	}
	if strings.Count(text, "\r") < 2 {
		t.Fatalf("console progress did not use carriage returns for single-line refresh:\n%s", text)
	}
}

func TestUpdatePendingMarkerRoundTrip(t *testing.T) {
	appDir := t.TempDir()
	if err := writeUpdatePendingMarker(appDir, "v1"); err != nil {
		t.Fatalf("writeUpdatePendingMarker returned error: %v", err)
	}
	marker, ok := readUpdatePendingMarker(appDir)
	if !ok {
		t.Fatal("readUpdatePendingMarker returned false")
	}
	if marker.Version != "v1" {
		t.Fatalf("marker version = %q, want v1", marker.Version)
	}
	if err := removeUpdatePendingMarker(appDir); err != nil {
		t.Fatalf("removeUpdatePendingMarker returned error: %v", err)
	}
	if _, ok := readUpdatePendingMarker(appDir); ok {
		t.Fatal("marker still exists after removal")
	}
}

func TestUpdatePendingMarkerMessageRoundTrip(t *testing.T) {
	appDir := t.TempDir()
	if err := writeUpdatePendingMarker(appDir, "v1"); err != nil {
		t.Fatalf("writeUpdatePendingMarker returned error: %v", err)
	}
	if err := updatePendingMarkerMessage(appDir, "Applying the new application files..."); err != nil {
		t.Fatalf("updatePendingMarkerMessage returned error: %v", err)
	}
	marker, ok := readUpdatePendingMarker(appDir)
	if !ok {
		t.Fatal("readUpdatePendingMarker returned false")
	}
	if marker.Message != "Applying the new application files..." {
		t.Fatalf("marker message = %q", marker.Message)
	}
}
