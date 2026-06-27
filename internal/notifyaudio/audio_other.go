//go:build !windows && !darwin && !linux

package notifyaudio

func playMIDIBytes(data []byte) error {
	return nil
}
