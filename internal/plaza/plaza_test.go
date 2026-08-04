package plaza

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchGeneLocusBuildsPLAZARowFromPublicReleaseFiles(t *testing.T) {
	files := map[string]string{
		"/dicots/SpeciesInformation/species_information.csv.gz":   "#species\tcommon_name\ttax_id\tsource\nath\tArabidopsis thaliana\t3702\tTAIR10\n",
		"/dicots/GeneFamilies/genefamily_data.HOMFAM.csv.gz":      "#gf_id\tspecies\tgene_id\nHOM05D000010\tath\tAT1G01010\n",
		"/dicots/IdConversion/id_conversion.ath.csv.gz":           "AT1G01010\tAlias\tANAC001,NAC domain containing protein 1\nAT1G01010\tsymbol\tNAC001\nAT1G01010\ttid\tAT1G01010.1\nAT1G01010\tuniprot\tQ0WV96\n",
		"/dicots/Descriptions/gene_description.ath.csv.gz":        "AT1G01010\tdescription\tNAC domain containing protein 1\n",
		"/dicots/Fasta/proteome.selected_transcript.ath.fasta.gz": ">AT1G01010.1 | AT1G01010\nMPEPTIDE\n",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(gzipText(t, body))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.instances = []instance{{Name: "dicots_05", Base: server.URL + "/dicots"}}
	rows, err := client.SearchGeneLocus(context.Background(), "AT1G01010")
	if err != nil {
		t.Fatalf("SearchGeneLocus returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.SourceDatabase != "plaza" || row.SearchType != searchTypeGeneLocusPriority {
		t.Fatalf("source/search type = %q/%q", row.SourceDatabase, row.SearchType)
	}
	if row.GeneLocus != "AT1G01010" || row.TranscriptID != "AT1G01010.1" || row.UniProt != "Q0WV96" {
		t.Fatalf("unexpected identifiers: %#v", row)
	}
	if row.Genome != "Arabidopsis thaliana" || row.Description != "NAC domain containing protein 1" {
		t.Fatalf("unexpected annotation: genome=%q description=%q", row.Genome, row.Description)
	}
	if got := row.ExtraColumns["plaza_fasta"]; !strings.Contains(got, "MPEPTIDE") {
		t.Fatalf("plaza FASTA = %q", got)
	}
	if row.GeneReportURL != "" {
		t.Fatalf("GeneReportURL must not be synthesized, got %q", row.GeneReportURL)
	}
}

func TestSearchGeneLocusReturnsNoRowsForUnknownCanonicalGene(t *testing.T) {
	client := NewClient(http.DefaultClient)
	client.geneIndex = map[string][]geneCandidate{"known": nil}
	rows, err := client.SearchGeneLocus(context.Background(), "missing")
	if err != nil {
		t.Fatalf("SearchGeneLocus returned error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("row count = %d, want 0", len(rows))
	}
}

func gzipText(t *testing.T, value string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write([]byte(value)); err != nil {
		t.Fatalf("write gzip fixture: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip fixture: %v", err)
	}
	return buffer.Bytes()
}
