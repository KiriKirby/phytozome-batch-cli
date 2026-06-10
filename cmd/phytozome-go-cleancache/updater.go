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
	"time"

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

func maybeHandleReleaseUpdate(args []string) bool {
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

	if err := runSpinner("Downloading and staging update", func() error {
		return prepareAndLaunchStagedUpdate(context.Background(), plan, args)
	}); err != nil {
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

	if err := downloadFile(downloadCtx, plan.Asset.BrowserDownloadURL, archivePath); err != nil {
		_ = os.RemoveAll(plan.StageDir)
		return err
	}
	appendUpdateDebugLog("archive downloaded: " + archivePath)
	if err := extractArchiveToStage(archivePath, plan.Spec, plan.StageDir); err != nil {
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

func downloadFile(ctx context.Context, rawURL string, dest string) error {
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

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("write update archive %s: %w", dest, err)
	}
	return nil
}

func extractArchiveToStage(archivePath string, spec updateAssetSpec, stageDir string) error {
	switch spec.ArchiveKind {
	case "zip":
		return extractZipArchive(archivePath, spec.StripPrefix, stageDir)
	case "tar.gz":
		return extractTarGzArchive(archivePath, spec.StripPrefix, stageDir)
	default:
		return fmt.Errorf("unsupported update archive kind %q", spec.ArchiveKind)
	}
}

func extractZipArchive(archivePath string, stripPrefix string, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open update zip %s: %w", archivePath, err)
	}
	defer reader.Close()

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
	}
	return nil
}

func extractTarGzArchive(archivePath string, stripPrefix string, dest string) error {
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

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
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
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create update directory %s: %w", filepath.Dir(targetPath), err)
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil && !os.IsExist(err) {
				return fmt.Errorf("create update symlink %s: %w", targetPath, err)
			}
		}
	}
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

function Copy-PreservedOutputToStage {
    param(
        [string]$CurrentRoot,
        [string]$StageRoot,
        [string]$RelativePath
    )

    if ([string]::IsNullOrWhiteSpace($RelativePath)) {
        return
    }

    $Source = Join-Path $CurrentRoot $RelativePath
    if (-not (Test-Path -LiteralPath $Source)) {
        Write-UpdateLog ("preserved output missing: " + $Source)
        return
    }

    $Destination = Join-Path $StageRoot $RelativePath
    if (Test-Path -LiteralPath $Source -PathType Leaf) {
        Write-UpdateLog ("preserve file start: " + $Source + " -> " + $Destination)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
        Copy-Item -LiteralPath $Source -Destination $Destination -Force
        return
    }
    if (Test-Path -LiteralPath $Source -PathType Container) {
        Write-UpdateLog ("preserve directory start: " + $Source + " -> " + $Destination)
        New-Item -ItemType Directory -Force -Path $Destination | Out-Null
        & robocopy $Source $Destination /E /COPY:DAT /DCOPY:DAT /R:20 /W:1 /NFL /NDL /NJH /NJS /NP | Out-Null
        $code = $LASTEXITCODE
        Write-UpdateLog ("preserve directory robocopy exit code: " + $code)
        if ($code -gt 7) {
            throw "preserve output failed with exit code $code"
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

Write-UpdateLog ("updater start; parent pid=" + $ParentPid)
while (Get-Process -Id $ParentPid -ErrorAction SilentlyContinue) {
    Start-Sleep -Milliseconds 200
}
Write-UpdateLog "parent exited"

$UpdateSucceeded = $false
$LastUpdateError = ''
for ($i = 0; $i -lt 120; $i++) {
    try {
        Write-UpdateLog ("update attempt " + ($i + 1))
        foreach ($relative in $PreserveRelative) {
            Copy-PreservedOutputToStage -CurrentRoot $TargetDir -StageRoot $StageDir -RelativePath $relative
        }
        Invoke-RobocopyMirror -Source $StageDir -Destination $TargetDir
        Assert-KeyFilesUpdated -StageRoot $StageDir -TargetRoot $TargetDir -RelativePaths $VerifyRelative
        Remove-Item -LiteralPath $StageDir -Recurse -Force -ErrorAction SilentlyContinue
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
    Write-UpdateLog ("launching after failed update: " + $Launcher)
    %s
    Write-UpdateLog "launch command returned after failed update"
    exit 0
}

[Environment]::SetEnvironmentVariable($LastUpdateErrorEnvName, $null, 'Process')
$env:%s = '1'
Write-UpdateLog ("launching after successful update: " + $Launcher)
%s
Write-UpdateLog "launch command returned after successful update"
`, os.Getpid(), psQuote(plan.InstallRoot), psQuote(plan.StageDir), psQuote(plan.RelaunchPath), psQuote(filepath.Dir(plan.RelaunchPath)), psQuote(plan.Spec.OutputRelative), strings.Join(psQuoteSlice(preserveRelative), ", "), strings.Join(psQuoteSlice(plan.Spec.VerifyRelative), ", "), psQuote(updateDebugLogEnv), psQuote(lastUpdateErrorEnv), startProcessLine, skipBundlePreflightEnv, startProcessLine)
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
BACKUP_DIR=%s
LAUNCHER=%s
WORKING_DIR=%s
OUTPUT_REL=%s
PRESERVE_RELATIVES=%s

preserve_output() {
  REL="$1"
  if [ -z "$REL" ]; then
    return
  fi
  SOURCE="$TARGET_DIR/$REL"
  if [ ! -e "$SOURCE" ]; then
    return
  fi
  DEST="$STAGE_DIR/$REL"
  if [ -d "$SOURCE" ]; then
    mkdir -p "$DEST"
    cp -a "$SOURCE"/. "$DEST"/
  else
    mkdir -p "$(dirname "$DEST")"
    cp -p "$SOURCE" "$DEST"
  fi
}

while kill -0 "$PARENT_PID" 2>/dev/null; do
  sleep 1
done

for REL in $PRESERVE_RELATIVES; do
  preserve_output "$REL"
done
rm -rf "$BACKUP_DIR"
if [ -e "$TARGET_DIR" ]; then
  mv "$TARGET_DIR" "$BACKUP_DIR"
fi
mv "$STAGE_DIR" "$TARGET_DIR"
rm -rf "$BACKUP_DIR"

cd "$WORKING_DIR"
env %s=1 "$LAUNCHER"%s >/dev/null 2>&1 &
`, os.Getpid(), shQuote(plan.InstallRoot), shQuote(plan.StageDir), shQuote(plan.BackupDir), shQuote(plan.RelaunchPath), shQuote(filepath.Dir(plan.RelaunchPath)), shQuote(plan.Spec.OutputRelative), preserveList, skipBundlePreflightEnv, argsSuffix)

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
