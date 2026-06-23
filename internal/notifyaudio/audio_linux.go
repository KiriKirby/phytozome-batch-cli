//go:build linux

package notifyaudio

import (
	"fmt"
	"os/exec"
)

func playMIDIFile(path string) error {
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
