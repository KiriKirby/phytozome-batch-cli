package notifyaudio

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
	"sync"

	"github.com/KiriKirby/phytozome-go/internal/appfs"
)

//go:embed assets/finally.mp3
var finallyMP3 []byte

var (
	finallyPathOnce sync.Once
	finallyPath     string
	finallyPathErr  error
)

func PlayResultTableReady() {
	go func() {
		path, err := embeddedFinallyPath()
		if err != nil || path == "" {
			return
		}
		_ = playAudioFile(path)
	}()
}

func embeddedFinallyPath() (string, error) {
	finallyPathOnce.Do(func() {
		dir, err := appfs.CacheDir("audio")
		if err != nil {
			finallyPathErr = err
			return
		}
		path := filepath.Join(dir, "finally.mp3")
		if data, err := os.ReadFile(path); err == nil && bytes.Equal(data, finallyMP3) {
			finallyPath = path
			return
		}
		if err := appfs.WriteFileAtomic(path, finallyMP3, 0o644); err != nil {
			finallyPathErr = err
			return
		}
		finallyPath = path
	})
	return finallyPath, finallyPathErr
}
