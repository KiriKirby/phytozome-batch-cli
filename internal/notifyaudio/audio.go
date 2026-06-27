package notifyaudio

import (
	_ "embed"
	"fmt"
	"strings"
)

const (
	startupMID = "startup.mid"
	blastMID   = "blast.mid"
	keywordMID = "keyword.mid"
	canvasMID  = "canvas.mid"
	doneMID    = "done.mid"
)

//go:embed cues/startup.mid
var startupMIDI []byte

//go:embed cues/blast.mid
var blastMIDI []byte

//go:embed cues/keyword.mid
var keywordMIDI []byte

//go:embed cues/canvas.mid
var canvasMIDI []byte

//go:embed cues/done.mid
var doneMIDI []byte

func Warmup() {
	for _, fileName := range []string{startupMID, blastMID, keywordMID, canvasMID, doneMID} {
		_, _ = cueBytes(fileName)
	}
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
	go func() {
		data, err := cueBytes(fileName)
		if err != nil || len(data) == 0 {
			return
		}
		_ = playMIDIBytes(data)
	}()
}

func cueBytes(fileName string) ([]byte, error) {
	switch strings.TrimSpace(fileName) {
	case startupMID:
		return startupMIDI, nil
	case blastMID:
		return blastMIDI, nil
	case keywordMID:
		return keywordMIDI, nil
	case canvasMID:
		return canvasMIDI, nil
	case doneMID:
		return doneMIDI, nil
	default:
		return nil, fmt.Errorf("embedded midi cue %s was not found", fileName)
	}
}

func init() {
	Warmup()
}
