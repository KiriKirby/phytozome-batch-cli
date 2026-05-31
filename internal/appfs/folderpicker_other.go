//go:build !windows

package appfs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func selectFolderSystem(title string, defaultDir string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Select export folder"
	}
	if runtime.GOOS == "darwin" {
		return selectFolderDarwin(title, defaultDir)
	}
	return selectFolderUnix(title, defaultDir)
}

func selectFolderDarwin(title string, defaultDir string) (string, error) {
	osascript, err := exec.LookPath("osascript")
	if err != nil {
		return "", err
	}
	script := `POSIX path of (choose folder with prompt (system attribute "PHGO_FOLDER_PICKER_TITLE") default location POSIX file (system attribute "PHGO_FOLDER_PICKER_DEFAULT"))`
	cmd := exec.Command(osascript, "-e", script)
	cmd.Env = append(os.Environ(), "PHGO_FOLDER_PICKER_TITLE="+title, "PHGO_FOLDER_PICKER_DEFAULT="+defaultDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out)), "user canceled") {
			return "", ErrFolderSelectionCancelled
		}
		return "", fmt.Errorf("open macOS folder picker: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func selectFolderUnix(title string, defaultDir string) (string, error) {
	if zenity, err := exec.LookPath("zenity"); err == nil {
		filename := filepath.Clean(defaultDir) + string(filepath.Separator)
		out, cmdErr := exec.Command(zenity, "--file-selection", "--directory", "--title", title, "--filename", filename).CombinedOutput()
		if cmdErr != nil {
			if exitErr, ok := cmdErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return "", ErrFolderSelectionCancelled
			}
			return "", fmt.Errorf("open zenity folder picker: %w: %s", cmdErr, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	}
	if kdialog, err := exec.LookPath("kdialog"); err == nil {
		out, cmdErr := exec.Command(kdialog, "--title", title, "--getexistingdirectory", defaultDir).CombinedOutput()
		if cmdErr != nil {
			return "", fmt.Errorf("open kdialog folder picker: %w: %s", cmdErr, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	}
	return "", fmt.Errorf("no supported system folder picker found")
}

func selectFileSystem(title string, defaultDir string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Open file"
	}
	if runtime.GOOS == "darwin" {
		return selectFileDarwin(title, defaultDir)
	}
	return selectFileUnix(title, defaultDir)
}

func selectFileDarwin(title string, defaultDir string) (string, error) {
	osascript, err := exec.LookPath("osascript")
	if err != nil {
		return "", err
	}
	script := `POSIX path of (choose file with prompt (system attribute "PHGO_FILE_PICKER_TITLE") default location POSIX file (system attribute "PHGO_FILE_PICKER_DEFAULT"))`
	cmd := exec.Command(osascript, "-e", script)
	cmd.Env = append(os.Environ(), "PHGO_FILE_PICKER_TITLE="+title, "PHGO_FILE_PICKER_DEFAULT="+defaultDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out)), "user canceled") {
			return "", ErrFileSelectionCancelled
		}
		return "", fmt.Errorf("open macOS file picker: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func selectFileUnix(title string, defaultDir string) (string, error) {
	if zenity, err := exec.LookPath("zenity"); err == nil {
		filename := filepath.Clean(defaultDir) + string(filepath.Separator)
		out, cmdErr := exec.Command(zenity, "--file-selection", "--title", title, "--filename", filename).CombinedOutput()
		if cmdErr != nil {
			if exitErr, ok := cmdErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return "", ErrFileSelectionCancelled
			}
			return "", fmt.Errorf("open zenity file picker: %w: %s", cmdErr, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	}
	if kdialog, err := exec.LookPath("kdialog"); err == nil {
		out, cmdErr := exec.Command(kdialog, "--title", title, "--getopenfilename", defaultDir).CombinedOutput()
		if cmdErr != nil {
			return "", fmt.Errorf("open kdialog file picker: %w: %s", cmdErr, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	}
	return "", fmt.Errorf("no supported system file picker found")
}
