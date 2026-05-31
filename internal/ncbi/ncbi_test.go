package ncbi

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/KiriKirby/phytozome-go/internal/model"
	"github.com/KiriKirby/phytozome-go/internal/searchengine/ncbiprotein"
)

func TestParseGenPeptRecordExtractsGeneID(t *testing.T) {
	gb := `LOCUS       XP_015650724             564 aa            linear   PLN 07-AUG-2018
DEFINITION  probable 4-coumarate--CoA ligase 1 [Oryza sativa Japonica Group].
ACCESSION   XP_015650724
VERSION     XP_015650724.1
FEATURES             Location/Qualifiers
     Protein         1..564
                     /product="probable 4-coumarate--CoA ligase 1"
     CDS             1..564
                     /gene="LOC4345054"
                     /coded_by="XM_015795238.2:151..1845"
                     /db_xref="GeneID:4345054"
ORIGIN
//
`
	records := parseGenPeptRecords(gb)
	record := records[normalizeAccessionKey("XP_015650724.1")]
	if record.GeneID != "4345054" {
		t.Fatalf("GeneID = %q", record.GeneID)
	}
	if record.GeneName != "LOC4345054" {
		t.Fatalf("GeneName = %q", record.GeneName)
	}
}

func TestChooseGeneLocusPrefersLocusStyleDesignation(t *testing.T) {
	record := proteinRecord{
		GeneName: "LOC4345054",
		GeneSummary: geneSummary{
			Name:              "LOC4345054",
			OtherAliases:      "OSNPB_080245200, 4CL, 4CL1, Os4CL1",
			OtherDesignations: "4-coumarate--CoA ligase 1|LOC_Os08g14760|putative 4-coumarate--CoA ligase 1",
		},
	}
	record.LocusAliases = ncbiLocusAliases(record)
	if got := chooseGeneLocus(record); got != "Os08g14760" {
		t.Fatalf("gene locus = %q, want Os08g14760", got)
	}
}

func TestParseFastaRecords(t *testing.T) {
	records := parseFastaRecords(">XP_015650724.1 probable protein\nMPEP\nTIDE\n")
	record := records[normalizeAccessionKey("XP_015650724.1")]
	if record.Sequence != "MPEPTIDE" {
		t.Fatalf("sequence = %q", record.Sequence)
	}
	if record.Header != ">XP_015650724.1 probable protein" {
		t.Fatalf("header = %q", record.Header)
	}
}

func TestKeywordRowFromProteinRecordPreservesInlineFASTA(t *testing.T) {
	row := keywordRowFromProteinRecord("XP_1", proteinRecord{
		Accession:   "XP_1.1",
		ReplacedBy:  "NP_1",
		FastaHeader: ">XP_1.1 sample protein",
		Sequence:    "MPEPTIDE",
		RawFasta:    ">XP_1.1 sample protein\nMPEPTIDE\n",
	})
	if got := row.ExtraColumns["ncbi_protein_sequence"]; got != "MPEPTIDE" {
		t.Fatalf("ncbi_protein_sequence = %q, want MPEPTIDE", got)
	}
	if got := row.ExtraColumns["ncbi_fasta"]; !strings.Contains(got, "MPEPTIDE") {
		t.Fatalf("ncbi_fasta missing sequence: %q", got)
	}
	if got := row.ExtraColumns["ncbi_replaced_by"]; got != "NP_1" {
		t.Fatalf("ncbi_replaced_by = %q, want NP_1", got)
	}
}

func TestParseNucleotideCDSRecordsExtractsMultilineTranslation(t *testing.T) {
	records := parseNucleotideCDSRecords(`LOCUS       NM_UNIT                  900 bp    mRNA    linear   PLN 01-JAN-2026
DEFINITION  unit nucleotide record.
ACCESSION   NM_UNIT
VERSION     NM_UNIT.1
FEATURES             Location/Qualifiers
     source          1..900
                     /organism="Arabidopsis thaliana"
     CDS             10..300
                     /gene="ABC1"
                     /locus_tag="AT1G01010"
                     /product="unit protein"
                     /protein_id="XP_UNIT.1"
                     /db_xref="GeneID:12345"
                     /translation="MPEP
                     TIDE"
ORIGIN
//
`)
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	record := records[0]
	if record.NucleotideAccession != "NM_UNIT.1" {
		t.Fatalf("nucleotide accession = %q, want NM_UNIT.1", record.NucleotideAccession)
	}
	if record.Organism != "Arabidopsis thaliana" {
		t.Fatalf("organism = %q, want Arabidopsis thaliana", record.Organism)
	}
	if record.ProteinSequence != "MPEPTIDE" {
		t.Fatalf("protein sequence = %q, want MPEPTIDE", record.ProteinSequence)
	}
	if record.GeneID != "12345" || record.LocusTag != "AT1G01010" || record.ProteinID != "XP_UNIT.1" {
		t.Fatalf("unexpected CDS metadata: %#v", record)
	}
}

func TestSearchProteinRowsFallsBackToNuccoreCDSTranslation(t *testing.T) {
	const term = "unit-nuccore-fallback-translation-20260601"
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "protein":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"0","idlist":[]}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "nuccore":
			if q.Get("term") != term {
				t.Fatalf("nuccore fallback term = %q, want %q", q.Get("term"), term)
			}
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["NUC_UNIT_TRANSLATION_1"]}}`), nil
		case strings.HasSuffix(req.URL.Path, "/efetch.fcgi") && q.Get("db") == "nuccore":
			return stringResponse(http.StatusOK, `LOCUS       NM_UNIT                  900 bp    mRNA    linear   PLN 01-JAN-2026
DEFINITION  unit nucleotide record.
ACCESSION   NM_UNIT
VERSION     NM_UNIT.1
FEATURES             Location/Qualifiers
     source          1..900
                     /organism="Arabidopsis thaliana"
     CDS             10..300
                     /gene="ABC1"
                     /locus_tag="AT1G01010"
                     /product="unit protein"
                     /protein_id="XP_UNIT.1"
                     /db_xref="GeneID:12345"
                     /translation="MPEP
                     TIDE"
ORIGIN
//
`), nil
		default:
			t.Fatalf("unexpected NCBI request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})

	rows, err := client.SearchProteinRows(context.Background(), model.SpeciesCandidate{}, term, 20)
	if err != nil {
		t.Fatalf("SearchProteinRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1: %#v", len(rows), rows)
	}
	row := rows[0]
	if row.SearchType != ncbiprotein.SearchTypeNucleotideFallback {
		t.Fatalf("search type = %q, want nucleotide fallback", row.SearchType)
	}
	if row.SequenceID != "XP_UNIT.1" || row.TranscriptID != "NM_UNIT.1" {
		t.Fatalf("unexpected sequence ids: sequence=%q transcript=%q", row.SequenceID, row.TranscriptID)
	}
	if row.GeneLocus != "AT1G01010" {
		t.Fatalf("GeneLocus = %q, want AT1G01010", row.GeneLocus)
	}
	if got := row.ExtraColumns["ncbi_fasta"]; !strings.Contains(got, "MPEPTIDE") {
		t.Fatalf("ncbi_fasta missing translated protein sequence: %q", got)
	}
}

func TestSearchProteinRowsFallsBackToNuccoreProteinIDFetch(t *testing.T) {
	const term = "unit-nuccore-fallback-proteinid-20260601"
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "protein":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"0","idlist":[]}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "nuccore":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["NUC_UNIT_PROTEINID_1"]}}`), nil
		case strings.HasSuffix(req.URL.Path, "/efetch.fcgi") && q.Get("db") == "nuccore":
			return stringResponse(http.StatusOK, `LOCUS       NM_LINKED                900 bp    mRNA    linear   PLN 01-JAN-2026
DEFINITION  linked nucleotide record.
ACCESSION   NM_LINKED
VERSION     NM_LINKED.1
FEATURES             Location/Qualifiers
     source          1..900
                     /organism="Oryza sativa Japonica Group"
     CDS             10..300
                     /gene="LOC_UNIT"
                     /locus_tag="LOC_Os01g01010"
                     /product="linked protein"
                     /protein_id="XP_LINKED.1"
ORIGIN
//
`), nil
		case strings.HasSuffix(req.URL.Path, "/esummary.fcgi") && q.Get("db") == "protein":
			return stringResponse(http.StatusOK, `{"result":{"XP_LINKED.1":{"uid":"XP_LINKED.1","caption":"XP_LINKED","title":"linked protein [Oryza sativa Japonica Group]","organism":"Oryza sativa Japonica Group","accessionversion":"XP_LINKED.1","slen":8}}}`), nil
		case strings.HasSuffix(req.URL.Path, "/efetch.fcgi") && q.Get("db") == "protein" && q.Get("rettype") == "fasta":
			return stringResponse(http.StatusOK, ">XP_LINKED.1 linked protein\nMLINKED\n"), nil
		case strings.HasSuffix(req.URL.Path, "/efetch.fcgi") && q.Get("db") == "protein" && q.Get("rettype") == "gb":
			return stringResponse(http.StatusOK, `LOCUS       XP_LINKED                  8 aa            linear   PLN 01-JAN-2026
DEFINITION  linked protein [Oryza sativa Japonica Group].
ACCESSION   XP_LINKED
VERSION     XP_LINKED.1
FEATURES             Location/Qualifiers
     Protein         1..8
                     /product="linked protein"
ORIGIN
//
`), nil
		default:
			t.Fatalf("unexpected NCBI request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})

	rows, err := client.SearchProteinRows(context.Background(), model.SpeciesCandidate{}, term, 20)
	if err != nil {
		t.Fatalf("SearchProteinRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1: %#v", len(rows), rows)
	}
	row := rows[0]
	if row.SequenceID != "XP_LINKED.1" {
		t.Fatalf("SequenceID = %q, want XP_LINKED.1", row.SequenceID)
	}
	if got := row.ExtraColumns["ncbi_fasta"]; !strings.Contains(got, "MLINKED") {
		t.Fatalf("ncbi_fasta missing linked protein sequence: %q", got)
	}
}

func TestClientFallsBackToNoKeyWhenAPIKeyInvalid(t *testing.T) {
	t.Setenv("NCBI_API_KEY", "bad-key")
	requestKeys := []string{}
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestKeys = append(requestKeys, req.URL.Query().Get("api_key"))
		if len(requestKeys) == 1 {
			return stringResponse(http.StatusBadRequest, `{"error":"API key invalid"}`), nil
		}
		return stringResponse(http.StatusOK, `{"esearchresult":{"count":"0","idlist":[]}}`), nil
	})})
	if err := client.getJSON(context.Background(), "esearch.fcgi", nil, &eSearchResponse{}); err != nil {
		t.Fatalf("getJSON returned error after key fallback: %v", err)
	}
	if len(requestKeys) != 2 {
		t.Fatalf("request count = %d, want 2", len(requestKeys))
	}
	if requestKeys[0] != "bad-key" {
		t.Fatalf("first request api_key = %q, want bad-key", requestKeys[0])
	}
	if requestKeys[1] != "" {
		t.Fatalf("fallback request should omit api_key, got %q", requestKeys[1])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func stringResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestLiveProteinSearchFetchesFASTAAndGeneLocus(t *testing.T) {
	if os.Getenv("PHYTOZOME_NCBI_LIVE") == "" {
		t.Skip("set PHYTOZOME_NCBI_LIVE=1 to run live NCBI E-utilities protein search")
	}
	client := NewClient(nil)
	rows, err := client.SearchKeywordRows(context.Background(), model.SpeciesCandidate{JBrowseName: "ncbi_protein"}, "XP_015650724.1")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one NCBI protein row")
	}
	if rows[0].SequenceID == "" {
		t.Fatalf("row missing sequence id: %#v", rows[0])
	}
	if rows[0].GeneLocus != "Os08g14760" {
		t.Fatalf("GeneLocus = %q, want Os08g14760", rows[0].GeneLocus)
	}
	sequence, err := client.FetchProteinSequence(context.Background(), 0, rows[0].SequenceID)
	if err != nil {
		t.Fatalf("FetchProteinSequence returned error: %v", err)
	}
	if sequence.Sequence == "" || sequence.OriginalHeader == "" {
		t.Fatalf("incomplete sequence payload: %#v", sequence)
	}
}
