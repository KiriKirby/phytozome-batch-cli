//go:build !windows && !darwin && !linux

package notifyaudio

func playMIDIFile(path string) error {
	return nil
}
