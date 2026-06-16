package main

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptYesNoAcceptsYAndRejectsDefault(t *testing.T) {
	if !promptYesNo(strings.NewReader("y\n"), &bytes.Buffer{}, "prompt: ") {
		t.Fatal("promptYesNo rejected y")
	}
	if promptYesNo(strings.NewReader("\n"), &bytes.Buffer{}, "prompt: ") {
		t.Fatal("promptYesNo accepted empty answer")
	}
}

func TestArchiveEntryTargetNameStripsArchiveRoot(t *testing.T) {
	got, ok := archiveEntryTargetName("phytozome-go_linux_amd64_wezterm/bin/file.txt", "phytozome-go_linux_amd64_wezterm")
	if !ok {
		t.Fatal("archiveEntryTargetName rejected valid entry")
	}
	want := filepath.Join("bin", "file.txt")
	if got != want {
		t.Fatalf("archiveEntryTargetName = %q, want %q", got, want)
	}
}

func TestArchiveEntryTargetNameRejectsOutsidePrefix(t *testing.T) {
	if _, ok := archiveEntryTargetName("../escape.txt", ""); ok {
		t.Fatal("archiveEntryTargetName accepted unsafe entry")
	}
	if _, ok := archiveEntryTargetName("other-root/file.txt", "expected-root"); ok {
		t.Fatal("archiveEntryTargetName accepted mismatched archive root")
	}
}

func TestSafeArchivePathRejectsTraversal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "bundle")
	if _, err := safeArchivePath(root, filepath.Join("..", "escape.txt")); err == nil {
		t.Fatal("safeArchivePath accepted parent traversal")
	}
}

func TestFindReleaseAssetMatchesCaseInsensitiveName(t *testing.T) {
	asset, ok := findReleaseAsset([]githubReleaseAsset{
		{Name: "phytozome-go_windows_amd64_wezterm.zip", BrowserDownloadURL: "https://example.invalid/a.zip"},
	}, "PHYTOZOME-GO_WINDOWS_AMD64_WEZTERM.ZIP")
	if !ok {
		t.Fatal("findReleaseAsset did not match asset")
	}
	if asset.BrowserDownloadURL != "https://example.invalid/a.zip" {
		t.Fatalf("findReleaseAsset returned wrong asset: %#v", asset)
	}
}

func TestShouldAutoAcceptUpdate(t *testing.T) {
	t.Setenv(autoAcceptUpdateEnv, "1")
	if !shouldAutoAcceptUpdate() {
		t.Fatal("shouldAutoAcceptUpdate returned false for enabled env")
	}
	t.Setenv(autoAcceptUpdateEnv, "no")
	if shouldAutoAcceptUpdate() {
		t.Fatal("shouldAutoAcceptUpdate returned true for disabled env")
	}
}

func TestShouldSkipReleaseUpdateCheck(t *testing.T) {
	original := version
	t.Cleanup(func() {
		version = original
	})

	version = ""
	if !shouldSkipReleaseUpdateCheck() {
		t.Fatal("shouldSkipReleaseUpdateCheck returned false for empty version")
	}

	version = "dev"
	if shouldSkipReleaseUpdateCheck() {
		t.Fatal("shouldSkipReleaseUpdateCheck returned true for dev builds")
	}

	version = "v20260531T075742Z"
	if shouldSkipReleaseUpdateCheck() {
		t.Fatal("shouldSkipReleaseUpdateCheck returned true for release builds")
	}
}

func TestUpdateAssetSpecForWindows(t *testing.T) {
	spec, err := updateAssetSpecFor("windows", "amd64")
	if err != nil {
		t.Fatalf("updateAssetSpecFor returned error: %v", err)
	}
	if spec.AssetName != "phytozome-go_windows_amd64_wezterm.zip" {
		t.Fatalf("AssetName = %q", spec.AssetName)
	}
	if spec.ArchiveKind != "zip" {
		t.Fatalf("ArchiveKind = %q", spec.ArchiveKind)
	}
	if spec.OutputRelative != "output" {
		t.Fatalf("OutputRelative = %q", spec.OutputRelative)
	}
	if len(spec.VerifyRelative) == 0 {
		t.Fatal("VerifyRelative is empty")
	}
}

func TestUpdateAssetSpecForMacArm64(t *testing.T) {
	spec, err := updateAssetSpecFor("darwin", "arm64")
	if err != nil {
		t.Fatalf("updateAssetSpecFor returned error: %v", err)
	}
	if spec.AssetName != "phytozome-go_macos_arm64_wezterm.tar.gz" {
		t.Fatalf("AssetName = %q", spec.AssetName)
	}
	if spec.RelaunchRelative != "Contents/MacOS/phytozome-go" {
		t.Fatalf("RelaunchRelative = %q", spec.RelaunchRelative)
	}
	if spec.OutputRelative != "Contents/MacOS/output" {
		t.Fatalf("OutputRelative = %q", spec.OutputRelative)
	}
	if len(spec.VerifyRelative) == 0 {
		t.Fatal("VerifyRelative is empty")
	}
}

func TestMacInstallRootFromExec(t *testing.T) {
	spec, err := updateAssetSpecFor("darwin", "amd64")
	if err != nil {
		t.Fatalf("updateAssetSpecFor returned error: %v", err)
	}
	cleanerPath := filepath.Join("/Applications", "phytozome GO.app", "Contents", "MacOS", "phgohelper.bin")
	got, err := spec.InstallRootFromExec(cleanerPath)
	if err != nil {
		t.Fatalf("InstallRootFromExec returned error: %v", err)
	}
	want := filepath.Join("/Applications", "phytozome GO.app")
	if got != want {
		t.Fatalf("InstallRootFromExec = %q, want %q", got, want)
	}
}

func TestBuildPowerShellUpdaterScriptOmitsEmptyArgumentList(t *testing.T) {
	plan := stagedUpdatePlan{
		InstallRoot:  `C:\bundle`,
		StageDir:     `C:\bundle.update-1`,
		RelaunchPath: `C:\bundle\phytozome-go.exe`,
		Spec:         updateAssetSpec{OutputRelative: "output", VerifyRelative: []string{"phytozome-go.exe", "phgohelper.bin"}},
	}
	script := buildPowerShellUpdaterScript(plan, nil)
	if strings.Contains(script, "-ArgumentList @()") {
		t.Fatalf("buildPowerShellUpdaterScript generated an empty ArgumentList:\n%s", script)
	}
	if !strings.Contains(script, "Start-Process -FilePath $Launcher -WorkingDirectory $WorkingDir") {
		t.Fatalf("buildPowerShellUpdaterScript missing Start-Process line:\n%s", script)
	}
	if !strings.Contains(script, "Copy-PreservedOutputToStage") || !strings.Contains(script, "$OutputRelative = 'output'") {
		t.Fatalf("buildPowerShellUpdaterScript missing output preservation:\n%s", script)
	}
	if !strings.Contains(script, "Assert-KeyFilesUpdated") || !strings.Contains(script, "$VerifyRelative = @('phytozome-go.exe', 'phgohelper.bin')") {
		t.Fatalf("buildPowerShellUpdaterScript missing key-file verification:\n%s", script)
	}
	if !strings.Contains(script, lastUpdateErrorEnv) || !strings.Contains(script, "launching after failed update") || !strings.Contains(script, "[Environment]::SetEnvironmentVariable($LastUpdateErrorEnvName, $LastUpdateError, 'Process')") {
		t.Fatalf("buildPowerShellUpdaterScript missing failed-update relaunch handling:\n%s", script)
	}
	if !strings.Contains(script, "Test-LauncherAlreadyRunning") || !strings.Contains(script, "skip launch after successful update because launcher is already running") {
		t.Fatalf("buildPowerShellUpdaterScript missing duplicate-launch guard:\n%s", script)
	}
	if !strings.Contains(script, updatePendingMarkerName) {
		t.Fatalf("buildPowerShellUpdaterScript missing pending marker cleanup:\n%s", script)
	}
}

func TestBuildPowerShellUpdaterScriptIncludesArgumentsWhenPresent(t *testing.T) {
	plan := stagedUpdatePlan{
		InstallRoot:  `C:\bundle`,
		StageDir:     `C:\bundle.update-1`,
		RelaunchPath: `C:\bundle\phytozome-go.exe`,
		Spec:         updateAssetSpec{OutputRelative: "output", VerifyRelative: []string{"phytozome-go.exe", "phgohelper.bin"}},
	}
	script := buildPowerShellUpdaterScript(plan, []string{"--instance-id", "1"})
	if !strings.Contains(script, "-ArgumentList @('--instance-id', '1')") {
		t.Fatalf("buildPowerShellUpdaterScript missing argument list:\n%s", script)
	}
}

func TestWriteShellUpdaterPreservesOutput(t *testing.T) {
	plan := stagedUpdatePlan{
		InstallRoot:  "/tmp/bundle",
		StageDir:     "/tmp/bundle.update-1",
		BackupDir:    "/tmp/bundle.backup-old",
		RelaunchPath: "/tmp/bundle/phytozome-go",
		Spec:         updateAssetSpec{OutputRelative: "output"},
	}
	path, err := writeShellUpdater(plan, nil)
	if err != nil {
		t.Fatalf("writeShellUpdater returned error: %v", err)
	}
	defer os.Remove(path)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shell updater: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "OUTPUT_REL='output'") {
		t.Fatalf("shell updater missing output relative path:\n%s", text)
	}
	if !strings.Contains(text, "preserve_output") || !strings.Contains(text, "cp -a \"$SOURCE\"/. \"$DEST\"/") {
		t.Fatalf("shell updater missing output preservation:\n%s", text)
	}
	if !strings.Contains(text, "PENDING_MARKER='.phgo-update-pending.json'") || !strings.Contains(text, "rm -f \"$TARGET_DIR/$PENDING_MARKER\"") {
		t.Fatalf("shell updater missing pending marker cleanup:\n%s", text)
	}
}

func TestWriteWindowsVBScriptLauncher(t *testing.T) {
	path, err := writeWindowsVBScriptLauncher(`C:\tmp\update.ps1`)
	if err != nil {
		t.Fatalf("writeWindowsVBScriptLauncher returned error: %v", err)
	}
	defer os.Remove(path)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read launcher: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `Wscript.Shell`) {
		t.Fatalf("launcher content missing Wscript.Shell:\n%s", text)
	}
	if !strings.Contains(text, `powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File ""C:\tmp\update.ps1""`) {
		t.Fatalf("launcher content missing powershell command:\n%s", text)
	}
}

func TestDownloadFileShowsDetailedProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := bytes.Repeat([]byte("a"), 8192)
		w.Header().Set("Content-Length", "8192")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "update.bin")
	var out bytes.Buffer
	if err := downloadFile(context.Background(), server.URL, dest, &out); err != nil {
		t.Fatalf("downloadFile returned error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Downloading application update...") || !strings.Contains(text, "Application update download complete.") {
		t.Fatalf("download progress output missing expected labels:\n%s", text)
	}
	if !strings.Contains(text, "/8.0 KiB") {
		t.Fatalf("download progress output missing size details:\n%s", text)
	}
}

func TestExtractZipArchiveShowsDetailedProgress(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "bundle.zip")
	writeTestZipArchive(t, archivePath, map[string]string{
		"bundle/a.txt": "hello",
		"bundle/b.txt": "world",
	})
	dest := filepath.Join(tmp, "out")
	var out bytes.Buffer
	if err := extractZipArchive(archivePath, "bundle", dest, &out); err != nil {
		t.Fatalf("extractZipArchive returned error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Extracting application update...") || !strings.Contains(text, "Application update extraction complete.") {
		t.Fatalf("extract progress output missing expected labels:\n%s", text)
	}
	if !strings.Contains(text, "2/2 files") {
		t.Fatalf("extract progress output missing file counts:\n%s", text)
	}
}

func writeTestZipArchive(t *testing.T, path string, files map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}
