package ncbi

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
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

func TestNCBIAPIKeyIsNormalizedToLowercase(t *testing.T) {
	t.Setenv("NCBI_API_KEY", "D22CF9B263C7CB8BFE07A1F7927E62588308")
	if got := ncbiAPIKey(); got != "d22cf9b263c7cb8bfe07a1f7927e62588308" {
		t.Fatalf("ncbiAPIKey() = %q", got)
	}
}

func TestFetchSpeciesCandidatesReturnsOnlySearchableNCBISearchTypes(t *testing.T) {
	client := NewClient(nil)
	candidates, err := client.FetchSpeciesCandidates(context.Background())
	if err != nil {
		t.Fatalf("FetchSpeciesCandidates returned error: %v", err)
	}
	if len(candidates) != len(SearchableSearchTypes()) {
		t.Fatalf("candidate count = %d, want %d", len(candidates), len(SearchableSearchTypes()))
	}
	if got := SearchTypeIDFromSpeciesCandidate(candidates[0]); got != "protein" {
		t.Fatalf("first synthetic search type = %q, want protein", got)
	}
	for _, forbidden := range []string{"pubmed", "pmc", "books", "mesh", "gds", "geoprofiles", "pcassay"} {
		for _, candidate := range candidates {
			if SearchTypeIDFromSpeciesCandidate(candidate) == forbidden {
				t.Fatalf("hidden search type %q should not be exposed in candidates", forbidden)
			}
		}
	}
}

func TestGenericNCBIGeneSearchBuildsGeneDomainRows(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "gene":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["12345"],"webenv":"NCBI_ENV","querykey":"1"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esummary.fcgi") && q.Get("db") == "gene":
			return stringResponse(http.StatusOK, `{"result":{"12345":{"uid":"12345","name":"PAL1","description":"phenylalanine ammonia-lyase 1","summary":"gene summary","maplocation":"chr1","organism":{"scientificname":"Arabidopsis thaliana","taxid":3702}}}}`), nil
		default:
			t.Fatalf("unexpected NCBI request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})
	rows, err := client.SearchKeywordRows(context.Background(), SyntheticSpeciesCandidate("gene"), "PAL1")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.ExtraColumns["ncbi_result_domain"] != ResultDomainGeneRecord {
		t.Fatalf("result domain = %q, want %q", row.ExtraColumns["ncbi_result_domain"], ResultDomainGeneRecord)
	}
	if row.ExtraColumns["ncbi_search_type_id"] != "gene" {
		t.Fatalf("search type id = %q, want gene", row.ExtraColumns["ncbi_search_type_id"])
	}
	if row.GeneIdentifier != "12345" || row.Symbols != "PAL1" {
		t.Fatalf("unexpected generic gene row: %#v", row)
	}
	if row.SequenceID != "" || row.ProteinID != "" {
		t.Fatalf("gene rows should not masquerade as sequence-exportable protein rows: %#v", row)
	}
	if got := row.ExtraColumns["ncbi_jump_targets"]; !strings.Contains(got, "protein(") {
		t.Fatalf("expected gene jump targets metadata, got %q", got)
	}
}

func TestGenericNCBINuccoreSearchDoesNotExposeProteinSequenceIDs(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "nuccore":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["NUC1"],"webenv":"NCBI_ENV","querykey":"1"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esummary.fcgi") && q.Get("db") == "nuccore":
			return stringResponse(http.StatusOK, `{"result":{"NUC1":{"uid":"NUC1","caption":"NC_000932.1","title":"Arabidopsis thaliana chloroplast, complete genome","organism":"Arabidopsis thaliana","accessionversion":"NC_000932.1","moltype":"genomic DNA","slen":154478}}}`), nil
		default:
			t.Fatalf("unexpected NCBI request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})
	rows, err := client.SearchKeywordRows(context.Background(), SyntheticSpeciesCandidate("nuccore"), "chloroplast")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.TranscriptID != "NC_000932.1" {
		t.Fatalf("TranscriptID = %q, want NC_000932.1", row.TranscriptID)
	}
	if row.SequenceID != "" || row.ProteinID != "" {
		t.Fatalf("nuccore summary rows should not expose protein sequence ids by default: %#v", row)
	}
	if got := row.Location; !strings.Contains(got, "genomic DNA") {
		t.Fatalf("location = %q, want moltype hint", got)
	}
}

func TestGenericNCBIClinVarSearchFallsBackThroughGeneELink(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "clinvar":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"0","idlist":[]}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "gene":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["12345"],"webenv":"NCBI_ENV","querykey":"1"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/elink.fcgi") && q.Get("dbfrom") == "gene" && q.Get("db") == "clinvar":
			return stringResponse(http.StatusOK, `{"linksets":[{"dbfrom":"gene","ids":["12345"],"linksetdbs":[{"dbto":"clinvar","linkname":"gene_clinvar","links":["987654"]}]}]}`), nil
		case strings.HasSuffix(req.URL.Path, "/esummary.fcgi") && q.Get("db") == "clinvar":
			return stringResponse(http.StatusOK, `{"result":{"987654":{"uid":"987654","accession":"RCV000000001","title":"PAL1-related variant","organism":"Arabidopsis thaliana","clinicalsignificance":"pathogenic"}}}`), nil
		case strings.HasSuffix(req.URL.Path, "/efetch.fcgi") && q.Get("db") == "clinvar":
			return stringResponse(http.StatusOK, `<ClinVarSet><Title>PAL1-related variant</Title><ReferenceClinVarAssertion><ClinicalSignificance><Description>Pathogenic</Description><ReviewStatus>reviewed by expert panel</ReviewStatus></ClinicalSignificance><TraitSet><Trait><Name><ElementValue Type="Preferred">PAL1 deficiency syndrome</ElementValue></Name></Trait></TraitSet></ReferenceClinVarAssertion><MeasureSet Type="single nucleotide variant"></MeasureSet></ClinVarSet>`), nil
		default:
			t.Fatalf("unexpected NCBI request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})
	rows, err := client.SearchKeywordRows(context.Background(), SyntheticSpeciesCandidate("clinvar"), "PAL1")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.SearchType != "NCBI Gene -> ClinVar" {
		t.Fatalf("SearchType = %q", row.SearchType)
	}
	if row.ExtraColumns["ncbi_link_resolution"] != "elink" || row.ExtraColumns["ncbi_linkname"] != "gene_clinvar" {
		t.Fatalf("missing elink provenance: %#v", row.ExtraColumns)
	}
}

func TestGenericNCBIPMCSearchFallsBackThroughPubMedELink(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "pmc":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"0","idlist":[]}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "pubmed":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["456789"],"webenv":"NCBI_ENV","querykey":"1"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/elink.fcgi") && q.Get("dbfrom") == "pubmed" && q.Get("db") == "pmc":
			return stringResponse(http.StatusOK, `{"linksets":[{"dbfrom":"pubmed","ids":["456789"],"linksetdbs":[{"dbto":"pmc","linkname":"pubmed_pmc","links":["PMC123456"]}]}]}`), nil
		case strings.HasSuffix(req.URL.Path, "/esummary.fcgi") && q.Get("db") == "pmc":
			return stringResponse(http.StatusOK, `{"result":{"PMC123456":{"uid":"PMC123456","title":"PAL1 full text article","pubdate":"2026","authors":["A Author","B Author"]}}}`), nil
		default:
			t.Fatalf("unexpected NCBI request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})
	rows, err := client.SearchKeywordRows(context.Background(), SyntheticSpeciesCandidate("pmc"), "PAL1")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if rows[0].SearchType != "NCBI PubMed -> PMC" {
		t.Fatalf("SearchType = %q", rows[0].SearchType)
	}
	if rows[0].ExtraColumns["ncbi_linkname"] != "pubmed_pmc" {
		t.Fatalf("ncbi_linkname = %q", rows[0].ExtraColumns["ncbi_linkname"])
	}
}

func TestGenericNCBIMedGenSearchFallsBackThroughClinVarELink(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "medgen":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"0","idlist":[]}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "clinvar":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["1001"],"webenv":"NCBI_ENV","querykey":"1"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/elink.fcgi") && q.Get("dbfrom") == "clinvar" && q.Get("db") == "medgen":
			return stringResponse(http.StatusOK, `{"linksets":[{"dbfrom":"clinvar","ids":["1001"],"linksetdbs":[{"dbto":"medgen","linkname":"clinvar_medgen","links":["C567890"]}]}]}`), nil
		case strings.HasSuffix(req.URL.Path, "/esummary.fcgi") && q.Get("db") == "medgen":
			return stringResponse(http.StatusOK, `{"result":{"C567890":{"uid":"C567890","conceptid":"C567890","title":"PAL1 deficiency syndrome","definition":"linked medgen concept"}}}`), nil
		default:
			t.Fatalf("unexpected NCBI request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})
	rows, err := client.SearchKeywordRows(context.Background(), SyntheticSpeciesCandidate("medgen"), "PAL1")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if rows[0].SearchType != "NCBI ClinVar -> MedGen" {
		t.Fatalf("SearchType = %q", rows[0].SearchType)
	}
	if rows[0].ExtraColumns["ncbi_linkname"] != "clinvar_medgen" {
		t.Fatalf("ncbi_linkname = %q", rows[0].ExtraColumns["ncbi_linkname"])
	}
}

func TestGenericNCBIGeneSearchFallsBackThroughMedGenELink(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "gene":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"0","idlist":[]}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "clinvar":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"0","idlist":[]}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "medgen":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["C123"],"webenv":"NCBI_ENV","querykey":"1"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/elink.fcgi") && q.Get("dbfrom") == "medgen" && q.Get("db") == "gene":
			return stringResponse(http.StatusOK, `{"linksets":[{"dbfrom":"medgen","ids":["C123"],"linksetdbs":[{"dbto":"gene","linkname":"medgen_gene_diseases","links":["8456"]}]}]}`), nil
		case strings.HasSuffix(req.URL.Path, "/esummary.fcgi") && q.Get("db") == "gene":
			return stringResponse(http.StatusOK, `{"result":{"8456":{"uid":"8456","name":"PAL1","description":"phenylalanine ammonia-lyase 1","summary":"linked from medgen","organism":{"scientificname":"Arabidopsis thaliana","taxid":3702}}}}`), nil
		default:
			t.Fatalf("unexpected NCBI request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})
	rows, err := client.SearchKeywordRows(context.Background(), SyntheticSpeciesCandidate("gene"), "PAL1 deficiency")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if rows[0].SearchType != "NCBI MedGen -> Gene" {
		t.Fatalf("SearchType = %q", rows[0].SearchType)
	}
	if rows[0].ExtraColumns["ncbi_linkname"] != "medgen_gene_diseases" {
		t.Fatalf("ncbi_linkname = %q", rows[0].ExtraColumns["ncbi_linkname"])
	}
}

func TestVisibleNCBISummaryRowBuildersPopulateLabelAndKeyExtras(t *testing.T) {
	tests := []struct {
		name         string
		specID       string
		doc          map[string]any
		wantLabel    string
		wantType     string
		wantGeneID   string
		wantGenome   string
		wantContains map[string]string
	}{
		{
			name:   "assembly",
			specID: "assembly",
			doc: map[string]any{
				"assemblyaccession": "GCF_000001405.40",
				"assemblyname":      "GRCh38.p14",
				"organism":          "Homo sapiens",
				"assemblystatus":    "latest",
				"assemblylevel":     "Chromosome",
				"submissiondate":    "2022/02/03",
				"ftppath_refseq":    "ftp://refseq/path",
			},
			wantLabel:  "GRCh38.p14",
			wantType:   "ncbi assembly name",
			wantGeneID: "GCF_000001405.40",
			wantGenome: "Homo sapiens",
			wantContains: map[string]string{
				"ncbi_assembly_accession": "GCF_000001405.40",
				"ncbi_assembly_level":     "Chromosome",
				"ncbi_ftp_path":           "ftp://refseq/path",
			},
		},
		{
			name:   "bioproject",
			specID: "bioproject",
			doc: map[string]any{
				"project_acc":       "PRJNA12345",
				"title":             "Arabidopsis stress atlas",
				"organism_name":     "Arabidopsis thaliana",
				"project_type":      "Genome sequencing",
				"project_data_type": "Raw sequence reads",
				"relevance":         "medical",
			},
			wantLabel:  "Arabidopsis stress atlas",
			wantType:   "ncbi project title",
			wantGeneID: "PRJNA12345",
			wantGenome: "Arabidopsis thaliana",
			wantContains: map[string]string{
				"ncbi_bioproject_accession": "PRJNA12345",
				"ncbi_project_type":         "Genome sequencing",
				"ncbi_project_data_type":    "Raw sequence reads",
			},
		},
		{
			name:   "biosample",
			specID: "biosample",
			doc: map[string]any{
				"accession":  "SAMN00012345",
				"title":      "leaf sample 1",
				"organism":   "Arabidopsis thaliana",
				"sampledata": "isolation_source=leaf; host=Arabidopsis thaliana; geo_loc_name=Japan: Kyoto; collection_date=2026-06-01",
			},
			wantLabel:  "leaf sample 1",
			wantType:   "ncbi sample title",
			wantGeneID: "SAMN00012345",
			wantGenome: "Arabidopsis thaliana",
			wantContains: map[string]string{
				"ncbi_biosample_accession": "SAMN00012345",
				"ncbi_isolation_source":    "leaf",
				"ncbi_geo_loc_name":        "Japan: Kyoto",
			},
		},
		{
			name:   "taxonomy",
			specID: "taxonomy",
			doc: map[string]any{
				"taxid":          "3702",
				"scientificname": "Arabidopsis thaliana",
				"commonname":     "thale cress",
				"rank":           "species",
				"lineage":        "Eukaryota; Viridiplantae",
			},
			wantLabel:  "Arabidopsis thaliana",
			wantType:   "ncbi scientific name",
			wantGeneID: "3702",
			wantGenome: "Arabidopsis thaliana",
			wantContains: map[string]string{
				"ncbi_taxonomy_id":     "3702",
				"ncbi_common_name":     "thale cress",
				"ncbi_lineage_summary": "Eukaryota; Viridiplantae",
			},
		},
		{
			name:   "sra",
			specID: "sra",
			doc: map[string]any{
				"accession":      "SRR123456",
				"title":          "PAL1 RNA-seq run",
				"organism":       "Arabidopsis thaliana",
				"expxml":         `<EXPERIMENT_PACKAGE><EXPERIMENT accession="SRX111"/><STUDY accession="SRP222"/><SAMPLE accession="SRS333"/><LIBRARY_DESCRIPTOR><LIBRARY_STRATEGY>RNA-Seq</LIBRARY_STRATEGY><LIBRARY_SOURCE>TRANSCRIPTOMIC</LIBRARY_SOURCE><LIBRARY_LAYOUT><PAIRED/></LIBRARY_LAYOUT></LIBRARY_DESCRIPTOR><PLATFORM><ILLUMINA><INSTRUMENT_MODEL>NovaSeq 6000</INSTRUMENT_MODEL></ILLUMINA></PLATFORM></EXPERIMENT_PACKAGE>`,
				"runs":           `<RUN_SET><RUN accession="SRR123456" experiment_ref="SRX111" study_ref="SRP222" sample_name="SRS333" instrument_model="NovaSeq 6000" total_spots="12345" total_bases="67890"/></RUN_SET>`,
				"bioprojectaccn": "PRJNA12345",
			},
			wantLabel:  "PAL1 RNA-seq run",
			wantType:   "ncbi sra title",
			wantGeneID: "SRR123456",
			wantGenome: "Arabidopsis thaliana",
			wantContains: map[string]string{
				"ncbi_sra_accession":        "SRR123456",
				"ncbi_library_strategy":     "RNA-Seq",
				"ncbi_bioproject_accession": "PRJNA12345",
			},
		},
		{
			name:   "clinvar",
			specID: "clinvar",
			doc: map[string]any{
				"accession":            "RCV000000001",
				"title":                "PAL1-related variant",
				"gene":                 "8456",
				"clinicalsignificance": "pathogenic",
				"reviewstatus":         "reviewed by expert panel",
				"traitset":             "PAL1 deficiency syndrome",
			},
			wantLabel:  "PAL1-related variant",
			wantType:   "ncbi clinvar title",
			wantGeneID: "8456",
			wantContains: map[string]string{
				"ncbi_clinvar_accession":     "RCV000000001",
				"ncbi_clinical_significance": "pathogenic",
				"ncbi_review_status":         "reviewed by expert panel",
			},
		},
		{
			name:   "snp",
			specID: "snp",
			doc: map[string]any{
				"caption":               "rs123",
				"title":                 "PAL1 missense variant",
				"geneid":                "8456",
				"organism":              "Arabidopsis thaliana",
				"snp_class":             "snp",
				"clinical_significance": "likely pathogenic",
			},
			wantLabel:  "rs123",
			wantType:   "ncbi rsid",
			wantGeneID: "8456",
			wantGenome: "Arabidopsis thaliana",
			wantContains: map[string]string{
				"ncbi_rsid":                  "rs123",
				"ncbi_variant_type":          "snp",
				"ncbi_variant_class":         "snp",
				"ncbi_clinical_significance": "likely pathogenic",
			},
		},
		{
			name:   "dbvar",
			specID: "dbvar",
			doc: map[string]any{
				"accession":          "nsv10001",
				"title":              "PAL1 structural variant",
				"geneid":             "8456",
				"organism":           "Arabidopsis thaliana",
				"variant_type":       "copy number loss",
				"clinical_assertion": "pathogenic",
				"phenotype":          "PAL1 deficiency",
			},
			wantLabel:  "nsv10001",
			wantType:   "ncbi dbvar accession",
			wantGeneID: "8456",
			wantGenome: "Arabidopsis thaliana",
			wantContains: map[string]string{
				"ncbi_dbvar_accession":    "nsv10001",
				"ncbi_variant_type":       "copy number loss",
				"ncbi_clinical_assertion": "pathogenic",
			},
		},
		{
			name:   "medgen",
			specID: "medgen",
			doc: map[string]any{
				"conceptid":  "C567890",
				"title":      "PAL1 deficiency syndrome",
				"gene":       "8456",
				"definition": "A concept definition",
				"source":     "MedGen",
				"summary":    "Condition summary text",
			},
			wantLabel:  "PAL1 deficiency syndrome",
			wantType:   "ncbi medgen preferred title",
			wantGeneID: "8456",
			wantContains: map[string]string{
				"ncbi_medgen_id":            "C567890",
				"ncbi_condition_summary":    "Condition summary text",
				"ncbi_related_gene_summary": "8456",
			},
		},
		{
			name:   "gtr",
			specID: "gtr",
			doc: map[string]any{
				"accession": "GTR000001",
				"title":     "PAL1 panel test",
				"gene":      "8456",
				"condition": "PAL1 deficiency syndrome",
				"testtype":  "Panel",
				"method":    "Sequencing",
				"labname":   "NCBI Test Lab",
			},
			wantLabel:  "PAL1 panel test",
			wantType:   "ncbi gtr test name",
			wantGeneID: "8456",
			wantContains: map[string]string{
				"ncbi_gtr_accession": "GTR000001",
				"ncbi_condition":     "PAL1 deficiency syndrome",
				"ncbi_lab":           "NCBI Test Lab",
			},
		},
		{
			name:   "omim",
			specID: "omim",
			doc: map[string]any{
				"omimid":  "123456",
				"title":   "PAL1 deficiency syndrome",
				"gene":    "8456",
				"summary": "OMIM condition summary",
				"text":    "Longer OMIM text",
			},
			wantLabel:  "PAL1 deficiency syndrome",
			wantType:   "ncbi omim title",
			wantGeneID: "8456",
			wantContains: map[string]string{
				"ncbi_omim_id":           "123456",
				"ncbi_condition_summary": "OMIM condition summary",
				"ncbi_omim_text":         "Longer OMIM text",
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := keywordRowFromGenericSummary(SearchTypeByID(tt.specID), "PAL1", strconv.Itoa(i+1), i+1, len(tests), tt.doc)
			if row.LabelName != tt.wantLabel {
				t.Fatalf("LabelName = %q, want %q", row.LabelName, tt.wantLabel)
			}
			if row.LabelNameType != tt.wantType {
				t.Fatalf("LabelNameType = %q, want %q", row.LabelNameType, tt.wantType)
			}
			if row.GeneIdentifier != tt.wantGeneID {
				t.Fatalf("GeneIdentifier = %q, want %q", row.GeneIdentifier, tt.wantGeneID)
			}
			if tt.wantGenome != "" && row.Genome != tt.wantGenome {
				t.Fatalf("Genome = %q, want %q", row.Genome, tt.wantGenome)
			}
			if row.ExtraColumns["ncbi_search_type_id"] != tt.specID {
				t.Fatalf("ncbi_search_type_id = %q, want %q", row.ExtraColumns["ncbi_search_type_id"], tt.specID)
			}
			for key, want := range tt.wantContains {
				if got := row.ExtraColumns[key]; got != want {
					t.Fatalf("%s = %q, want %q", key, got, want)
				}
			}
			if strings.TrimSpace(row.AutoDefine) == "" {
				t.Fatalf("AutoDefine should not be blank: %#v", row)
			}
			if strings.TrimSpace(row.GeneReportURL) == "" {
				t.Fatalf("GeneReportURL should not be blank: %#v", row)
			}
		})
	}
}

func TestParseBioSampleAttributeBagExtractsNormalizedPairs(t *testing.T) {
	attrs := parseBioSampleAttributeBag("isolation_source=leaf; host=Arabidopsis thaliana; geo_loc_name=Japan: Kyoto")
	if attrs["isolation_source"] != "leaf" {
		t.Fatalf("isolation_source = %q", attrs["isolation_source"])
	}
	if attrs["host"] != "Arabidopsis thaliana" {
		t.Fatalf("host = %q", attrs["host"])
	}
	if attrs["geo_loc_name"] != "Japan: Kyoto" {
		t.Fatalf("geo_loc_name = %q", attrs["geo_loc_name"])
	}
}

func TestParseSRAExperimentXMLExtractsHierarchy(t *testing.T) {
	meta := parseSRAExperimentXML(`<EXPERIMENT_PACKAGE><EXPERIMENT accession="SRX111"/><STUDY accession="SRP222"/><SAMPLE accession="SRS333"/><LIBRARY_DESCRIPTOR><LIBRARY_STRATEGY>RNA-Seq</LIBRARY_STRATEGY><LIBRARY_SOURCE>TRANSCRIPTOMIC</LIBRARY_SOURCE><LIBRARY_LAYOUT><PAIRED/></LIBRARY_LAYOUT></LIBRARY_DESCRIPTOR><PLATFORM><ILLUMINA><INSTRUMENT_MODEL>NovaSeq 6000</INSTRUMENT_MODEL></ILLUMINA></PLATFORM></EXPERIMENT_PACKAGE>`)
	if meta.ExperimentAccession != "SRX111" || meta.StudyAccession != "SRP222" || meta.BioSampleAccession != "SRS333" {
		t.Fatalf("unexpected experiment meta: %#v", meta)
	}
	if meta.LibraryStrategy != "RNA-Seq" || meta.LibrarySource != "TRANSCRIPTOMIC" || meta.Platform != "ILLUMINA" || meta.Layout != "PAIRED" {
		t.Fatalf("unexpected library/platform meta: %#v", meta)
	}
}

func TestParseSRARunsExtractsPrimaryRun(t *testing.T) {
	meta := parseSRARuns(`<RUN_SET><RUN accession="SRR123456" experiment_ref="SRX111" study_ref="SRP222" sample_name="SRS333" instrument_model="NovaSeq 6000" total_spots="12345" total_bases="67890"/></RUN_SET>`)
	if meta.RunAccession != "SRR123456" || meta.ExperimentAccession != "SRX111" || meta.StudyAccession != "SRP222" {
		t.Fatalf("unexpected run meta: %#v", meta)
	}
	if meta.Spots != "12345" || meta.Bases != "67890" {
		t.Fatalf("unexpected counts: %#v", meta)
	}
}

func TestGenericClinVarRowsAreEnrichedByEFetchXML(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "clinvar":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["1"],"webenv":"NCBI_ENV","querykey":"1"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esummary.fcgi") && q.Get("db") == "clinvar":
			return stringResponse(http.StatusOK, `{"result":{"1":{"uid":"1","accession":"RCV000000001","title":"PAL1-related variant","gene":"8456","clinicalsignificance":"pathogenic"}}}`), nil
		case strings.HasSuffix(req.URL.Path, "/efetch.fcgi") && q.Get("db") == "clinvar":
			return stringResponse(http.StatusOK, `<ClinVarSet><Title>PAL1-related variant</Title><ReferenceClinVarAssertion><ClinicalSignificance><Description>Pathogenic</Description><ReviewStatus>reviewed by expert panel</ReviewStatus></ClinicalSignificance><TraitSet><Trait><Name><ElementValue Type="Preferred">PAL1 deficiency syndrome</ElementValue></Name></Trait></TraitSet></ReferenceClinVarAssertion><MeasureSet Type="single nucleotide variant"></MeasureSet></ClinVarSet>`), nil
		default:
			t.Fatalf("unexpected request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})
	rows, err := client.SearchKeywordRows(context.Background(), SyntheticSpeciesCandidate("clinvar"), "PAL1")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d", len(rows))
	}
	row := rows[0]
	if got := row.ExtraColumns["ncbi_review_status"]; got != "reviewed by expert panel" {
		t.Fatalf("ncbi_review_status = %q", got)
	}
	if got := row.ExtraColumns["ncbi_condition"]; got != "PAL1 deficiency syndrome" {
		t.Fatalf("ncbi_condition = %q", got)
	}
	if got := row.ExtraColumns["ncbi_variant_type"]; got != "single nucleotide variant" {
		t.Fatalf("ncbi_variant_type = %q", got)
	}
}

func TestGenericGTRRowsAreEnrichedByEFetchXML(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "gtr":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["1"],"webenv":"NCBI_ENV","querykey":"1"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esummary.fcgi") && q.Get("db") == "gtr":
			return stringResponse(http.StatusOK, `{"result":{"1":{"uid":"1","accession":"GTR000001","title":"PAL1 panel test","gene":"8456"}}}`), nil
		case strings.HasSuffix(req.URL.Path, "/efetch.fcgi") && q.Get("db") == "gtr":
			return stringResponse(http.StatusOK, `<TestReport><Test><Name>PAL1 panel test</Name><ClinicalDomain><Disease><Name>PAL1 deficiency syndrome</Name></Disease></ClinicalDomain><Method><MethodCategory>Sequencing</MethodCategory></Method><Laboratory><Name>NCBI Test Lab</Name></Laboratory></Test></TestReport>`), nil
		default:
			t.Fatalf("unexpected request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})
	rows, err := client.SearchKeywordRows(context.Background(), SyntheticSpeciesCandidate("gtr"), "PAL1")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d", len(rows))
	}
	row := rows[0]
	if got := row.ExtraColumns["ncbi_condition"]; got != "PAL1 deficiency syndrome" {
		t.Fatalf("ncbi_condition = %q", got)
	}
	if got := row.ExtraColumns["ncbi_method"]; got != "Sequencing" {
		t.Fatalf("ncbi_method = %q", got)
	}
	if got := row.ExtraColumns["ncbi_lab"]; got != "NCBI Test Lab" {
		t.Fatalf("ncbi_lab = %q", got)
	}
}

func TestGenericBioProjectRowsAreEnrichedByEFetchXML(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		switch {
		case strings.HasSuffix(req.URL.Path, "/esearch.fcgi") && q.Get("db") == "bioproject":
			return stringResponse(http.StatusOK, `{"esearchresult":{"count":"1","idlist":["12345"],"webenv":"NCBI_ENV","querykey":"1"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/esummary.fcgi") && q.Get("db") == "bioproject":
			return stringResponse(http.StatusOK, `{"result":{"12345":{"uid":"12345","project_acc":"PRJNA12345","title":"Arabidopsis stress atlas","organism_name":"Arabidopsis thaliana","project_type":"Genome sequencing"}}}`), nil
		case strings.HasSuffix(req.URL.Path, "/efetch.fcgi") && q.Get("db") == "bioproject":
			return stringResponse(http.StatusOK, `<?xml version="1.0" ?><RecordSet><DocumentSummary uid="12345"><Project><ProjectID><ArchiveID accession="PRJNA12345" archive="NCBI" id="12345"/></ProjectID><ProjectDescr><Title>Arabidopsis stress atlas</Title><Description>Project description text</Description></ProjectDescr><ProjectType><ProjectTypeSubmission><Target material="eGenome" sample_scope="eMonoisolate"><Organism taxID="3702"><OrganismName>Arabidopsis thaliana</OrganismName></Organism></Target><Objectives><Data data_type="eSequence">Sequence</Data></Objectives></ProjectTypeSubmission></ProjectType></Project><Submission submitted="2024-01-05"><Description><Organization><Name>NCBI Submitter</Name></Organization><Access>public</Access></Description></Submission></DocumentSummary></RecordSet>`), nil
		default:
			t.Fatalf("unexpected request: %s?%s", req.URL.Path, req.URL.RawQuery)
			return stringResponse(http.StatusInternalServerError, ""), nil
		}
	})})
	rows, err := client.SearchKeywordRows(context.Background(), SyntheticSpeciesCandidate("bioproject"), "stress")
	if err != nil {
		t.Fatalf("SearchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d", len(rows))
	}
	row := rows[0]
	if got := row.ExtraColumns["ncbi_project_description"]; got != "Project description text" {
		t.Fatalf("ncbi_project_description = %q", got)
	}
	if got := row.ExtraColumns["ncbi_project_data_type"]; got != "eSequence" {
		t.Fatalf("ncbi_project_data_type = %q", got)
	}
	if got := row.ExtraColumns["ncbi_submitter"]; got != "NCBI Submitter" {
		t.Fatalf("ncbi_submitter = %q", got)
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
