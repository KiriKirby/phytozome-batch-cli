package megaphgo

import (
	"archive/zip"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallManagedAcceptsLocalRuntimeDirectory(t *testing.T) {
	t.Cleanup(withTempApplicationDir(t))
	runtimeDir := t.TempDir()
	exe := filepath.Join(runtimeDir, executableName(RuntimeExecutable))
	if err := writeProbeRuntime(exe); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, executableName(MuscleExecutable)), []byte("fake muscle"), 0o755); err != nil {
		t.Fatalf("write fake muscle: %v", err)
	}
	t.Setenv(envDownloadURL, runtimeDir)
	binDir, err := InstallManaged(context.Background(), nil)
	if err != nil {
		t.Fatalf("InstallManaged returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(binDir, executableName(RuntimeExecutable))); err != nil {
		t.Fatalf("installed runtime missing: %v", err)
	}
	toolsDir, err := ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	if binDir != toolsDir {
		t.Fatalf("binDir = %q, want exact tools dir %q", binDir, toolsDir)
	}
}

func TestInstallManagedAcceptsLocalRuntimeZip(t *testing.T) {
	t.Cleanup(withTempApplicationDir(t))
	archivePath := filepath.Join(t.TempDir(), "phytozome-go_mega-phgo-runtime_test.zip")
	if err := writeTestZip(archivePath, map[string]string{
		executableName(RuntimeExecutable): probeRuntimeScript(),
		executableName(MuscleExecutable):  "fake muscle",
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	t.Setenv(envDownloadURL, archivePath)
	binDir, err := InstallManaged(context.Background(), nil)
	if err != nil {
		t.Fatalf("InstallManaged returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(binDir, executableName(RuntimeExecutable))); err != nil {
		t.Fatalf("installed runtime missing: %v", err)
	}
	toolsDir, err := ToolsDir()
	if err != nil {
		t.Fatalf("ToolsDir returned error: %v", err)
	}
	if binDir != toolsDir {
		t.Fatalf("binDir = %q, want exact tools dir %q", binDir, toolsDir)
	}
}

func TestManagedExecutableRequiresExactMegaPHGORuntimeFolder(t *testing.T) {
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
		t.Fatalf("ManagedExecutable = %q, %v, %v; want runtime in tools dir", exe, found, err)
	}
}

func TestManagedExecutableRequiresRuntimeOwnedMuscle(t *testing.T) {
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

func TestManagedExecutableRejectsRenamedNonPHGORuntime(t *testing.T) {
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

func TestAssetNameForPlatformUsesRuntimeZipNames(t *testing.T) {
	name, err := assetNameForPlatform()
	switch runtime.GOOS {
	case "windows":
		if err != nil {
			t.Fatalf("assetNameForPlatform returned error: %v", err)
		}
		if name != currentRuntimeReleaseManifest.Assets["windows-amd64"] {
			t.Fatalf("asset name = %q", name)
		}
	default:
		if err == nil || !strings.Contains(err.Error(), "published only for Windows amd64") {
			t.Fatalf("assetNameForPlatform error = %v, want Windows-only unsupported-platform error", err)
		}
	}
}

func TestResolveDownloadUsesConfiguredReleaseAssetURL(t *testing.T) {
	asset, err := ResolveDownload()
	if runtime.GOOS != "windows" {
		if err == nil || !strings.Contains(err.Error(), "published only for Windows amd64") {
			t.Fatalf("ResolveDownload error = %v, want Windows-only unsupported-platform error", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("ResolveDownload returned error: %v", err)
	}
	if !strings.Contains(asset.URL, "/releases/download/"+currentRuntimeReleaseManifest.ReleaseTag+"/") {
		t.Fatalf("asset URL = %q, want configured release path", asset.URL)
	}
	expectedName, err := assetNameForPlatform()
	if err != nil {
		t.Fatalf("assetNameForPlatform returned error: %v", err)
	}
	if asset.FileName != expectedName {
		t.Fatalf("asset file name = %q, want configured runtime archive name %q", asset.FileName, expectedName)
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

func writeTestZip(path string, files map[string]string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	for name, body := range files {
		entry, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func writeProbeRuntime(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(probeRuntimeScript()), 0o755)
}

func probeRuntimeScript() string {
	if runtime.GOOS == "windows" {
		return "@echo off\r\nif \"%1\"==\"--phgo-runtime-probe\" echo mega-phgo-runtime probe ok\r\n"
	}
	return "#!/bin/sh\nif [ \"$1\" = \"--phgo-runtime-probe\" ]; then echo mega-phgo-runtime probe ok; exit 0; fi\nexit 1\n"
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
