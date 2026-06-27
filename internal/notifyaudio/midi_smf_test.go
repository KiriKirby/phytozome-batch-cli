package notifyaudio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRepositoryMIDICues(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "assets", "sounds", "*.mid"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no MIDI cue files found")
	}
	for _, file := range files {
		file := file
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			seq, err := parseSMF(data)
			if err != nil {
				t.Fatal(err)
			}
			if seq.division == 0 {
				t.Fatalf("division is zero")
			}
			if len(seq.events) == 0 {
				t.Fatalf("no playable events parsed")
			}
		})
	}
}

func TestParseSMFRunningStatus(t *testing.T) {
	data := []byte{
		'M', 'T', 'h', 'd', 0, 0, 0, 6, 0, 0, 0, 1, 0, 96,
		'M', 'T', 'r', 'k', 0, 0, 0, 11,
		0x00, 0x90, 60, 64,
		0x60, 60, 0,
		0x00, 0xFF, 0x2F, 0x00,
	}
	seq, err := parseSMF(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(seq.events) != 2 {
		t.Fatalf("events = %d, want 2", len(seq.events))
	}
	if seq.events[1].tick != 96 || seq.events[1].status != 0x90 || seq.events[1].data1 != 60 || seq.events[1].data2 != 0 {
		t.Fatalf("running-status event parsed incorrectly: %+v", seq.events[1])
	}
}
