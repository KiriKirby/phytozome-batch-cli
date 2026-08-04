package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KiriKirby/phytozome-go/internal/model"
)

func TestWriteProteinSequencesTextNormalizesAllHeaderWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.fasta")
	err := WriteProteinSequencesText(path, []model.ProteinSequenceRecord{{
		Header:   ">source original\theader",
		Sequence: "MPEPTIDE",
	}})
	if err != nil {
		t.Fatalf("WriteProteinSequencesText: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	line := strings.SplitN(string(content), "\n", 2)[0]
	if line != ">source_original_header" {
		t.Fatalf("header = %q", line)
	}
}
