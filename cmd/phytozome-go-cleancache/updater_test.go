package main

import (
	"bytes"
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
}

func TestMacInstallRootFromExec(t *testing.T) {
	spec, err := updateAssetSpecFor("darwin", "amd64")
	if err != nil {
		t.Fatalf("updateAssetSpecFor returned error: %v", err)
	}
	cleanerPath := filepath.Join("/Applications", "phytozome GO.app", "Contents", "MacOS", "phytozome-go-cleancache.bin")
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
	}
	script := buildPowerShellUpdaterScript(plan, nil)
	if strings.Contains(script, "-ArgumentList @()") {
		t.Fatalf("buildPowerShellUpdaterScript generated an empty ArgumentList:\n%s", script)
	}
	if !strings.Contains(script, "Start-Process -FilePath $Launcher -WorkingDirectory $WorkingDir") {
		t.Fatalf("buildPowerShellUpdaterScript missing Start-Process line:\n%s", script)
	}
}

func TestBuildPowerShellUpdaterScriptIncludesArgumentsWhenPresent(t *testing.T) {
	plan := stagedUpdatePlan{
		InstallRoot:  `C:\bundle`,
		StageDir:     `C:\bundle.update-1`,
		RelaunchPath: `C:\bundle\phytozome-go.exe`,
	}
	script := buildPowerShellUpdaterScript(plan, []string{"--instance-id", "1"})
	if !strings.Contains(script, "-ArgumentList @('--instance-id', '1')") {
		t.Fatalf("buildPowerShellUpdaterScript missing argument list:\n%s", script)
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
