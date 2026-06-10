package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/labelname"
	"github.com/KiriKirby/phytozome-go/internal/startupstate"
)

const mainProgramName = "core.bin"
const windowsWezTermCLIName = "wezterm-cli.bin"
const displayName = "phytozome GO"
const author = "wangsychn"
const repoURL = "https://github.com/KiriKirby/phytozome-go"
const licenseName = "Common Public Attribution License 1.0"
const licenseID = "CPAL-1.0"
const skipBundlePreflightEnv = "PHGO_SKIP_BUNDLE_PREFLIGHT"
const autoAcceptUpdateEnv = "PHGO_AUTO_ACCEPT_UPDATE"
const updateDebugLogEnv = "PHGO_UPDATE_DEBUG_LOG"
const lastUpdateErrorEnv = "PHGO_LAST_UPDATE_ERROR"
const startupASCIIArt = `          _____                    _____                    _____                   _______
         /\    \                  /\    \                  /\    \                 /::\    \
        /::\    \                /::\____\                /::\    \               /::::\    \
       /::::\    \              /:::/    /               /::::\    \             /::::::\    \
      /::::::\    \            /:::/    /               /::::::\    \           /::::::::\    \
     /:::/\:::\    \          /:::/    /               /:::/\:::\    \         /:::/~~\:::\    \
    /:::/__\:::\    \        /:::/____/               /:::/  \:::\    \       /:::/    \:::\    \
   /::::\   \:::\    \      /::::\    \              /:::/    \:::\    \     /:::/    / \:::\    \
  /::::::\   \:::\    \    /::::::\    \   _____    /:::/    / \:::\    \   /:::/____/   \:::\____\
 /:::/\:::\   \:::\____\  /:::/\:::\    \ /\    \  /:::/    /   \:::\ ___\ |:::|    |     |:::|    |
/:::/  \:::\   \:::|    |/:::/  \:::\    /::\____\/:::/____/  ___\:::|    ||:::|____|     |:::|    |
\::/    \:::\  /:::|____|\::/    \:::\  /:::/    /\:::\    \ /\  /:::|____| \:::\    \   /:::/    /
 \/_____/\:::\/:::/    /  \/____/ \:::\/:::/    /  \:::\    /::\ \::/    /   \:::\    \ /:::/    /
          \::::::/    /            \::::::/    /    \:::\   \:::\ \/____/     \:::\    /:::/    /
           \::::/    /              \::::/    /      \:::\   \:::\____\        \:::\__/:::/    /
            \::/____/               /:::/    /        \:::\  /:::/    /         \::::::::/    /
             ~~                    /:::/    /          \:::\/:::/    /           \::::::/    /
                                  /:::/    /            \::::::/    /             \::::/    /
                                 /:::/    /              \::::/    /               \::/____/
                                 \::/    /                \::/____/                 ~~
                                  \/____/
`

var version = "dev"

func main() {
	if err := run(); err != nil {
		fatal(err)
	}
}

func run() error {
	printStartupNotice()
	printLastUpdateErrorNotice()
	appDir, err := applicationDir()
	if err != nil {
		return err
	}
	_ = startupstate.Write(appDir, startupstate.State{Status: startupstate.StatusInitializing, Message: "Startup helper is preparing phytozome GO."})
	if shouldSkipBundlePreflight() {
		_, _ = fmt.Fprintln(os.Stdout)
		_, _ = fmt.Fprintln(os.Stdout, "Bundle preflight skipped for relaunch.")
		_ = startupstate.Complete(appDir)
		return launchMainProgram(os.Args[1:])
	}

	cacheTargets, err := resolveCacheTargets()
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(os.Stdout)
	_, _ = fmt.Fprintln(os.Stdout, "Cache cleanup targets:")
	for _, target := range cacheTargets {
		_, _ = fmt.Fprintf(os.Stdout, "  - %s\n", target)
	}

	if err := runSpinner("Deleting .cache directories", func() error {
		return removeCacheTargets(cacheTargets)
	}); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(os.Stdout, "Cache cleanup complete.")
	if updateLaunched := maybeHandleReleaseUpdate(os.Args[1:]); updateLaunched {
		_ = startupstate.Complete(appDir)
		return nil
	}
	mainAlreadyLaunched, err := maybeEnsureSymbolNameDatabase(appDir, os.Args[1:])
	if err != nil {
		_ = startupstate.Complete(appDir)
		return err
	}
	_ = startupstate.Complete(appDir)
	if mainAlreadyLaunched {
		return nil
	}
	return launchMainProgram(os.Args[1:])
}

func printStartupNotice() {
	_, _ = fmt.Fprint(os.Stdout, startupASCIIArt)
	_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", displayName, version)
	_, _ = fmt.Fprintf(os.Stdout, "Author: %s\n", author)
	_, _ = fmt.Fprintf(os.Stdout, "Repo:   %s\n", repoURL)
	_, _ = fmt.Fprintf(os.Stdout, "License: %s (%s)\n", licenseName, licenseID)
}

func printLastUpdateErrorNotice() {
	message := strings.TrimSpace(os.Getenv(lastUpdateErrorEnv))
	if message == "" {
		return
	}
	_, _ = fmt.Fprintln(os.Stdout)
	_, _ = fmt.Fprintln(os.Stdout, "Previous update attempt did not fully apply.")
	_, _ = fmt.Fprintf(os.Stdout, "Reason: %s\n", message)
}

func shouldSkipBundlePreflight() bool {
	return strings.TrimSpace(os.Getenv(skipBundlePreflightEnv)) != ""
}

func runSpinner(label string, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	frames := []rune{'|', '/', '-', '\\'}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	index := 0
	for {
		select {
		case err := <-done:
			_, _ = fmt.Fprint(os.Stdout, "\r\033[2K")
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(os.Stdout, "\r%s... done.\n", label)
			return nil
		case <-ticker.C:
			_, _ = fmt.Fprintf(os.Stdout, "\r%c %s...", frames[index%len(frames)], label)
			index++
		}
	}
}

func launchMainProgramDirect(args []string) error {
	mainPath, err := resolveMainProgramPath()
	if err != nil {
		return err
	}

	cmd := exec.Command(mainPath, args...)
	cmd.Dir = filepath.Dir(mainPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start %s: %w", filepath.Base(mainPath), err)
	}
	return nil
}

func launchMainProgramInNewTab(args []string) error {
	cleanerPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate helper executable: %w", err)
	}
	mainPath, err := resolveMainProgramPathFrom(cleanerPath)
	if err != nil {
		return err
	}
	weztermCLIPath, err := resolveWezTermCLIPathFrom(cleanerPath)
	if err != nil {
		return err
	}

	cmd := exec.Command(weztermCLIPath, buildWezTermSpawnArgs(mainPath, args)...)
	cmd.Dir = filepath.Dir(weztermCLIPath)
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("open %s in new tab: %w", filepath.Base(mainPath), err)
	}
	return nil
}

func launchMainProgram(args []string) error {
	if shouldSpawnMainProgramInNewTab() {
		_, _ = fmt.Fprintln(os.Stdout, "Opening phytozome GO in a new tab...")
		return launchMainProgramInNewTab(args)
	}
	_, _ = fmt.Fprintln(os.Stdout, "Starting phytozome GO...")
	return launchMainProgramDirect(args)
}

func resolveMainProgramPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate helper executable: %w", err)
	}
	return resolveMainProgramPathFrom(exePath)
}

func resolveCacheTargets() ([]string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate helper executable: %w", err)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve current working directory: %w", err)
	}
	return resolveCacheTargetsFrom(exePath, workingDir), nil
}

func resolveWezTermCLIPathFrom(cleanerPath string) (string, error) {
	cleanerPath = strings.TrimSpace(cleanerPath)
	if cleanerPath == "" {
		return "", fmt.Errorf("helper path is empty")
	}
	dir := filepath.Dir(cleanerPath)
	candidates := []string{}
	switch runtime.GOOS {
	case "windows":
		candidates = append(candidates,
			filepath.Join(dir, windowsWezTermCLIName),
			filepath.Join(dir, "wezterm.exe"),
			filepath.Join(dir, "wezterm.bin"),
		)
	default:
		candidates = append(candidates,
			filepath.Join(dir, "wezterm-cli"),
			filepath.Join(dir, "wezterm"),
			filepath.Join(dir, "wezterm-gui"),
			filepath.Join(dir, "wezterm.AppImage"),
		)
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not locate WezTerm CLI next to the helper:\n%s", dir)
}

func resolveCacheTargetsFrom(cleanerPath string, workingDir string) []string {
	seen := make(map[string]struct{}, 4)
	targets := make([]string, 0, 4)
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		targets = append(targets, clean)
	}

	cleanerDir := filepath.Dir(strings.TrimSpace(cleanerPath))
	if cleanerDir != "" && cleanerDir != "." {
		add(filepath.Join(cleanerDir, ".cache"))
		if repoRoot, ok := detectDevRepoRootFromBundleDir(cleanerDir); ok {
			add(filepath.Join(repoRoot, ".cache"))
		}
	}
	if strings.TrimSpace(workingDir) != "" {
		add(filepath.Join(workingDir, ".cache"))
	}

	sort.Strings(targets)
	return targets
}

func detectDevRepoRootFromBundleDir(bundleDir string) (string, bool) {
	bundleDir = filepath.Clean(strings.TrimSpace(bundleDir))
	if bundleDir == "" {
		return "", false
	}
	parent := filepath.Dir(bundleDir)
	if !strings.EqualFold(filepath.Base(parent), "bin") {
		return "", false
	}
	repoRoot := filepath.Dir(parent)
	if repoRoot == "" || repoRoot == "." || samePath(repoRoot, parent) {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		return "", false
	}
	return repoRoot, true
}

func removeCacheTargets(targets []string) error {
	for _, target := range targets {
		if err := os.RemoveAll(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete cache directory %s: %w", target, err)
		}
	}
	return nil
}

func maybeEnsureSymbolNameDatabase(appDir string, args []string) (bool, error) {
	dbPath := labelname.DefaultGeneInfoDatabasePath(appDir)
	labelname.SetDefaultGeneInfoDatabasePath(dbPath)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	remote, err := labelname.FetchRemoteGeneInfoMetadata(ctx)
	if err != nil {
		return false, err
	}
	local, localErr := labelname.InspectGeneInfoDatabase(dbPath)
	if localErr != nil {
		_, _ = fmt.Fprintln(os.Stdout)
		_, _ = fmt.Fprintln(os.Stdout, "NCBI Gene symbol name library is missing.")
		_, _ = fmt.Fprintf(os.Stdout, "Database path: %s\n", dbPath)
		_, _ = fmt.Fprintf(os.Stdout, "Source: %s\n", labelname.GeneInfoURL)
		_, _ = fmt.Fprintf(os.Stdout, "Remote size: %s\n", humanBytes(remote.ContentLength))
		_, _ = fmt.Fprintf(os.Stdout, "Last modified: %s\n", firstNonEmptyText(remote.LastModifiedRaw, "unknown"))
		if !canPromptForUpdateConsent() {
			return false, fmt.Errorf("symbol name library is missing and confirmation is not interactive")
		}
		if !promptYesNo(os.Stdin, os.Stdout, "Download and build the symbol name library now? [y/N]: ") {
			_, _ = fmt.Fprintln(os.Stdout, "Skipping symbol name library download. It will be required when symbol names are used.")
			return false, nil
		}
		launchNow := promptYesNo(os.Stdin, os.Stdout, "Open phytozome GO while the symbol name library downloads? [y/N]: ")
		if launchNow {
			_ = startupstate.Write(appDir, startupstate.State{Status: startupstate.StatusDownloading, AllowUse: true, Message: "Symbol name library is downloading.", DBPath: dbPath})
			_, _ = fmt.Fprintln(os.Stdout, "Opening phytozome GO in a new tab while download continues in tab 0...")
			if err := launchMainProgramInNewTab(args); err != nil {
				return false, err
			}
		}
		if err := downloadSymbolNameDatabaseStartupWithRetry(appDir, "Downloading and building NCBI Gene symbol name library", dbPath, remote, launchNow); err != nil {
			return launchNow, err
		}
		return launchNow, nil
	}
	if remote.LastModified.IsZero() || local.LastModified.IsZero() || !remote.LastModified.After(local.LastModified) {
		_, _ = fmt.Fprintf(os.Stdout, "Symbol name library: current (%d records).\n", local.RecordCount)
		return false, nil
	}
	expiredDays := int(remote.LastModified.Sub(local.LastModified).Hours() / 24)
	if expiredDays < 0 {
		expiredDays = 0
	}
	_, _ = fmt.Fprintln(os.Stdout)
	_, _ = fmt.Fprintln(os.Stdout, "NCBI Gene symbol name library update available.")
	_, _ = fmt.Fprintf(os.Stdout, "Database path: %s\n", dbPath)
	_, _ = fmt.Fprintf(os.Stdout, "Local Last-Modified:  %s\n", firstNonEmptyText(local.LastModifiedRaw, local.LastModified.Format(time.RFC1123)))
	_, _ = fmt.Fprintf(os.Stdout, "Remote Last-Modified: %s\n", firstNonEmptyText(remote.LastModifiedRaw, remote.LastModified.Format(time.RFC1123)))
	_, _ = fmt.Fprintf(os.Stdout, "Expired by: %d days\n", expiredDays)
	_, _ = fmt.Fprintf(os.Stdout, "Remote size: %s\n", humanBytes(remote.ContentLength))
	if !canPromptForUpdateConsent() {
		_, _ = fmt.Fprintln(os.Stdout, "Skipping symbol name library update because confirmation is not interactive.")
		return false, nil
	}
	if !promptYesNo(os.Stdin, os.Stdout, "Download updated symbol name library now? [y/N]: ") {
		_, _ = fmt.Fprintln(os.Stdout, "Keeping existing symbol name library.")
		return false, nil
	}
	launchNow := promptYesNo(os.Stdin, os.Stdout, "Open phytozome GO while the symbol name library update downloads? [y/N]: ")
	if launchNow {
		_ = startupstate.Write(appDir, startupstate.State{Status: startupstate.StatusDownloading, AllowUse: true, Message: "Symbol name library update is downloading.", DBPath: dbPath})
		_, _ = fmt.Fprintln(os.Stdout, "Opening phytozome GO in a new tab while update continues in tab 0...")
		if err := launchMainProgramInNewTab(args); err != nil {
			return false, err
		}
	}
	if err := downloadSymbolNameDatabaseStartupWithRetry(appDir, "Downloading and building updated NCBI Gene symbol name library", dbPath, remote, launchNow); err != nil {
		return launchNow, err
	}
	return launchNow, nil
}

func downloadSymbolNameDatabaseStartupWithRetry(appDir string, label string, dbPath string, remote labelname.GeneInfoMetadata, allowUse bool) error {
	for {
		_ = startupstate.Write(appDir, startupstate.State{Status: startupstate.StatusDownloading, AllowUse: allowUse, Message: label, DBPath: dbPath})
		err := downloadSymbolNameDatabaseWithProgress(label, dbPath, remote)
		if err == nil {
			return nil
		}
		_, _ = fmt.Fprintln(os.Stdout)
		_, _ = fmt.Fprintf(os.Stdout, "Symbol name library download/build failed: %v\n", err)
		if !canPromptForUpdateConsent() {
			return err
		}
		switch promptRetrySkip(os.Stdin, os.Stdout, "Retry download/build, or skip startup install? [r/S]: ") {
		case "retry":
			continue
		default:
			_, _ = fmt.Fprintln(os.Stdout, "Skipping startup symbol name library install. It will be required when symbol names are used.")
			return nil
		}
	}
}

func downloadSymbolNameDatabaseWithProgress(label string, dbPath string, remote labelname.GeneInfoMetadata) error {
	_, _ = fmt.Fprintf(os.Stdout, "%s...\n", label)
	_, _ = fmt.Fprintf(os.Stdout, "Writing database to: %s\n", dbPath)
	progress := newConsoleProgress(os.Stdout)
	downloadCtx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	err := labelname.DownloadAndBuildGeneInfoDatabase(downloadCtx, dbPath, remote, labelname.DownloadOptions{
		Workers: labelname.DefaultDownloadWorkers(),
		Stdout:  os.Stdout,
		Progress: func(event labelname.GeneInfoProgress) {
			progress.Update(labelname.FormatGeneInfoProgress(event), event.Done)
		},
	})
	progress.Finish(err == nil)
	return err
}

func applicationDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate helper executable: %w", err)
	}
	return filepath.Dir(exePath), nil
}

func promptRetrySkip(input io.Reader, output io.Writer, label string) string {
	reader := bufio.NewReader(input)
	for {
		_, _ = fmt.Fprint(output, label)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "skip"
		}
		answer := strings.TrimSpace(strings.ToLower(line))
		switch answer {
		case "r", "retry":
			return "retry"
		case "", "s", "skip":
			return "skip"
		}
		_, _ = fmt.Fprintln(output, "Please enter r or s.")
		if err == io.EOF {
			return "skip"
		}
	}
}

type consoleProgress struct {
	output io.Writer
	last   string
}

func newConsoleProgress(output io.Writer) *consoleProgress {
	return &consoleProgress{output: output}
}

func (p *consoleProgress) Update(line string, done bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	p.last = line
	_, _ = fmt.Fprintf(p.output, "\r\033[2K%s", line)
	if done {
		_, _ = fmt.Fprintln(p.output)
	}
}

func (p *consoleProgress) Finish(ok bool) {
	if strings.TrimSpace(p.last) != "" {
		_, _ = fmt.Fprint(p.output, "\r\033[2K")
	}
	if ok {
		_, _ = fmt.Fprintln(p.output, "Symbol name library download/build complete.")
	}
}

func humanBytes(size int64) string {
	if size <= 0 {
		return "unknown"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", size, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resolveMainProgramPathFrom(cleanerPath string) (string, error) {
	cleanerPath = strings.TrimSpace(cleanerPath)
	if cleanerPath == "" {
		return "", fmt.Errorf("helper path is empty")
	}
	mainPath := filepath.Join(filepath.Dir(cleanerPath), mainProgramName)
	info, err := os.Stat(mainPath)
	if err != nil {
		return "", fmt.Errorf("could not locate phytozome GO core program next to the helper:\n%s", mainPath)
	}
	if info.IsDir() {
		return "", fmt.Errorf("phytozome GO main program path is a directory:\n%s", mainPath)
	}
	return mainPath, nil
}

func shouldSpawnMainProgramInNewTab() bool {
	return strings.TrimSpace(os.Getenv("WEZTERM_PANE")) != ""
}

func buildWezTermSpawnArgs(mainPath string, args []string) []string {
	mainPath = strings.TrimSpace(mainPath)
	spawnArgs := []string{
		"cli",
		"spawn",
		"--cwd", filepath.Dir(mainPath),
		"--",
		mainPath,
	}
	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			continue
		}
		spawnArgs = append(spawnArgs, arg)
	}
	return spawnArgs
}

func samePath(a string, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func fatal(err error) {
	_, _ = os.Stderr.WriteString(err.Error() + "\n")
	os.Exit(1)
}
