//go:build !windows

package notifyaudio

func playAudioFile(path string) error {
	return nil
}
