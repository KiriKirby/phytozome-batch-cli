package startupstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/appfs"
)

const (
	StatusInitializing = "initializing"
	StatusDownloading  = "downloading"
	StatusComplete     = "complete"
	stateFileName      = ".phgo-init-state.json"
	staleAfter         = 24 * time.Hour
)

type State struct {
	Status    string    `json:"status"`
	AllowUse  bool      `json:"allow_use"`
	Message   string    `json:"message,omitempty"`
	DBPath    string    `json:"db_path,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

func Path(appDir string) string {
	appDir = strings.TrimSpace(appDir)
	if appDir == "" {
		if resolved, err := appfs.ApplicationDir(); err == nil {
			appDir = resolved
		}
	}
	if appDir == "" {
		return stateFileName
	}
	return filepath.Join(appDir, stateFileName)
}

func Write(appDir string, state State) error {
	state.Status = strings.TrimSpace(state.Status)
	if state.Status == "" {
		state.Status = StatusInitializing
	}
	state.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return appfs.WriteFileAtomic(Path(appDir), data, 0o644)
}

func Complete(appDir string) error {
	if err := Write(appDir, State{Status: StatusComplete}); err != nil {
		return err
	}
	return os.Remove(Path(appDir))
}

func Read(appDir string) (State, bool) {
	data, err := os.ReadFile(Path(appDir))
	if err != nil {
		return State{}, false
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, false
	}
	if state.Status == "" || state.Status == StatusComplete {
		return State{}, false
	}
	if !state.UpdatedAt.IsZero() && time.Since(state.UpdatedAt) > staleAfter {
		return State{}, false
	}
	return state, true
}
