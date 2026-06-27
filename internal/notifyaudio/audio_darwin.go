//go:build darwin

package notifyaudio

import (
	"os"
	"os/exec"
	"path/filepath"
)

func playMIDIBytes(data []byte) error {
	path, err := writeTempMIDI(data)
	if err != nil {
		return err
	}
	defer os.Remove(path)
	script := []string{
		`on run argv`,
		`  set midiPath to POSIX file (item 1 of argv)`,
		`  tell application "QuickTime Player"`,
		`    set docRef to open midiPath`,
		`    play docRef`,
		`    repeat while playing of docRef`,
		`      delay 0.1`,
		`    end repeat`,
		`    close docRef saving no`,
		`    if (count of documents) is 0 then`,
		`      quit`,
		`    end if`,
		`  end tell`,
		`end run`,
	}
	args := make([]string, 0, len(script)*2+1)
	for _, line := range script {
		args = append(args, "-e", line)
	}
	args = append(args, path)
	return exec.Command("osascript", args...).Run()
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
