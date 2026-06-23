//go:build darwin

package notifyaudio

import "os/exec"

func playMIDIFile(path string) error {
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
