package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/startupstate"
	"golang.org/x/term"
)

const githubLatestReleaseAPI = "https://api.github.com/repos/KiriKirby/phytozome-go/releases/latest"

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type updateAssetSpec struct {
	AssetName           string
	ArchiveKind         string
	StripPrefix         string
	RelaunchRelative    string
	OutputRelative      string
	PreserveRelative    []string
	VerifyRelative      []string
	InstallRootFromExec func(string) (string, error)
}

type stagedUpdatePlan struct {
	CurrentVersion string
	LatestVersion  string
	Asset          githubReleaseAsset
	Spec           updateAssetSpec
	CleanerPath    string
	InstallRoot    string
	RelaunchPath   string
	StageDir       string
	BackupDir      string
}

func maybeHandleReleaseUpdate(appDir string, args []string) bool {
	if shouldSkipReleaseUpdateCheck() {
		return false
	}
	currentVersion := strings.TrimSpace(version)
	appendUpdateDebugLog("release update check start")
	appendUpdateDebugLog("current build version: " + currentVersion)
	if currentVersion == "" {
		currentVersion = "unknown"
	}
	_, _ = fmt.Fprintf(os.Stdout, "Checking for updates on GitHub (%s)...\n", currentVersion)
	writeStartupStatus(appDir, startupstate.StatusInitializing, false, "Checking GitHub for application updates.", "")

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		appendUpdateDebugLog("release update check skipped: " + err.Error())
		_, _ = fmt.Fprintf(os.Stdout, "Update check skipped: %s\n", err.Error())
		return false
	}
	appendUpdateDebugLog("latest release tag: " + strings.TrimSpace(release.TagName))

	plan, hasUpdate, err := planStagedUpdate(release, version)
	if err != nil {
		appendUpdateDebugLog("release update planning skipped: " + err.Error())
		_, _ = fmt.Fprintf(os.Stdout, "Update check skipped: %s\n", err.Error())
		return false
	}
	if !hasUpdate {
		appendUpdateDebugLog("already on latest release")
		writeStartupStatus(appDir, startupstate.StatusInitializing, false, "Application update check complete.", "")
		_, _ = fmt.Fprintln(os.Stdout, "Update check: already on the latest release.")
		return false
	}
	if shouldAutoAcceptUpdate() {
		appendUpdateDebugLog("auto-accept update enabled")
		_, _ = fmt.Fprintf(os.Stdout, "Update available: %s -> %s\n", plan.CurrentVersion, plan.LatestVersion)
		_, _ = fmt.Fprintf(os.Stdout, "Auto-accepting update because %s is set.\n", autoAcceptUpdateEnv)
	} else if !canPromptForUpdateConsent() {
		appendUpdateDebugLog("update available but prompt is not interactive")
		_, _ = fmt.Fprintln(os.Stdout, "Update available, but confirmation is not interactive. Skipping update.")
		return false
	} else {
		appendUpdateDebugLog("prompting for update confirmation")
		_, _ = fmt.Fprintf(os.Stdout, "Update available: %s -> %s\n", plan.CurrentVersion, plan.LatestVersion)
		if !promptYesNo(os.Stdin, os.Stdout, "Download and install the latest release now? [y/N]: ") {
			appendUpdateDebugLog("update declined")
			_, _ = fmt.Fprintln(os.Stdout, "Skipping update.")
			return false
		}
	}
	if !shouldAutoAcceptUpdate() && !canPromptForUpdateConsent() {
		_, _ = fmt.Fprintln(os.Stdout, "Skipping update.")
		return false
	}

	writeStartupStatus(appDir, startupstate.StatusInitializing, false, "Downloading and staging application update.", "")
	if err := writeUpdatePendingMarker(appDir, plan.LatestVersion); err != nil {
		appendUpdateDebugLog("update marker write failed: " + err.Error())
		_, _ = fmt.Fprintf(os.Stdout, "Update skipped: %s\n", err.Error())
		return false
	}
	_, _ = fmt.Fprintln(os.Stdout, "Downloading and staging application update...")
	_ = updatePendingMarkerMessage(appDir, "Downloading the latest application bundle...")
	if err := prepareAndLaunchStagedUpdate(context.Background(), plan, args); err != nil {
		_ = removeUpdatePendingMarker(appDir)
		appendUpdateDebugLog("update staging failed: " + err.Error())
		_, _ = fmt.Fprintf(os.Stdout, "Update skipped: %s\n", err.Error())
		return false
	}

	appendUpdateDebugLog("update staged and updater launch requested")
	_, _ = fmt.Fprintln(os.Stdout, "Update staged. Relaunching phytozome GO...")
	return true
}

func shouldSkipReleaseUpdateCheck() bool {
	return strings.TrimSpace(version) == ""
}

func shouldAutoAcceptUpdate() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(autoAcceptUpdateEnv)))
	return value == "1" || value == "true" || value == "yes" || value == "y"
}

func canPromptForUpdateConsent() bool {
	stdinFD := int(os.Stdin.Fd())
	stdoutFD := int(os.Stdout.Fd())
	return term.IsTerminal(stdinFD) && term.IsTerminal(stdoutFD)
}

func promptYesNo(input io.Reader, output io.Writer, label string) bool {
	reader := bufio.NewReader(input)
	for {
		_, _ = fmt.Fprint(output, label)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false
		}
		answer := strings.TrimSpace(strings.ToLower(line))
		switch answer {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		}
		_, _ = fmt.Fprintln(output, "Please enter y or n.")
		if err == io.EOF {
			return false
		}
	}
}

func fetchLatestRelease(ctx context.Context) (githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubLatestReleaseAPI, nil)
	if err != nil {
		return githubRelease{}, fmt.Errorf("build GitHub release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "phytozome-go-cleancache/"+strings.TrimSpace(version))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("could not reach GitHub releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("GitHub releases returned %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("decode GitHub release response: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubRelease{}, fmt.Errorf("latest GitHub release is missing a tag")
	}
	return release, nil
}

func planStagedUpdate(release githubRelease, currentVersion string) (stagedUpdatePlan, bool, error) {
	spec, err := currentUpdateAssetSpec()
	if err != nil {
		return stagedUpdatePlan{}, false, err
	}
	if normalizedVersion(currentVersion) == normalizedVersion(release.TagName) {
		return stagedUpdatePlan{}, false, nil
	}

	asset, ok := findReleaseAsset(release.Assets, spec.AssetName)
	if !ok {
		return stagedUpdatePlan{}, false, fmt.Errorf("latest release %s is missing asset %s", release.TagName, spec.AssetName)
	}

	cleanerPath, err := os.Executable()
	if err != nil {
		return stagedUpdatePlan{}, false, fmt.Errorf("locate helper executable: %w", err)
	}
	installRoot, err := spec.InstallRootFromExec(cleanerPath)
	if err != nil {
		return stagedUpdatePlan{}, false, err
	}
	relaunchPath := filepath.Join(installRoot, filepath.FromSlash(spec.RelaunchRelative))
	stageDir := filepath.Join(filepath.Dir(installRoot), filepath.Base(installRoot)+".update-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	backupDir := filepath.Join(filepath.Dir(installRoot), filepath.Base(installRoot)+".backup-old")

	return stagedUpdatePlan{
		CurrentVersion: strings.TrimSpace(currentVersion),
		LatestVersion:  strings.TrimSpace(release.TagName),
		Asset:          asset,
		Spec:           spec,
		CleanerPath:    cleanerPath,
		InstallRoot:    installRoot,
		RelaunchPath:   relaunchPath,
		StageDir:       stageDir,
		BackupDir:      backupDir,
	}, true, nil
}

func currentUpdateAssetSpec() (updateAssetSpec, error) {
	return updateAssetSpecFor(runtime.GOOS, runtime.GOARCH)
}

func updateAssetSpecFor(goos string, goarch string) (updateAssetSpec, error) {
	switch goos {
	case "windows":
		if goarch != "amd64" {
			return updateAssetSpec{}, fmt.Errorf("self-update is not packaged for %s/%s", goos, goarch)
		}
		return updateAssetSpec{
			AssetName:        "phytozome-go_windows_amd64_wezterm.zip",
			ArchiveKind:      "zip",
			RelaunchRelative: "phytozome-go.exe",
			OutputRelative:   "output",
			PreserveRelative: []string{"output", "symbolname.pgd"},
			VerifyRelative: []string{
				"phytozome-go.exe",
				"phgohelper.bin",
				"core.bin",
				"wezterm.bin",
			},
			InstallRootFromExec: func(cleanerPath string) (string, error) {
				return filepath.Dir(strings.TrimSpace(cleanerPath)), nil
			},
		}, nil
	case "linux":
		if goarch != "amd64" {
			return updateAssetSpec{}, fmt.Errorf("self-update is not packaged for %s/%s", goos, goarch)
		}
		return updateAssetSpec{
			AssetName:        "phytozome-go_linux_amd64_wezterm.tar.gz",
			ArchiveKind:      "tar.gz",
			StripPrefix:      "phytozome-go_linux_amd64_wezterm",
			RelaunchRelative: "phytozome-go",
			OutputRelative:   "output",
			PreserveRelative: []string{"output", "symbolname.pgd"},
			VerifyRelative: []string{
				"phytozome-go",
				"phgohelper.bin",
				"core.bin",
				"wezterm",
				"wezterm.AppImage",
			},
			InstallRootFromExec: func(cleanerPath string) (string, error) {
				return filepath.Dir(strings.TrimSpace(cleanerPath)), nil
			},
		}, nil
	case "darwin":
		switch goarch {
		case "amd64", "arm64":
		default:
			return updateAssetSpec{}, fmt.Errorf("self-update is not packaged for %s/%s", goos, goarch)
		}
		return updateAssetSpec{
			AssetName:        "phytozome-go_macos_" + goarch + "_wezterm.tar.gz",
			ArchiveKind:      "tar.gz",
			StripPrefix:      "phytozome GO.app",
			RelaunchRelative: "Contents/MacOS/phytozome-go",
			OutputRelative:   "Contents/MacOS/output",
			PreserveRelative: []string{"Contents/MacOS/output", "Contents/MacOS/symbolname.pgd"},
			VerifyRelative: []string{
				"Contents/MacOS/phytozome-go",
				"Contents/MacOS/phgohelper.bin",
				"Contents/MacOS/core.bin",
				"Contents/MacOS/wezterm",
			},
			InstallRootFromExec: func(cleanerPath string) (string, error) {
				cleanerPath = filepath.Clean(strings.TrimSpace(cleanerPath))
				if cleanerPath == "" {
					return "", fmt.Errorf("helper path is empty")
				}
				macOSDir := filepath.Dir(cleanerPath)
				contentsDir := filepath.Dir(macOSDir)
				appDir := filepath.Dir(contentsDir)
				if strings.EqualFold(filepath.Base(contentsDir), "Contents") && strings.HasSuffix(strings.ToLower(filepath.Base(appDir)), ".app") {
					return appDir, nil
				}
				return "", fmt.Errorf("could not locate macOS app bundle from helper:\n%s", cleanerPath)
			},
		}, nil
	default:
		return updateAssetSpec{}, fmt.Errorf("self-update is not supported on %s", goos)
	}
}

func normalizedVersion(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "v")
	return value
}

func findReleaseAsset(assets []githubReleaseAsset, name string) (githubReleaseAsset, bool) {
	for _, asset := range assets {
		if strings.EqualFold(strings.TrimSpace(asset.Name), strings.TrimSpace(name)) {
			return asset, true
		}
	}
	return githubReleaseAsset{}, false
}

func prepareAndLaunchStagedUpdate(ctx context.Context, plan stagedUpdatePlan, args []string) error {
	appendUpdateDebugLog("prepare staged update start")
	if err := os.RemoveAll(plan.StageDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear update staging directory %s: %w", plan.StageDir, err)
	}
	if err := os.MkdirAll(plan.StageDir, 0o755); err != nil {
		return fmt.Errorf("create update staging directory %s: %w", plan.StageDir, err)
	}
	appendUpdateDebugLog("stage dir ready: " + plan.StageDir)

	archiveFile, err := os.CreateTemp("", "phytozome-go-update-*"+archiveSuffix(plan.Spec.ArchiveKind))
	if err != nil {
		return fmt.Errorf("create temporary update archive: %w", err)
	}
	archivePath := archiveFile.Name()
	archiveFile.Close()
	defer os.Remove(archivePath)

	downloadCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	_ = updatePendingMarkerMessage(plan.InstallRoot, "Downloading the latest application bundle...")
	if err := downloadFile(downloadCtx, plan.Asset.BrowserDownloadURL, archivePath, os.Stdout); err != nil {
		_ = os.RemoveAll(plan.StageDir)
		return err
	}
	appendUpdateDebugLog("archive downloaded: " + archivePath)
	_ = updatePendingMarkerMessage(plan.InstallRoot, "Extracting the new application bundle...")
	if err := extractArchiveToStage(archivePath, plan.Spec, plan.StageDir, os.Stdout); err != nil {
		_ = os.RemoveAll(plan.StageDir)
		return err
	}
	appendUpdateDebugLog("archive extracted to stage")

	stagedRelaunchPath := filepath.Join(plan.StageDir, filepath.FromSlash(plan.Spec.RelaunchRelative))
	if _, err := os.Stat(stagedRelaunchPath); err != nil {
		_ = os.RemoveAll(plan.StageDir)
		return fmt.Errorf("staged update is missing relaunch program:\n%s", stagedRelaunchPath)
	}
	appendUpdateDebugLog("staged relaunch path exists: " + stagedRelaunchPath)

	scriptPath, err := writeUpdaterScript(plan, args)
	if err != nil {
		_ = os.RemoveAll(plan.StageDir)
		return err
	}
	appendUpdateDebugLog("updater script written: " + scriptPath)
	_ = updatePendingMarkerMessage(plan.InstallRoot, "Waiting for the old window to close before applying the update...")
	if err := startUpdaterScript(scriptPath); err != nil {
		_ = os.RemoveAll(plan.StageDir)
		return err
	}
	appendUpdateDebugLog("updater script start requested")
	return nil
}

func archiveSuffix(kind string) string {
	switch kind {
	case "zip":
		return ".zip"
	default:
		return ".tar.gz"
	}
}

func downloadFile(ctx context.Context, rawURL string, dest string, output io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("build update download request: %w", err)
	}
	req.Header.Set("User-Agent", "phytozome-go-cleancache/"+strings.TrimSpace(version))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download latest release asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download latest release asset returned %s", resp.Status)
	}

	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create update archive %s: %w", dest, err)
	}
	defer file.Close()

	progress := newDetailedProgress(output, "Downloading application update...", resp.ContentLength, 0)
	if _, err := io.Copy(file, io.TeeReader(resp.Body, progress.byteWriter())); err != nil {
		progress.Finish("Application update download complete.", false)
		return fmt.Errorf("write update archive %s: %w", dest, err)
	}
	progress.Finish("Application update download complete.", true)
	return nil
}

func extractArchiveToStage(archivePath string, spec updateAssetSpec, stageDir string, output io.Writer) error {
	switch spec.ArchiveKind {
	case "zip":
		return extractZipArchive(archivePath, spec.StripPrefix, stageDir, output)
	case "tar.gz":
		return extractTarGzArchive(archivePath, spec.StripPrefix, stageDir, output)
	default:
		return fmt.Errorf("unsupported update archive kind %q", spec.ArchiveKind)
	}
}

func extractZipArchive(archivePath string, stripPrefix string, dest string, output io.Writer) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open update zip %s: %w", archivePath, err)
	}
	defer reader.Close()

	totalBytes := int64(0)
	totalFiles := 0
	for _, file := range reader.File {
		entryName, ok := archiveEntryTargetName(file.Name, stripPrefix)
		if !ok {
			continue
		}
		if strings.HasSuffix(strings.ReplaceAll(file.Name, "\\", "/"), "/") {
			continue
		}
		if entryName == "" {
			continue
		}
		totalFiles++
		totalBytes += int64(file.UncompressedSize64)
	}
	progress := newDetailedProgress(output, "Extracting application update...", totalBytes, totalFiles)
	success := false
	defer func() {
		progress.Finish("Application update extraction complete.", success)
	}()

	for _, file := range reader.File {
		entryName, ok := archiveEntryTargetName(file.Name, stripPrefix)
		if !ok {
			continue
		}
		targetPath, err := safeArchivePath(dest, entryName)
		if err != nil {
			return err
		}
		if strings.HasSuffix(strings.ReplaceAll(file.Name, "\\", "/"), "/") {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create update directory %s: %w", targetPath, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create update directory %s: %w", filepath.Dir(targetPath), err)
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			rc.Close()
			return fmt.Errorf("create update file %s: %w", targetPath, err)
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return fmt.Errorf("extract zip entry %s: %w", file.Name, err)
		}
		out.Close()
		rc.Close()
		progress.AddBytes(int64(file.UncompressedSize64))
		progress.AddFile()
	}
	success = true
	return nil
}

func extractTarGzArchive(archivePath string, stripPrefix string, dest string, output io.Writer) error {
	totalBytes, totalFiles, err := scanTarGzArchiveTotals(archivePath, stripPrefix)
	if err != nil {
		return err
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open update archive %s: %w", archivePath, err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip stream %s: %w", archivePath, err)
	}
	defer gzReader.Close()

	progress := newDetailedProgress(output, "Extracting application update...", totalBytes, totalFiles)
	success := false
	defer func() {
		progress.Finish("Application update extraction complete.", success)
	}()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			success = true
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar archive %s: %w", archivePath, err)
		}

		entryName, ok := archiveEntryTargetName(header.Name, stripPrefix)
		if !ok {
			continue
		}
		targetPath, err := safeArchivePath(dest, entryName)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create update directory %s: %w", targetPath, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create update directory %s: %w", filepath.Dir(targetPath), err)
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create update file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(out, tarReader); err != nil {
				out.Close()
				return fmt.Errorf("extract tar entry %s: %w", header.Name, err)
			}
			out.Close()
			progress.AddBytes(header.Size)
			progress.AddFile()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create update directory %s: %w", filepath.Dir(targetPath), err)
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil && !os.IsExist(err) {
				return fmt.Errorf("create update symlink %s: %w", targetPath, err)
			}
			progress.AddFile()
		}
	}
}

func scanTarGzArchiveTotals(archivePath string, stripPrefix string) (int64, int, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return 0, 0, fmt.Errorf("open update archive %s: %w", archivePath, err)
	}
	defer file.Close()
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, 0, fmt.Errorf("open gzip stream %s: %w", archivePath, err)
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)
	var totalBytes int64
	totalFiles := 0
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return totalBytes, totalFiles, nil
		}
		if err != nil {
			return 0, 0, fmt.Errorf("scan tar archive %s: %w", archivePath, err)
		}
		if _, ok := archiveEntryTargetName(header.Name, stripPrefix); !ok {
			continue
		}
		switch header.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			totalBytes += header.Size
			totalFiles++
		case tar.TypeSymlink:
			totalFiles++
		}
	}
}

type detailedProgress struct {
	mu         sync.Mutex
	message    string
	totalBytes int64
	current    int64
	filesTotal int
	filesDone  int
	started    time.Time
	lastEmit   time.Time
	console    *consoleProgress
}

func newDetailedProgress(output io.Writer, message string, totalBytes int64, filesTotal int) *detailedProgress {
	now := time.Now()
	progress := &detailedProgress{
		message:    strings.TrimSpace(message),
		totalBytes: totalBytes,
		filesTotal: filesTotal,
		started:    now,
		lastEmit:   now.Add(-time.Second),
		console:    newConsoleProgress(output),
	}
	progress.emitLocked(false, true)
	return progress
}

func (p *detailedProgress) byteWriter() io.Writer {
	return progressByteWriter{progress: p}
}

func (p *detailedProgress) AddBytes(n int64) {
	if p == nil || n <= 0 {
		return
	}
	p.mu.Lock()
	p.current += n
	p.emitLocked(false, false)
	p.mu.Unlock()
}

func (p *detailedProgress) AddFile() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.filesDone++
	p.emitLocked(false, false)
	p.mu.Unlock()
}

func (p *detailedProgress) Finish(successMessage string, ok bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	if ok {
		if p.totalBytes > 0 && p.current < p.totalBytes {
			p.current = p.totalBytes
		}
		if p.filesTotal > 0 && p.filesDone < p.filesTotal {
			p.filesDone = p.filesTotal
		}
		p.emitLocked(true, true)
	}
	p.mu.Unlock()
	p.console.Finish(successMessage, ok)
}

func (p *detailedProgress) emitLocked(done bool, force bool) {
	now := time.Now()
	if !force && now.Sub(p.lastEmit) < 250*time.Millisecond {
		return
	}
	p.lastEmit = now
	parts := []string{firstNonEmptyText(p.message, "Working...")}
	if percent := detailedProgressPercent(p.current, p.totalBytes, p.filesDone, p.filesTotal); percent >= 0 {
		parts = append(parts, fmt.Sprintf("%.1f%%", percent))
	}
	if p.totalBytes > 0 && p.current > 0 {
		parts = append(parts, fmt.Sprintf("%s/%s", humanBytes(p.current), humanBytes(p.totalBytes)))
	} else if p.totalBytes > 0 {
		parts = append(parts, humanBytes(p.totalBytes))
	}
	if !done {
		if speed := detailedProgressSpeed(p.started, p.current); speed > 0 {
			parts = append(parts, fmt.Sprintf("%s/s", humanBytes(int64(speed))))
		}
	}
	if p.filesTotal > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d files", p.filesDone, p.filesTotal))
	}
	p.console.Update(strings.Join(parts, " | "), done)
}

func detailedProgressPercent(current int64, total int64, filesDone int, filesTotal int) float64 {
	if total > 0 {
		return float64(current) * 100 / float64(total)
	}
	if filesTotal > 0 {
		return float64(filesDone) * 100 / float64(filesTotal)
	}
	return -1
}

func detailedProgressSpeed(started time.Time, current int64) float64 {
	if current <= 0 {
		return 0
	}
	elapsed := time.Since(started).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(current) / elapsed
}

type progressByteWriter struct {
	progress *detailedProgress
}

func (w progressByteWriter) Write(p []byte) (int, error) {
	if w.progress != nil && len(p) > 0 {
		w.progress.AddBytes(int64(len(p)))
	}
	return len(p), nil
}

func archiveEntryTargetName(rawName string, stripPrefix string) (string, bool) {
	name := strings.TrimSpace(strings.ReplaceAll(rawName, "\\", "/"))
	name = strings.TrimPrefix(name, "./")
	name = strings.Trim(name, "/")
	if name == "" {
		return "", false
	}
	if strings.TrimSpace(stripPrefix) != "" {
		prefix := strings.Trim(strings.ReplaceAll(stripPrefix, "\\", "/"), "/")
		if name == prefix {
			return "", false
		}
		if !strings.HasPrefix(name, prefix+"/") {
			return "", false
		}
		name = strings.TrimPrefix(name, prefix+"/")
	}
	name = strings.Trim(name, "/")
	if name == "" {
		return "", false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", false
		}
	}
	return filepath.FromSlash(name), true
}

func safeArchivePath(root string, relative string) (string, error) {
	root = filepath.Clean(root)
	target := filepath.Clean(filepath.Join(root, relative))
	if target == root {
		return target, nil
	}
	prefix := root + string(os.PathSeparator)
	if !strings.HasPrefix(target, prefix) {
		return "", fmt.Errorf("update archive tried to write outside staging directory:\n%s", target)
	}
	return target, nil
}

func writeUpdaterScript(plan stagedUpdatePlan, args []string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		return writePowerShellUpdater(plan, args)
	default:
		return writeShellUpdater(plan, args)
	}
}

func writePowerShellUpdater(plan stagedUpdatePlan, args []string) (string, error) {
	scriptFile, err := os.CreateTemp("", "phytozome-go-updater-*.ps1")
	if err != nil {
		return "", fmt.Errorf("create updater script: %w", err)
	}
	defer scriptFile.Close()

	script := buildPowerShellUpdaterScript(plan, args)
	if _, err := scriptFile.WriteString(script); err != nil {
		return "", fmt.Errorf("write updater script %s: %w", scriptFile.Name(), err)
	}
	return scriptFile.Name(), nil
}

func buildPowerShellUpdaterScript(plan stagedUpdatePlan, args []string) string {
	argList := make([]string, 0, len(args))
	for _, arg := range args {
		argList = append(argList, psQuote(arg))
	}
	startProcessLine := fmt.Sprintf("Start-Process -FilePath $Launcher -WorkingDirectory $WorkingDir")
	if len(argList) > 0 {
		startProcessLine += " -ArgumentList @(" + strings.Join(argList, ", ") + ")"
	}
	preserveRelative := preserveRelativePaths(plan.Spec)

	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$ParentPid = %d
$TargetDir = %s
$StageDir = %s
$PreserveDir = $StageDir + '.preserve'
$PendingMarkerPath = Join-Path $TargetDir %s
$Launcher = %s
$WorkingDir = %s
$OutputRelative = %s
$PreserveRelative = @(%s)
$VerifyRelative = @(%s)
$LogPath = [Environment]::GetEnvironmentVariable(%s)
$LastUpdateErrorEnvName = %s

function Write-UpdateLog {
    param([string]$Message)
    if ([string]::IsNullOrWhiteSpace($LogPath)) {
        return
    }
    $dir = Split-Path -Parent $LogPath
    if (-not [string]::IsNullOrWhiteSpace($dir)) {
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
    }
    Add-Content -LiteralPath $LogPath -Value ((Get-Date).ToString('yyyy-MM-dd HH:mm:ss.fff') + ' ' + $Message)
}

function Update-PendingMarkerMessage {
    param([string]$Message)
    if ([string]::IsNullOrWhiteSpace($PendingMarkerPath) -or -not (Test-Path -LiteralPath $PendingMarkerPath)) {
        return
    }
    try {
        $marker = Get-Content -LiteralPath $PendingMarkerPath -Raw | ConvertFrom-Json
        $marker.message = $Message
        $marker | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $PendingMarkerPath
    } catch {
        Write-UpdateLog ("pending marker update skipped: " + $_.Exception.Message)
    }
}

function Invoke-RobocopyMirror {
    param(
        [string]$Source,
        [string]$Destination
    )

    Write-UpdateLog ("robocopy mirror start: " + $Source + " -> " + $Destination)
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    & robocopy $Source $Destination /MIR /COPY:DAT /DCOPY:DAT /R:20 /W:1 /NFL /NDL /NJH /NJS /NP | Out-Null
    $code = $LASTEXITCODE
    Write-UpdateLog ("robocopy exit code: " + $code)
    if ($code -gt 7) {
        throw "robocopy mirror failed with exit code $code"
    }
}

function Get-NormalizedRelativePath {
    param([string]$RelativePath)
    if ([string]::IsNullOrWhiteSpace($RelativePath)) {
        return ''
    }
    return ($RelativePath.Replace('\', '/').Trim('/')).ToLowerInvariant()
}

function Should-PreserveByCopy {
    param(
        [string]$RelativePath,
        [string]$OutputRelativePath
    )
    $normalizedRelative = Get-NormalizedRelativePath $RelativePath
    $normalizedOutput = Get-NormalizedRelativePath $OutputRelativePath
    if ([string]::IsNullOrWhiteSpace($normalizedRelative) -or [string]::IsNullOrWhiteSpace($normalizedOutput)) {
        return $false
    }
    return [string]::Equals($normalizedRelative, $normalizedOutput, [System.StringComparison]::OrdinalIgnoreCase)
}

function Copy-DirectoryWithRobocopy {
    param(
        [string]$Source,
        [string]$Destination,
        [string]$ContextLabel
    )

    Write-UpdateLog ($ContextLabel + " robocopy start: " + $Source + " -> " + $Destination)
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    & robocopy $Source $Destination /E /COPY:DAT /DCOPY:DAT /R:20 /W:1 /NFL /NDL /NJH /NJS /NP | Out-Null
    $code = $LASTEXITCODE
    Write-UpdateLog ($ContextLabel + " robocopy exit code: " + $code)
    if ($code -gt 7) {
        throw ($ContextLabel + " failed with exit code " + $code)
    }
}

function Preserve-RelativePath {
    param(
        [string]$CurrentRoot,
        [string]$PreserveRoot,
        [string]$RelativePath,
        [string]$OutputRelativePath
    )

    if ([string]::IsNullOrWhiteSpace($RelativePath)) {
        return
    }

    $Source = Join-Path $CurrentRoot $RelativePath
    $Destination = Join-Path $PreserveRoot $RelativePath
    $copyMode = Should-PreserveByCopy -RelativePath $RelativePath -OutputRelativePath $OutputRelativePath
    if (-not (Test-Path -LiteralPath $Source)) {
        if (Test-Path -LiteralPath $Destination) {
            Write-UpdateLog ("preserved path already staged: " + $Destination)
            return
        }
        Write-UpdateLog ("preserved path missing: " + $Source)
        return
    }

    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
    if (Test-Path -LiteralPath $Source -PathType Leaf) {
        if ($copyMode) {
            Write-UpdateLog ("preserve file by copy: " + $Source + " -> " + $Destination)
            Copy-Item -LiteralPath $Source -Destination $Destination -Force
            return
        }
        try {
            Write-UpdateLog ("preserve file by move: " + $Source + " -> " + $Destination)
            Move-Item -LiteralPath $Source -Destination $Destination -Force
            return
        } catch {
            Write-UpdateLog ("preserve file move fallback to copy: " + $_.Exception.Message)
            Copy-Item -LiteralPath $Source -Destination $Destination -Force
            Remove-Item -LiteralPath $Source -Force -ErrorAction SilentlyContinue
            return
        }
    }
    if (Test-Path -LiteralPath $Source -PathType Container) {
        if ($copyMode) {
            Write-UpdateLog ("preserve directory by copy: " + $Source + " -> " + $Destination)
            Copy-DirectoryWithRobocopy -Source $Source -Destination $Destination -ContextLabel "preserve directory"
            return
        }
        try {
            Write-UpdateLog ("preserve directory by move: " + $Source + " -> " + $Destination)
            Move-Item -LiteralPath $Source -Destination $Destination -Force
            return
        } catch {
            Write-UpdateLog ("preserve directory move fallback to copy: " + $_.Exception.Message)
            Copy-DirectoryWithRobocopy -Source $Source -Destination $Destination -ContextLabel "preserve directory"
            Remove-Item -LiteralPath $Source -Recurse -Force -ErrorAction SilentlyContinue
            return
        }
    }
}

function Restore-PreservedPath {
    param(
        [string]$PreserveRoot,
        [string]$TargetRoot,
        [string]$RelativePath,
        [string]$OutputRelativePath
    )

    if ([string]::IsNullOrWhiteSpace($RelativePath)) {
        return
    }

    $Source = Join-Path $PreserveRoot $RelativePath
    if (-not (Test-Path -LiteralPath $Source)) {
        return
    }
    $Destination = Join-Path $TargetRoot $RelativePath
    $copyMode = Should-PreserveByCopy -RelativePath $RelativePath -OutputRelativePath $OutputRelativePath
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null

    if (Test-Path -LiteralPath $Source -PathType Leaf) {
        if ($copyMode) {
            Write-UpdateLog ("restore file by copy: " + $Source + " -> " + $Destination)
            Copy-Item -LiteralPath $Source -Destination $Destination -Force
            return
        }
        try {
            Write-UpdateLog ("restore file by move: " + $Source + " -> " + $Destination)
            Move-Item -LiteralPath $Source -Destination $Destination -Force
            return
        } catch {
            Write-UpdateLog ("restore file move fallback to copy: " + $_.Exception.Message)
            Copy-Item -LiteralPath $Source -Destination $Destination -Force
            Remove-Item -LiteralPath $Source -Force -ErrorAction SilentlyContinue
            return
        }
    }
    if (Test-Path -LiteralPath $Source -PathType Container) {
        if ($copyMode) {
            Write-UpdateLog ("restore directory by copy: " + $Source + " -> " + $Destination)
            Copy-DirectoryWithRobocopy -Source $Source -Destination $Destination -ContextLabel "restore directory"
            return
        }
        try {
            Write-UpdateLog ("restore directory by move: " + $Source + " -> " + $Destination)
            Move-Item -LiteralPath $Source -Destination $Destination -Force
            return
        } catch {
            Write-UpdateLog ("restore directory move fallback to copy: " + $_.Exception.Message)
            Copy-DirectoryWithRobocopy -Source $Source -Destination $Destination -ContextLabel "restore directory"
            Remove-Item -LiteralPath $Source -Recurse -Force -ErrorAction SilentlyContinue
            return
        }
    }
}

function Test-FilesMatch {
    param(
        [string]$LeftPath,
        [string]$RightPath
    )

    if (-not (Test-Path -LiteralPath $LeftPath -PathType Leaf)) {
        throw "missing expected file: $LeftPath"
    }
    if (-not (Test-Path -LiteralPath $RightPath -PathType Leaf)) {
        throw "missing expected file: $RightPath"
    }

    $left = Get-Item -LiteralPath $LeftPath
    $right = Get-Item -LiteralPath $RightPath
    if ($left.Length -ne $right.Length) {
        return $false
    }

    $leftHash = (Get-FileHash -LiteralPath $LeftPath -Algorithm SHA256).Hash
    $rightHash = (Get-FileHash -LiteralPath $RightPath -Algorithm SHA256).Hash
    return $leftHash -eq $rightHash
}

function Assert-KeyFilesUpdated {
    param(
        [string]$StageRoot,
        [string]$TargetRoot,
        [string[]]$RelativePaths
    )

    foreach ($relative in $RelativePaths) {
        if ([string]::IsNullOrWhiteSpace($relative)) {
            continue
        }
        $stagePath = Join-Path $StageRoot $relative
        $targetPath = Join-Path $TargetRoot $relative
        Write-UpdateLog ("verify key file: " + $relative)
        if (-not (Test-FilesMatch -LeftPath $stagePath -RightPath $targetPath)) {
            throw "updated file verification failed: $relative"
        }
    }
}

function Test-LauncherAlreadyRunning {
    param([string]$LauncherPath)
    $name = [System.IO.Path]::GetFileName($LauncherPath)
    if ([string]::IsNullOrWhiteSpace($name)) {
        return $false
    }
    foreach ($proc in (Get-CimInstance Win32_Process -Filter ("Name = '" + $name.Replace("'", "''") + "'"))) {
        $procPath = ''
        try {
            $procPath = [string]$proc.ExecutablePath
        } catch {
            $procPath = ''
        }
        if (-not [string]::IsNullOrWhiteSpace($procPath) -and ([string]::Equals($procPath, $LauncherPath, [System.StringComparison]::OrdinalIgnoreCase))) {
            Write-UpdateLog ("launcher already running: " + $LauncherPath)
            return $true
        }
    }
    return $false
}

Write-UpdateLog ("updater start; parent pid=" + $ParentPid)
while (Get-Process -Id $ParentPid -ErrorAction SilentlyContinue) {
    Start-Sleep -Milliseconds 200
}
Write-UpdateLog "parent exited"
Update-PendingMarkerMessage "Preserving files from the current installation..."
Remove-Item -LiteralPath $PreserveDir -Recurse -Force -ErrorAction SilentlyContinue
foreach ($relative in $PreserveRelative) {
    Preserve-RelativePath -CurrentRoot $TargetDir -PreserveRoot $PreserveDir -RelativePath $relative -OutputRelativePath $OutputRelative
}

$UpdateSucceeded = $false
$LastUpdateError = ''
for ($i = 0; $i -lt 120; $i++) {
    try {
        Write-UpdateLog ("update attempt " + ($i + 1))
        Update-PendingMarkerMessage "Applying the new application files..."
        Invoke-RobocopyMirror -Source $StageDir -Destination $TargetDir
        Update-PendingMarkerMessage "Restoring preserved files into the new installation..."
        foreach ($relative in $PreserveRelative) {
            Restore-PreservedPath -PreserveRoot $PreserveDir -TargetRoot $TargetDir -RelativePath $relative -OutputRelativePath $OutputRelative
        }
        Update-PendingMarkerMessage "Verifying the updated installation..."
        Assert-KeyFilesUpdated -StageRoot $StageDir -TargetRoot $TargetDir -RelativePaths $VerifyRelative
        Remove-Item -LiteralPath $StageDir -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $PreserveDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-UpdateLog "stage directory removed"
        $UpdateSucceeded = $true
        break
    } catch {
        $LastUpdateError = $_.Exception.Message
        Write-UpdateLog ("update attempt failed: " + $LastUpdateError)
        if ($i -eq 119) { break }
        Start-Sleep -Seconds 1
    }
}

if (-not $UpdateSucceeded) {
    if ([string]::IsNullOrWhiteSpace($LastUpdateError)) {
        $LastUpdateError = 'unknown update failure'
    }
    [Environment]::SetEnvironmentVariable($LastUpdateErrorEnvName, $LastUpdateError, 'Process')
    Remove-Item -LiteralPath (Join-Path $TargetDir %s) -Force -ErrorAction SilentlyContinue
    if (-not (Test-LauncherAlreadyRunning -LauncherPath $Launcher)) {
        Write-UpdateLog ("launching after failed update: " + $Launcher)
        %s
        Write-UpdateLog "launch command returned after failed update"
    } else {
        Write-UpdateLog "skip launch after failed update because launcher is already running"
    }
    exit 0
}

[Environment]::SetEnvironmentVariable($LastUpdateErrorEnvName, $null, 'Process')
Update-PendingMarkerMessage "Launching the updated application..."
Remove-Item -LiteralPath $PendingMarkerPath -Force -ErrorAction SilentlyContinue
$env:%s = '1'
if (-not (Test-LauncherAlreadyRunning -LauncherPath $Launcher)) {
    Write-UpdateLog ("launching after successful update: " + $Launcher)
    %s
    Write-UpdateLog "launch command returned after successful update"
} else {
    Write-UpdateLog "skip launch after successful update because launcher is already running"
}
`, os.Getpid(), psQuote(plan.InstallRoot), psQuote(plan.StageDir), psQuote(updatePendingMarkerName), psQuote(plan.RelaunchPath), psQuote(filepath.Dir(plan.RelaunchPath)), psQuote(plan.Spec.OutputRelative), strings.Join(psQuoteSlice(preserveRelative), ", "), strings.Join(psQuoteSlice(plan.Spec.VerifyRelative), ", "), psQuote(updateDebugLogEnv), psQuote(lastUpdateErrorEnv), psQuote(updatePendingMarkerName), startProcessLine, skipBundlePreflightEnv, startProcessLine)
}

func writeShellUpdater(plan stagedUpdatePlan, args []string) (string, error) {
	scriptFile, err := os.CreateTemp("", "phytozome-go-updater-*.sh")
	if err != nil {
		return "", fmt.Errorf("create updater script: %w", err)
	}
	defer scriptFile.Close()

	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, shQuote(arg))
	}
	argsSuffix := ""
	if len(quotedArgs) > 0 {
		argsSuffix = " " + strings.Join(quotedArgs, " ")
	}
	preserveRelative := preserveRelativePaths(plan.Spec)
	preserveList := shQuote(strings.Join(preserveRelative, "\n"))

	script := fmt.Sprintf(`#!/bin/sh
set -eu
PARENT_PID=%d
TARGET_DIR=%s
STAGE_DIR=%s
PRESERVE_DIR="$STAGE_DIR.preserve"
BACKUP_DIR=%s
PENDING_MARKER_PATH="$TARGET_DIR/"%s
LAUNCHER=%s
WORKING_DIR=%s
OUTPUT_REL=%s
PRESERVE_RELATIVES=%s
PENDING_MARKER=%s

normalize_relative_path() {
  printf '%%s' "$1" | tr '\\' '/' | sed 's#^/*##; s#/*$##'
}

should_preserve_by_copy() {
  REL_NORM="$(normalize_relative_path "$1")"
  OUT_NORM="$(normalize_relative_path "$2")"
  [ -n "$REL_NORM" ] && [ -n "$OUT_NORM" ] && [ "$REL_NORM" = "$OUT_NORM" ]
}

copy_directory_preserve() {
  SOURCE="$1"
  DEST="$2"
  mkdir -p "$DEST"
  cp -a "$SOURCE"/. "$DEST"/
}

update_pending_marker_message() {
  MESSAGE="$1"
  if [ ! -f "$PENDING_MARKER_PATH" ]; then
    return
  fi
  @'
import json
import sys
from pathlib import Path
path = Path(sys.argv[1])
message = sys.argv[2]
try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception:
    sys.exit(0)
data["message"] = message
path.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")
'@ | python - "$PENDING_MARKER_PATH" "$MESSAGE" >/dev/null 2>&1 || true
}

preserve_relative_path() {
  REL="$1"
  if [ -z "$REL" ]; then
    return
  fi
  SOURCE="$TARGET_DIR/$REL"
  DEST="$PRESERVE_DIR/$REL"
  mkdir -p "$(dirname "$DEST")"
  if [ ! -e "$SOURCE" ]; then
    return
  fi
  if should_preserve_by_copy "$REL" "$OUTPUT_REL"; then
    if [ -d "$SOURCE" ]; then
      copy_directory_preserve "$SOURCE" "$DEST"
    else
      cp -p "$SOURCE" "$DEST"
    fi
    return
  fi
  if mv "$SOURCE" "$DEST" 2>/dev/null; then
    return
  fi
  if [ -d "$SOURCE" ]; then
    copy_directory_preserve "$SOURCE" "$DEST"
    rm -rf "$SOURCE"
  else
    cp -p "$SOURCE" "$DEST"
    rm -f "$SOURCE"
  fi
}

restore_preserved_path() {
  REL="$1"
  if [ -z "$REL" ]; then
    return
  fi
  SOURCE="$PRESERVE_DIR/$REL"
  DEST="$TARGET_DIR/$REL"
  if [ ! -e "$SOURCE" ]; then
    return
  fi
  mkdir -p "$(dirname "$DEST")"
  if should_preserve_by_copy "$REL" "$OUTPUT_REL"; then
    if [ -d "$SOURCE" ]; then
      copy_directory_preserve "$SOURCE" "$DEST"
    else
      cp -p "$SOURCE" "$DEST"
    fi
    return
  fi
  if mv "$SOURCE" "$DEST" 2>/dev/null; then
    return
  fi
  if [ -d "$SOURCE" ]; then
    copy_directory_preserve "$SOURCE" "$DEST"
    rm -rf "$SOURCE"
  else
    cp -p "$SOURCE" "$DEST"
    rm -f "$SOURCE"
  fi
}

while kill -0 "$PARENT_PID" 2>/dev/null; do
  sleep 1
done

update_pending_marker_message "Preserving files from the current installation..."
rm -rf "$PRESERVE_DIR"
for REL in $PRESERVE_RELATIVES; do
  preserve_relative_path "$REL"
done
rm -rf "$BACKUP_DIR"
if [ -e "$TARGET_DIR" ]; then
  mv "$TARGET_DIR" "$BACKUP_DIR"
fi
update_pending_marker_message "Applying the new application files..."
mv "$STAGE_DIR" "$TARGET_DIR"
update_pending_marker_message "Restoring preserved files into the new installation..."
for REL in $PRESERVE_RELATIVES; do
  restore_preserved_path "$REL"
done
update_pending_marker_message "Launching the updated application..."
rm -rf "$PRESERVE_DIR"
rm -rf "$BACKUP_DIR"
rm -f "$TARGET_DIR/$PENDING_MARKER"

cd "$WORKING_DIR"
if ! pgrep -f "$LAUNCHER" >/dev/null 2>&1; then
  env %s=1 "$LAUNCHER"%s >/dev/null 2>&1 &
fi
`, os.Getpid(), shQuote(plan.InstallRoot), shQuote(plan.StageDir), shQuote(plan.BackupDir), shQuote(updatePendingMarkerName), shQuote(plan.RelaunchPath), shQuote(filepath.Dir(plan.RelaunchPath)), shQuote(plan.Spec.OutputRelative), preserveList, shQuote(updatePendingMarkerName), skipBundlePreflightEnv, argsSuffix)

	if _, err := scriptFile.WriteString(script); err != nil {
		return "", fmt.Errorf("write updater script %s: %w", scriptFile.Name(), err)
	}
	if err := scriptFile.Chmod(0o700); err != nil {
		return "", fmt.Errorf("chmod updater script %s: %w", scriptFile.Name(), err)
	}
	return scriptFile.Name(), nil
}

func startUpdaterScript(scriptPath string) error {
	switch runtime.GOOS {
	case "windows":
		launcherPath, err := writeWindowsVBScriptLauncher(scriptPath)
		if err != nil {
			return err
		}
		cmd := exec.Command("wscript.exe", "//B", "//NoLogo", launcherPath)
		cmd.Dir = filepath.Dir(launcherPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("start updater script %s: %w", scriptPath, err)
		}
		return nil
	default:
		cmd := exec.Command("/bin/sh", "-c", "nohup "+shQuote(scriptPath)+" >/dev/null 2>&1 &")
		cmd.Dir = filepath.Dir(scriptPath)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start updater script %s: %w", scriptPath, err)
		}
		return nil
	}
}

func writeWindowsVBScriptLauncher(scriptPath string) (string, error) {
	launcherFile, err := os.CreateTemp("", "phytozome-go-updater-launcher-*.vbs")
	if err != nil {
		return "", fmt.Errorf("create updater launcher: %w", err)
	}
	defer launcherFile.Close()

	command := `CreateObject("Wscript.Shell").Run "powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File ""` +
		vbsDoubleQuote(scriptPath) + `""", 0, False`
	if _, err := launcherFile.WriteString(command); err != nil {
		return "", fmt.Errorf("write updater launcher %s: %w", launcherFile.Name(), err)
	}
	return launcherFile.Name(), nil
}

func preserveRelativePaths(spec updateAssetSpec) []string {
	values := append([]string(nil), spec.PreserveRelative...)
	if len(values) == 0 && strings.TrimSpace(spec.OutputRelative) != "" {
		values = append(values, spec.OutputRelative)
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.Trim(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func psQuoteSlice(values []string) []string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, psQuote(value))
	}
	return quoted
}

func vbsDoubleQuote(value string) string {
	return strings.ReplaceAll(value, `"`, `""`)
}

func shQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func appendUpdateDebugLog(message string) {
	logPath := strings.TrimSpace(os.Getenv(updateDebugLogEnv))
	if logPath == "" {
		return
	}
	dir := filepath.Dir(logPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	line := time.Now().Format("2006-01-02 15:04:05.000") + " " + message + "\n"
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(line)
}
