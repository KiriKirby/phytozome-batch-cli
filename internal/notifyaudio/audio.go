package notifyaudio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/KiriKirby/phytozome-go/internal/appfs"
)

const (
	startupMID = "startup.mid"
	blastMID   = "blast.mid"
	keywordMID = "keyword.mid"
	canvasMID  = "canvas.mid"
	doneMID    = "done.mid"
)

var cuePathCache sync.Map
var warmupOnce sync.Once

func init() {
	Warmup()
}

func Warmup() {
	warmupOnce.Do(func() {
		for _, fileName := range []string{startupMID, blastMID, keywordMID, canvasMID, doneMID} {
			_, _ = cuePath(fileName)
		}
	})
}

func PlayStartup() {
	playCueAsync(startupMID)
}

func PlayBlastResult() {
	playCueAsync(blastMID)
}

func PlayKeywordResult() {
	playCueAsync(keywordMID)
}

func PlayCanvas() {
	playCueAsync(canvasMID)
}

func PlayDone() {
	playCueAsync(doneMID)
}

func playCueAsync(fileName string) {
	Warmup()
	go func() {
		path, err := cuePath(fileName)
		if err != nil || strings.TrimSpace(path) == "" {
			return
		}
		_ = playMIDIFile(path)
	}()
}

func cuePath(fileName string) (string, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return "", fmt.Errorf("empty midi cue file")
	}
	if cached, ok := cuePathCache.Load(fileName); ok {
		return cached.(string), nil
	}
	appDir, err := appfs.ApplicationDir()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(appDir, "assets", "sounds", fileName),
		filepath.Join(appDir, "..", "assets", "sounds", fileName),
		filepath.Join(appDir, "..", "..", "assets", "sounds", fileName),
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "assets", "sounds", fileName))
	}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			if abs, err := filepath.Abs(candidate); err == nil {
				cuePathCache.Store(fileName, abs)
				return abs, nil
			}
			cuePathCache.Store(fileName, candidate)
			return candidate, nil
		}
	}
	return "", fmt.Errorf("midi cue %s was not found", fileName)
}
