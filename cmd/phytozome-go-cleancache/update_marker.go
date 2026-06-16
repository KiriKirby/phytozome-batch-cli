package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const updatePendingMarkerName = ".phgo-update-pending.json"
const updatePendingMarkerStaleAfter = 90 * time.Minute

type updatePendingMarker struct {
	Version   string    `json:"version,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

func updatePendingMarkerPath(appDir string) string {
	appDir = strings.TrimSpace(appDir)
	if appDir == "" {
		return updatePendingMarkerName
	}
	return filepath.Join(appDir, updatePendingMarkerName)
}

func writeUpdatePendingMarker(appDir string, version string) error {
	marker := updatePendingMarker{
		Version:   strings.TrimSpace(version),
		StartedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal update pending marker: %w", err)
	}
	path := updatePendingMarkerPath(appDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create marker directory %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write update pending marker %s: %w", path, err)
	}
	return nil
}

func removeUpdatePendingMarker(appDir string) error {
	path := updatePendingMarkerPath(appDir)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove update pending marker %s: %w", path, err)
	}
	return nil
}

func readUpdatePendingMarker(appDir string) (updatePendingMarker, bool) {
	path := updatePendingMarkerPath(appDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return updatePendingMarker{}, false
	}
	var marker updatePendingMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return updatePendingMarker{}, false
	}
	if marker.StartedAt.IsZero() {
		return updatePendingMarker{}, false
	}
	return marker, true
}

func waitForPendingBundleUpdate(appDir string) error {
	progress := newConsoleProgress(os.Stdout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	waited := false
	for {
		marker, ok := readUpdatePendingMarker(appDir)
		if !ok {
			progress.Finish("", false)
			return nil
		}
		if time.Since(marker.StartedAt) > updatePendingMarkerStaleAfter {
			_ = removeUpdatePendingMarker(appDir)
			progress.Finish("", false)
			return nil
		}
		waited = true
		waitText := "Waiting for the background updater to finish applying files..."
		if version := strings.TrimSpace(marker.Version); version != "" {
			waitText = fmt.Sprintf("Waiting for the background updater to finish applying %s...", version)
		}
		progress.Update(waitText, false)
		select {
		case <-ticker.C:
		}
		if waited {
			_ = startupStatusDuringUpdateWait(appDir, waitText)
		}
	}
}

func startupStatusDuringUpdateWait(appDir string, message string) error {
	if strings.TrimSpace(appDir) == "" {
		return nil
	}
	writeStartupStatus(appDir, "updating", false, message, "")
	return nil
}
