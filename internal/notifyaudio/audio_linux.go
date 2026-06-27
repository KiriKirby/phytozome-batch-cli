//go:build linux

package notifyaudio

import (
	"fmt"
	"os"
	"path/filepath"
	"os/exec"
)

func playMIDIBytes(data []byte) error {
	path, err := writeTempMIDI(data)
	if err != nil {
		return err
	}
	defer os.Remove(path)
	commands := [][]string{
		{"aplaymidi", path},
		{"timidity", path},
		{"ffplay", "-autoexit", "-nodisp", "-loglevel", "quiet", path},
		{"cvlc", "--play-and-exit", "--intf", "dummy", "--quiet", path},
		{"vlc", "--play-and-exit", "--intf", "dummy", "--quiet", path},
	}
	var lastErr error
	for _, candidate := range commands {
		if len(candidate) == 0 {
			continue
		}
		if _, err := exec.LookPath(candidate[0]); err != nil {
			lastErr = err
			continue
		}
		if err := exec.Command(candidate[0], candidate[1:]...).Run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no midi player is available")
	}
	return lastErr
}

func writeTempMIDI(data []byte) (string, error) {
	file, err := os.CreateTemp("", "phytozome-go-*.mid")
	if err != nil {
		return "", err
	}
	path := filepath.Clean(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}
