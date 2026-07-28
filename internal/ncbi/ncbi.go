package ncbi

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/model"
	"github.com/KiriKirby/phytozome-go/internal/netconfig"
	"github.com/KiriKirby/phytozome-go/internal/searchengine/ncbiprotein"
	"golang.org/x/sync/singleflight"
)

const (
	eutilsBaseURL       = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"
	defaultRetMax       = 20
	maxNCBITextResponse = 32 << 20
	defaultNCBIAPIKey   = "D22CF9B263C7CB8BFE07A1F7927E62588308"
	proteinRecordSchema = "ncbiprotein-record-v4"
	genericRecordSchema = "ncbi-generic-record-v1"
)

var (
	ncbiThrottleMu         sync.Mutex
	ncbiLastRequest        time.Time
	errNoUsableProteinRows = errors.New("NCBI protein search returned no usable protein FASTA")
	geneIDPattern          = regexp.MustCompile(`(?i)/db_xref="GeneID:(\d+)"`)
	qualifierPattern       = regexp.MustCompile(`^\s*/([A-Za-z_]+)="?(.*?)"?$`)
	locusLikePattern       = regexp.MustCompile(`(?i)^(?:LOC_)?(?:AT\dG\d+|[A-Z][A-Za-z0-9_.-]*[Gg]\d+[A-Za-z0-9_.-]*)$`)
	locNumberPattern       = regexp.MustCompile(`(?i)^LOC\d+$`)
)

type Client struct {
	baseHTTP *http.Client
	apiKey   string

	mu            sync.RWMutex
	apiKeyInvalid bool
	proteinByID   map[string]proteinRecord
	sequenceCache map[string]model.ProteinSequenceData
	keywordEngine *ncbiprotein.Engine
	sf            singleflight.Group
}

type proteinRecord struct {
	UID          string
	Accession    string
	Title        string
	Organism     string
	TaxID        int
	Length       int
	SourceDB     string
	Status       string
	ReplacedBy   string
	CreatedAt    string
	UpdatedAt    string
	FastaHeader  string
	Sequence     string
	Definition   string
	GeneID       string
	GeneName     string
	LocusTag     string
	Product      string
	CodedBy      string
	GeneSummary  geneSummary
	GeneLocus    string
	LabelAliases []string
	LocusAliases []string
	RawGenPept   string
	RawFasta     string
}

type geneSummary struct {
	UID               string
	Name              string
	Description       string
	OtherAliases      string
	OtherDesignations string
	Nomenclature      string
	Chromosome        string
	MapLocation       string
	Summary           string
	Organism          string
	TaxID             int
}

type eSearchResponse struct {
	SearchResult struct {
		Count string   `json:"count"`
		IDs   []string `json:"idlist"`
	} `json:"esearchresult"`
}

type eSummaryProteinResponse struct {
	Result map[string]json.RawMessage `json:"result"`
}

type eSummaryProteinDoc struct {
	UID              string `json:"uid"`
	Caption          string `json:"caption"`
	Title            string `json:"title"`
	Extra            string `json:"extra"`
	GI               int64  `json:"gi"`
	CreateDate       string `json:"createdate"`
	UpdateDate       string `json:"updatedate"`
	TaxID            int    `json:"taxid"`
	Length           int    `json:"slen"`
	MolType          string `json:"moltype"`
	SourceDB         string `json:"sourcedb"`
	Genome           string `json:"genome"`
	Status           string `json:"status"`
	ReplacedBy       string `json:"replacedby"`
	Organism         string `json:"organism"`
	AccessionVersion string `json:"accessionversion"`
}

type eSummaryGeneResponse struct {
	Result map[string]json.RawMessage `json:"result"`
}

type eSummaryGeneDoc struct {
	UID               string `json:"uid"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	Chromosome        string `json:"chromosome"`
	MapLocation       string `json:"maplocation"`
	OtherAliases      string `json:"otheraliases"`
	OtherDesignations string `json:"otherdesignations"`
	Nomenclature      string `json:"nomenclaturesymbol"`
	Summary           string `json:"summary"`
	Organism          struct {
		ScientificName string `json:"scientificname"`
		TaxID          int    `json:"taxid"`
	} `json:"organism"`
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = netconfig.DefaultHTTPClient()
	}
	client := &Client{
		baseHTTP:      httpClient,
		apiKey:        ncbiAPIKey(),
		proteinByID:   make(map[string]proteinRecord),
		sequenceCache: make(map[string]model.ProteinSequenceData),
	}
	client.keywordEngine = ncbiprotein.New(client)
	return client
}

func (c *Client) Name() string {
	return "ncbi"
}

func (c *Client) FetchSpeciesCandidates(ctx context.Context) ([]model.SpeciesCandidate, error) {
	_ = ctx
	specs := SearchableSearchTypes()
	out := make([]model.SpeciesCandidate, 0, len(specs))
	for _, spec := range specs {
		candidate := SyntheticSpeciesCandidate(spec.ID)
		candidate.CommonName = spec.Description
		out = append(out, candidate)
	}
	return out, nil
}

func (c *Client) SearchKeywordRows(ctx context.Context, species model.SpeciesCandidate, keyword string) ([]model.KeywordResultRow, error) {
	if c.keywordEngine == nil {
		c.keywordEngine = ncbiprotein.New(c)
	}
	if SearchTypeIDFromSpeciesCandidate(species) != "protein" {
		return c.searchGenericKeywordRows(ctx, species, keyword)
	}
	return c.keywordEngine.SearchKeywordRows(ctx, species, keyword)
}

func (c *Client) SearchProteinRows(ctx context.Context, species model.SpeciesCandidate, term string, limit int) ([]model.KeywordResultRow, error) {
	_ = species
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultRetMax
	}
	rows, err := c.searchProteinRowsFromProteinDB(ctx, term, limit)
	if err != nil && !errors.Is(err, errNoUsableProteinRows) {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	rows, err = c.searchProteinRowsFromNucleotideDB(ctx, term, limit)
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	return c.searchProteinRowsFromGeneDB(ctx, term, limit)
}

func (c *Client) searchProteinRowsFromProteinDB(ctx context.Context, term string, limit int) ([]model.KeywordResultRow, error) {
	ids, err := c.searchProteinIDs(ctx, term, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	records, err := c.fetchProteinRecords(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%w for %q: protein search returned %d ids but no protein records", errNoUsableProteinRows, term, len(ids))
	}
	rows := make([]model.KeywordResultRow, 0, len(records))
	for _, record := range records {
		row := keywordRowFromProteinRecord(term, record)
		if strings.TrimSpace(row.SequenceID) == "" || strings.TrimSpace(record.Sequence) == "" {
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w for %q: protein records had no FASTA sequence payloads", errNoUsableProteinRows, term)
	}
	return rows, nil
}

func (c *Client) searchProteinRowsFromNucleotideDB(ctx context.Context, term string, limit int) ([]model.KeywordResultRow, error) {
	ids, err := c.searchNucleotideIDs(ctx, term, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	records, err := c.fetchNucleotideCDSRecords(ctx, ids)
	if err != nil {
		return nil, err
	}
	records, err = c.enrichNucleotideCDSRecordsWithProteinRecords(ctx, records)
	if err != nil {
		return nil, err
	}
	rows := make([]model.KeywordResultRow, 0, len(records))
	for _, record := range records {
		row := keywordRowFromNucleotideCDSRecord(term, record)
		if strings.TrimSpace(row.SequenceID) == "" || strings.TrimSpace(record.ProteinSequence) == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (c *Client) searchProteinRowsFromGeneDB(ctx context.Context, term string, limit int) ([]model.KeywordResultRow, error) {
	sourceSpec := SearchTypeByID("gene")
	ids, _, _, _, err := c.searchGenericIDs(ctx, sourceSpec, term, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	targetIDs, linkName, err := c.fetchLinkedIDs(ctx, sourceSpec.EntrezDB, "protein", ids, []string{"gene_protein_refseq"})
	if err != nil {
		return nil, err
	}
	if len(targetIDs) == 0 {
		return nil, nil
	}
	records, err := c.fetchProteinRecords(ctx, targetIDs)
	if err != nil {
		return nil, err
	}
	rows := make([]model.KeywordResultRow, 0, len(records))
	for _, record := range records {
		row := keywordRowFromProteinRecord(term, record)
		if strings.TrimSpace(row.SequenceID) == "" || strings.TrimSpace(record.Sequence) == "" {
			continue
		}
		rows = append(rows, row)
	}
	return annotateLinkedKeywordRows(rows, SearchTypeByID("protein"), sourceSpec, linkName, ids, targetIDs, term), nil
}

func (c *Client) enrichNucleotideCDSRecordsWithProteinRecords(ctx context.Context, records []nucleotideCDSRecord) ([]nucleotideCDSRecord, error) {
	proteinIDs := make([]string, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.ProteinSequence) != "" {
			continue
		}
		if proteinID := strings.TrimSpace(record.ProteinID); proteinID != "" {
			proteinIDs = append(proteinIDs, proteinID)
		}
	}
	proteinIDs = uniqueNonEmpty(proteinIDs)
	if len(proteinIDs) == 0 {
		return records, nil
	}
	proteins, err := c.fetchProteinRecords(ctx, proteinIDs)
	if err != nil {
		return records, err
	}
	proteinByID := make(map[string]proteinRecord, len(proteins)*2)
	for _, protein := range proteins {
		for _, key := range []string{protein.Accession, protein.UID} {
			if key = normalizeAccessionKey(key); key != "" {
				proteinByID[key] = protein
			}
		}
	}
	for i := range records {
		if strings.TrimSpace(records[i].ProteinSequence) != "" {
			continue
		}
		protein, ok := proteinByID[normalizeAccessionKey(records[i].ProteinID)]
		if !ok || strings.TrimSpace(protein.Sequence) == "" {
			continue
		}
		records[i].ProteinSequence = strings.TrimSpace(protein.Sequence)
		records[i].Product = firstNonEmpty(records[i].Product, protein.Product, protein.Definition, protein.Title)
		records[i].GeneID = firstNonEmpty(records[i].GeneID, protein.GeneID)
		records[i].GeneName = firstNonEmpty(records[i].GeneName, protein.GeneName)
		records[i].LocusTag = firstNonEmpty(records[i].LocusTag, protein.LocusTag)
		if strings.TrimSpace(records[i].ProteinID) == "" {
			records[i].ProteinID = protein.Accession
		}
	}
	return records, nil
}

func (c *Client) FetchProteinSequence(ctx context.Context, targetID int, sequenceID string) (model.ProteinSequenceData, error) {
	_ = targetID
	sequenceID = strings.TrimSpace(sequenceID)
	if sequenceID == "" {
		return model.ProteinSequenceData{}, fmt.Errorf("empty NCBI protein accession")
	}
	c.mu.RLock()
	if cached, ok := c.sequenceCache[strings.ToUpper(sequenceID)]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()
	if cached, ok := readCachedJSON[model.ProteinSequenceData]("protein-sequences", sequenceID); ok && strings.TrimSpace(cached.Sequence) != "" {
		c.mu.Lock()
		c.sequenceCache[strings.ToUpper(sequenceID)] = cached
		c.mu.Unlock()
		return cached, nil
	}
	value, err, _ := c.sf.Do("protein-sequence:"+strings.ToUpper(sequenceID), func() (any, error) {
		records, err := c.fetchProteinRecords(ctx, []string{sequenceID})
		if err != nil {
			return model.ProteinSequenceData{}, err
		}
		for _, record := range records {
			if strings.EqualFold(record.Accession, sequenceID) || strings.EqualFold(record.UID, sequenceID) {
				data := model.ProteinSequenceData{
					Sequence:       strings.TrimSpace(record.Sequence),
					OriginalHeader: strings.TrimSpace(record.FastaHeader),
				}
				if data.Sequence == "" {
					return model.ProteinSequenceData{}, fmt.Errorf("NCBI protein %s returned no FASTA sequence", sequenceID)
				}
				c.mu.Lock()
				c.sequenceCache[strings.ToUpper(sequenceID)] = data
				c.mu.Unlock()
				writeCachedJSON("protein-sequences", sequenceID, data)
				return data, nil
			}
		}
		return model.ProteinSequenceData{}, fmt.Errorf("NCBI protein %s was not found", sequenceID)
	})
	if err != nil {
		return model.ProteinSequenceData{}, err
	}
	return value.(model.ProteinSequenceData), nil
}

func (c *Client) FetchGeneQuerySequence(ctx context.Context, species model.SpeciesCandidate, reportType string, identifier string) (*model.QuerySequenceSource, error) {
	_ = species
	if strings.TrimSpace(reportType) != "" && !strings.EqualFold(strings.TrimSpace(reportType), "protein") {
		return nil, fmt.Errorf("NCBI only supports protein query sequence resolution")
	}
	data, err := c.FetchProteinSequence(ctx, 0, identifier)
	if err != nil {
		return nil, err
	}
	return &model.QuerySequenceSource{
		Sequence:            data.Sequence,
		ProteinSequence:     data.Sequence,
		SequenceKind:        model.SequenceProtein,
		PreferredSequenceID: strings.TrimSpace(identifier),
		SourceDatabase:      c.Name(),
		SourceJBrowseName:   SyntheticSpeciesCandidate("protein").JBrowseName,
		SourceGenomeLabel:   SyntheticSpeciesCandidate("protein").GenomeLabel,
		ProteinID:           strings.TrimSpace(identifier),
		Annotation:          strings.TrimPrefix(strings.TrimSpace(data.OriginalHeader), ">"),
	}, nil
}

type eSearchGenericResponse struct {
	SearchResult struct {
		Count        string   `json:"count"`
		IDs          []string `json:"idlist"`
		QueryKey     string   `json:"querykey"`
		WebEnv       string   `json:"webenv"`
		Translation  []string `json:"translationstack"`
		TranslationS string   `json:"querytranslation"`
	} `json:"esearchresult"`
}

type eSummaryGenericResponse struct {
	Result map[string]json.RawMessage `json:"result"`
}

type eLinkResponse struct {
	LinkSets []struct {
		DBFrom     string `json:"dbfrom"`
		IDs        []any  `json:"ids"`
		LinkSetDBs []struct {
			DBTo     string `json:"dbto"`
			LinkName string `json:"linkname"`
			Links    []any  `json:"links"`
		} `json:"linksetdbs"`
	} `json:"linksets"`
}

type ncbiLinkTarget struct {
	DBTo         string
	LinkName     string
	DisplayLabel string
}

type linkedKeywordFallback struct {
	SourceSpec SearchType
	Targets    []ncbiLinkTarget
}

var ncbiLinkTargetsBySearchType = map[string][]ncbiLinkTarget{
	"gene": {
		{DBTo: "protein", LinkName: "gene_protein_refseq", DisplayLabel: "protein"},
		{DBTo: "nuccore", LinkName: "gene_nuccore_refseqrna", DisplayLabel: "nuccore"},
		{DBTo: "nuccore", LinkName: "gene_nuccore_refseqgenomic", DisplayLabel: "nuccore"},
		{DBTo: "clinvar", LinkName: "gene_clinvar", DisplayLabel: "clinvar"},
		{DBTo: "pubmed", LinkName: "gene_pubmed", DisplayLabel: "pubmed"},
	},
	"clinvar": {
		{DBTo: "gene", LinkName: "clinvar_gene", DisplayLabel: "gene"},
		{DBTo: "medgen", LinkName: "clinvar_medgen", DisplayLabel: "medgen"},
		{DBTo: "gtr", LinkName: "clinvar_gtr", DisplayLabel: "gtr"},
		{DBTo: "dbvar", LinkName: "clinvar_dbvar", DisplayLabel: "dbvar"},
		{DBTo: "omim", LinkName: "clinvar_omim", DisplayLabel: "omim"},
		{DBTo: "pubmed", LinkName: "clinvar_pubmed", DisplayLabel: "pubmed"},
		{DBTo: "pmc", LinkName: "clinvar_pmc", DisplayLabel: "pmc"},
		{DBTo: "snp", LinkName: "clinvar_snp", DisplayLabel: "snp"},
	},
	"snp": {
		{DBTo: "clinvar", LinkName: "snp_clinvar", DisplayLabel: "clinvar"},
		{DBTo: "dbvar", LinkName: "snp_dbvar", DisplayLabel: "dbvar"},
		{DBTo: "gene", LinkName: "snp_gene", DisplayLabel: "gene"},
		{DBTo: "pubmed", LinkName: "snp_pubmed", DisplayLabel: "pubmed"},
		{DBTo: "pmc", LinkName: "snp_pmc", DisplayLabel: "pmc"},
	},
	"dbvar": {
		{DBTo: "clinvar", LinkName: "dbvar_clinvar", DisplayLabel: "clinvar"},
		{DBTo: "bioproject", LinkName: "dbvar_bioproject", DisplayLabel: "bioproject"},
		{DBTo: "biosample", LinkName: "dbvar_biosample", DisplayLabel: "biosample"},
		{DBTo: "gene", LinkName: "dbvar_gene", DisplayLabel: "gene"},
		{DBTo: "omim", LinkName: "dbvar_omim", DisplayLabel: "omim"},
		{DBTo: "pubmed", LinkName: "dbvar_pubmed", DisplayLabel: "pubmed"},
		{DBTo: "snp", LinkName: "dbvar_snp", DisplayLabel: "snp"},
	},
	"medgen": {
		{DBTo: "gene", LinkName: "medgen_gene_diseases", DisplayLabel: "gene"},
		{DBTo: "clinvar", LinkName: "medgen_clinvar", DisplayLabel: "clinvar"},
		{DBTo: "gtr", LinkName: "medgen_gtr", DisplayLabel: "gtr"},
		{DBTo: "omim", LinkName: "medgen_omim", DisplayLabel: "omim"},
		{DBTo: "pubmed", LinkName: "medgen_pubmed", DisplayLabel: "pubmed"},
		{DBTo: "pmc", LinkName: "medgen_pmc", DisplayLabel: "pmc"},
		{DBTo: "books", LinkName: "medgen_books", DisplayLabel: "books"},
		{DBTo: "mesh", LinkName: "medgen_mesh", DisplayLabel: "mesh"},
	},
	"gtr": {
		{DBTo: "gene", LinkName: "gtr_gene", DisplayLabel: "gene"},
		{DBTo: "medgen", LinkName: "gtr_medgen", DisplayLabel: "medgen"},
		{DBTo: "omim", LinkName: "gtr_omim", DisplayLabel: "omim"},
	},
	"omim": {
		{DBTo: "gene", LinkName: "omim_gene", DisplayLabel: "gene"},
		{DBTo: "clinvar", LinkName: "omim_clinvar", DisplayLabel: "clinvar"},
		{DBTo: "dbvar", LinkName: "omim_dbvar", DisplayLabel: "dbvar"},
		{DBTo: "medgen", LinkName: "omim_medgen", DisplayLabel: "medgen"},
		{DBTo: "gtr", LinkName: "omim_gtr", DisplayLabel: "gtr"},
		{DBTo: "pubmed", LinkName: "omim_pubmed_cited", DisplayLabel: "pubmed"},
		{DBTo: "pmc", LinkName: "omim_pmc", DisplayLabel: "pmc"},
		{DBTo: "books", LinkName: "omim_books", DisplayLabel: "books"},
	},
	"assembly": {
		{DBTo: "bioproject", LinkName: "assembly_bioproject", DisplayLabel: "bioproject"},
		{DBTo: "biosample", LinkName: "assembly_biosample", DisplayLabel: "biosample"},
		{DBTo: "genome", LinkName: "assembly_genome", DisplayLabel: "genome"},
		{DBTo: "nuccore", LinkName: "assembly_nuccore_refseq", DisplayLabel: "nuccore"},
	},
	"bioproject": {
		{DBTo: "assembly", LinkName: "bioproject_assembly_all", DisplayLabel: "assembly"},
		{DBTo: "biosample", LinkName: "bioproject_biosample_all", DisplayLabel: "biosample"},
		{DBTo: "sra", LinkName: "bioproject_sra_all", DisplayLabel: "sra"},
	},
	"biosample": {
		{DBTo: "bioproject", LinkName: "biosample_bioproject", DisplayLabel: "bioproject"},
		{DBTo: "assembly", LinkName: "biosample_assembly", DisplayLabel: "assembly"},
		{DBTo: "sra", LinkName: "biosample_sra", DisplayLabel: "sra"},
	},
	"taxonomy": {
		{DBTo: "assembly", LinkName: "taxonomy_assembly", DisplayLabel: "assembly"},
		{DBTo: "bioproject", LinkName: "taxonomy_bioproject", DisplayLabel: "bioproject"},
		{DBTo: "biosample", LinkName: "taxonomy_biosample", DisplayLabel: "biosample"},
		{DBTo: "gene", LinkName: "taxonomy_gene", DisplayLabel: "gene"},
		{DBTo: "nuccore", LinkName: "taxonomy_nuccore", DisplayLabel: "nuccore"},
		{DBTo: "protein", LinkName: "taxonomy_protein", DisplayLabel: "protein"},
		{DBTo: "sra", LinkName: "taxonomy_sra", DisplayLabel: "sra"},
	},
	"sra": {
		{DBTo: "bioproject", LinkName: "sra_bioproject", DisplayLabel: "bioproject"},
		{DBTo: "biosample", LinkName: "sra_biosample", DisplayLabel: "biosample"},
		{DBTo: "taxonomy", LinkName: "sra_taxonomy_analysis", DisplayLabel: "taxonomy"},
	},
	"pubmed": {
		{DBTo: "pmc", LinkName: "pubmed_pmc", DisplayLabel: "pmc"},
		{DBTo: "gene", LinkName: "pubmed_gene", DisplayLabel: "gene"},
		{DBTo: "protein", LinkName: "pubmed_protein", DisplayLabel: "protein"},
		{DBTo: "clinvar", LinkName: "pubmed_clinvar", DisplayLabel: "clinvar"},
		{DBTo: "dbvar", LinkName: "pubmed_dbvar", DisplayLabel: "dbvar"},
		{DBTo: "medgen", LinkName: "pubmed_medgen", DisplayLabel: "medgen"},
		{DBTo: "omim", LinkName: "pubmed_omim_cited", DisplayLabel: "omim"},
		{DBTo: "snp", LinkName: "pubmed_snp", DisplayLabel: "snp"},
		{DBTo: "sra", LinkName: "pubmed_sra", DisplayLabel: "sra"},
	},
	"pmc": {
		{DBTo: "pubmed", LinkName: "pmc_pubmed", DisplayLabel: "pubmed"},
		{DBTo: "gene", LinkName: "pmc_gene", DisplayLabel: "gene"},
		{DBTo: "clinvar", LinkName: "pmc_clinvar", DisplayLabel: "clinvar"},
		{DBTo: "medgen", LinkName: "pmc_medgen", DisplayLabel: "medgen"},
		{DBTo: "omim", LinkName: "pmc_omim", DisplayLabel: "omim"},
		{DBTo: "snp", LinkName: "pmc_snp", DisplayLabel: "snp"},
		{DBTo: "sra", LinkName: "pmc_sra", DisplayLabel: "sra"},
		{DBTo: "bioproject", LinkName: "pmc_bioproject", DisplayLabel: "bioproject"},
	},
	"books": {
		{DBTo: "gene", LinkName: "books_gene", DisplayLabel: "gene"},
		{DBTo: "medgen", LinkName: "books_medgen", DisplayLabel: "medgen"},
		{DBTo: "omim", LinkName: "books_omim", DisplayLabel: "omim"},
		{DBTo: "pmc", LinkName: "books_pmc_refs", DisplayLabel: "pmc"},
		{DBTo: "pubmed", LinkName: "books_pubmed_refs", DisplayLabel: "pubmed"},
	},
	"mesh": {
		{DBTo: "medgen", LinkName: "mesh_medgen", DisplayLabel: "medgen"},
		{DBTo: "pccompound", LinkName: "mesh_pccompound", DisplayLabel: "pccompound"},
	},
	"gds": {
		{DBTo: "bioproject", LinkName: "gds_bioproject", DisplayLabel: "bioproject"},
		{DBTo: "biosample", LinkName: "gds_biosample", DisplayLabel: "biosample"},
		{DBTo: "dbvar", LinkName: "gds_dbvar", DisplayLabel: "dbvar"},
		{DBTo: "geoprofiles", LinkName: "gds_geoprofiles", DisplayLabel: "geoprofiles"},
		{DBTo: "pmc", LinkName: "gds_pmc", DisplayLabel: "pmc"},
		{DBTo: "pubmed", LinkName: "gds_pubmed", DisplayLabel: "pubmed"},
		{DBTo: "sra", LinkName: "gds_sra", DisplayLabel: "sra"},
	},
	"geoprofiles": {
		{DBTo: "gds", LinkName: "geoprofiles_gds", DisplayLabel: "gds"},
		{DBTo: "gene", LinkName: "geoprofiles_gene", DisplayLabel: "gene"},
		{DBTo: "omim", LinkName: "geoprofiles_omim", DisplayLabel: "omim"},
		{DBTo: "pmc", LinkName: "geoprofiles_pmc", DisplayLabel: "pmc"},
		{DBTo: "pubmed", LinkName: "geoprofiles_pubmed", DisplayLabel: "pubmed"},
	},
}

func (c *Client) searchGenericKeywordRows(ctx context.Context, species model.SpeciesCandidate, keyword string) ([]model.KeywordResultRow, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, nil
	}
	spec := SearchTypeFromSpeciesCandidate(species)
	rows, err := c.searchDirectGenericKeywordRows(ctx, spec, keyword)
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	for _, fallback := range c.linkedFallbacksForSearchType(spec.ID) {
		rows, err = c.searchLinkedKeywordRows(ctx, spec, keyword, fallback)
		if err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			return rows, nil
		}
	}
	return nil, nil
}

func (c *Client) searchDirectGenericKeywordRows(ctx context.Context, spec SearchType, keyword string) ([]model.KeywordResultRow, error) {
	ids, webEnv, queryKey, total, err := c.searchGenericIDs(ctx, spec, keyword, defaultRetMax)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return c.fetchGenericSummaryRows(ctx, spec, keyword, ids, webEnv, queryKey, total)
}

func (c *Client) searchGenericIDs(ctx context.Context, spec SearchType, term string, limit int) ([]string, string, string, int, error) {
	if limit <= 0 {
		limit = defaultRetMax
	}
	values := url.Values{}
	values.Set("db", spec.EntrezDB)
	values.Set("term", term)
	values.Set("retmode", "json")
	values.Set("retmax", strconv.Itoa(limit))
	values.Set("usehistory", "y")
	var payload eSearchGenericResponse
	if err := c.getJSON(ctx, "esearch.fcgi", values, &payload); err != nil {
		return nil, "", "", 0, err
	}
	total, _ := strconv.Atoi(strings.TrimSpace(payload.SearchResult.Count))
	return uniqueNonEmpty(payload.SearchResult.IDs), strings.TrimSpace(payload.SearchResult.WebEnv), strings.TrimSpace(payload.SearchResult.QueryKey), total, nil
}

func (c *Client) fetchGenericSummaryRows(ctx context.Context, spec SearchType, keyword string, ids []string, webEnv string, queryKey string, total int) ([]model.KeywordResultRow, error) {
	values := url.Values{}
	values.Set("db", spec.EntrezDB)
	values.Set("retmode", "json")
	if strings.TrimSpace(webEnv) != "" && strings.TrimSpace(queryKey) != "" {
		values.Set("query_key", queryKey)
		values.Set("WebEnv", webEnv)
		values.Set("retmax", strconv.Itoa(len(ids)))
	} else {
		values.Set("id", strings.Join(ids, ","))
	}
	var payload eSummaryGenericResponse
	if err := c.getJSON(ctx, "esummary.fcgi", values, &payload); err != nil {
		return nil, err
	}
	rows := make([]model.KeywordResultRow, 0, len(ids))
	for i, id := range ids {
		raw := payload.Result[id]
		if len(raw) == 0 {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			continue
		}
		row := keywordRowFromGenericSummary(spec, keyword, id, i+1, total, doc)
		rows = append(rows, row)
	}
	rows = c.enrichGenericRowsWithEFetch(ctx, spec, rows)
	return rows, nil
}

func (c *Client) enrichGenericRowsWithEFetch(ctx context.Context, spec SearchType, rows []model.KeywordResultRow) []model.KeywordResultRow {
	switch spec.ID {
	case "bioproject":
		return c.enrichBioProjectRowsWithEFetch(ctx, rows)
	case "clinvar":
		return c.enrichClinVarRowsWithEFetch(ctx, rows)
	case "gtr":
		return c.enrichGTRRowsWithEFetch(ctx, rows)
	default:
		return rows
	}
}

func keywordRowFromGenericSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	switch spec.ID {
	case "gene":
		return geneKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "nuccore", "nucleotide":
		return nuccoreKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "assembly":
		return assemblyKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "bioproject":
		return bioprojectKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "biosample":
		return biosampleKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "taxonomy":
		return taxonomyKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "sra":
		return sraKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "clinvar":
		return clinvarKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "snp":
		return snpKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "dbvar":
		return dbvarKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "medgen":
		return medgenKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "gtr":
		return gtrKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "omim":
		return omimKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "pubmed":
		return pubmedKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	case "pmc":
		return pmcKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	default:
		return genericKeywordRowFromSummary(spec, keyword, id, index, total, doc)
	}
}

func genericKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	title := firstNonEmpty(
		stringFromSummary(doc, "title"),
		stringFromSummary(doc, "name"),
		stringFromSummary(doc, "description"),
		stringFromSummary(doc, "caption"),
	)
	accession := firstNonEmpty(
		stringFromSummary(doc, "accessionversion"),
		stringFromSummary(doc, "caption"),
		stringFromSummary(doc, "assemblyaccession"),
		stringFromSummary(doc, "accession"),
		strings.TrimSpace(id),
	)
	organism := firstNonEmpty(
		stringFromSummary(doc, "organism"),
		stringFromNestedSummary(doc, "organism", "scientificname"),
		stringFromNestedSummary(doc, "biosource", "infraspecies"),
		stringFromSummary(doc, "taxname"),
	)
	geneID := firstNonEmpty(
		stringFromSummary(doc, "uid"),
		stringFromSummary(doc, "geneid"),
		strings.TrimSpace(id),
	)
	geneSymbol := firstNonEmpty(
		stringFromSummary(doc, "name"),
		stringFromSummary(doc, "nomenclaturesymbol"),
		stringFromSummary(doc, "caption"),
	)
	description := firstNonEmpty(
		title,
		stringFromSummary(doc, "summary"),
		stringFromSummary(doc, "extra"),
	)
	geneLocus := firstNonEmpty(
		stringFromSummary(doc, "locus"),
		stringFromSummary(doc, "locus_tag"),
		stringFromSummary(doc, "maplocation"),
	)
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, title, organism, doc)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		GeneLocus:           geneLocus,
		GeneIdentifier:      geneID,
		Genome:              organism,
		Symbols:             geneSymbol,
		Description:         description,
		Comments:            firstNonEmpty(stringFromSummary(doc, "summary"), stringFromSummary(doc, "extra")),
		AutoDefine:          description,
		SequenceHeaderLabel: organism,
		ExtraColumns:        extra,
	}
}

func geneKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	symbol := firstNonEmpty(
		stringFromSummary(doc, "nomenclaturesymbol"),
		stringFromSummary(doc, "name"),
		stringFromSummary(doc, "caption"),
	)
	description := firstNonEmpty(
		stringFromSummary(doc, "description"),
		stringFromSummary(doc, "summary"),
		stringFromSummary(doc, "title"),
	)
	organism := firstNonEmpty(
		stringFromNestedSummary(doc, "organism", "scientificname"),
		stringFromSummary(doc, "organism"),
		stringFromSummary(doc, "taxname"),
	)
	geneID := firstNonEmpty(stringFromSummary(doc, "uid"), stringFromSummary(doc, "geneid"), strings.TrimSpace(id))
	geneLocus := firstNonEmpty(
		stringFromSummary(doc, "nomenclaturelocus"),
		stringFromSummary(doc, "locus"),
		stringFromSummary(doc, "locus_tag"),
		stringFromSummary(doc, "maplocation"),
		symbol,
	)
	synonyms := firstNonEmpty(stringFromSummary(doc, "otheraliases"), stringFromSummary(doc, "otherdesignations"))
	extra := buildGenericSummaryExtra(spec, id, index, total, geneID, description, organism, doc)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		GeneLocus:           geneLocus,
		GeneIdentifier:      geneID,
		Genome:              organism,
		Aliases:             stringFromSummary(doc, "otheraliases"),
		Symbols:             symbol,
		Synonyms:            synonyms,
		Description:         description,
		Comments:            firstNonEmpty(stringFromSummary(doc, "summary"), stringFromSummary(doc, "genomicinfo")),
		AutoDefine:          firstNonEmpty(stringFromSummary(doc, "otherdesignations"), description),
		SequenceHeaderLabel: organism,
		ExtraColumns:        extra,
	}
}

func nuccoreKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	accession := firstNonEmpty(
		stringFromSummary(doc, "accessionversion"),
		stringFromSummary(doc, "caption"),
		stringFromSummary(doc, "accession"),
		strings.TrimSpace(id),
	)
	title := firstNonEmpty(
		stringFromSummary(doc, "title"),
		stringFromSummary(doc, "subname"),
		stringFromSummary(doc, "name"),
	)
	organism := firstNonEmpty(
		stringFromSummary(doc, "organism"),
		stringFromSummary(doc, "taxname"),
		stringFromNestedSummary(doc, "biosource", "scientificname"),
	)
	molType := firstNonEmpty(stringFromSummary(doc, "moltype"), stringFromSummary(doc, "biomol"))
	subtype := firstNonEmpty(stringFromSummary(doc, "subtype"), stringFromSummary(doc, "subname"))
	description := firstNonEmpty(title, molType, accession)
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, title, organism, doc)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		TranscriptID:        accession,
		GeneIdentifier:      accession,
		Genome:              organism,
		Location:            joinNonEmpty("; ", molType, subtype, summaryLengthText(doc)),
		Description:         description,
		Comments:            firstNonEmpty(stringFromSummary(doc, "extra"), stringFromSummary(doc, "completeness")),
		AutoDefine:          firstNonEmpty(title, accession),
		SequenceHeaderLabel: organism,
		ExtraColumns:        extra,
	}
}

func assemblyKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	accession := firstNonEmpty(stringFromSummary(doc, "assemblyaccession"), stringFromSummary(doc, "rsuid"), strings.TrimSpace(id))
	name := firstNonEmpty(stringFromSummary(doc, "assemblyname"), stringFromSummary(doc, "assemblystatus"), stringFromSummary(doc, "title"))
	organism := firstNonEmpty(stringFromSummary(doc, "organism"), stringFromSummary(doc, "speciesname"), stringFromSummary(doc, "taxname"))
	status := firstNonEmpty(stringFromSummary(doc, "assemblystatus"), stringFromSummary(doc, "status"))
	ftpPath := firstNonEmpty(stringFromSummary(doc, "ftppath_refseq"), stringFromSummary(doc, "ftppath_genbank"))
	description := firstNonEmpty(name, accession)
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, name, organism, doc)
	setExtraColumns(extra,
		"ncbi_assembly_accession", accession,
		"ncbi_assembly_name", name,
		"ncbi_assembly_status", status,
		"ncbi_assembly_level", firstNonEmpty(stringFromSummary(doc, "assemblylevel"), status),
		"ncbi_refseq_category", stringFromSummary(doc, "refseq_category"),
		"ncbi_submission_date", stringFromSummary(doc, "submissiondate"),
		"ncbi_submitter", stringFromSummary(doc, "submitter"),
		"ncbi_taxonomy_id", firstNonEmpty(stringFromSummary(doc, "taxid"), stringFromNestedSummary(doc, "organism", "taxid")),
		"ncbi_bioproject_accession", firstNonEmpty(stringFromSummary(doc, "bioprojectaccn"), stringFromSummary(doc, "project_acc"), stringFromSummary(doc, "projectaccession")),
		"ncbi_biosample_accession", firstNonEmpty(stringFromSummary(doc, "biosampleaccn"), stringFromSummary(doc, "sampleaccn")),
		"ncbi_ftp_path", ftpPath,
	)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		LabelName:           name,
		LabelNameType:       "ncbi assembly name",
		GeneIdentifier:      accession,
		Genome:              organism,
		Description:         description,
		Comments:            joinNonEmpty("; ", status, ftpPath),
		AutoDefine:          joinNonEmpty("; ", name, accession, status, stringFromSummary(doc, "submissiondate")),
		SequenceHeaderLabel: organism,
		ExtraColumns:        extra,
	}
}

func bioprojectKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	accession := firstNonEmpty(stringFromSummary(doc, "project_acc"), stringFromSummary(doc, "projectaccession"), stringFromSummary(doc, "caption"), strings.TrimSpace(id))
	title := firstNonEmpty(stringFromSummary(doc, "title"), stringFromSummary(doc, "name"), accession)
	organism := firstNonEmpty(stringFromSummary(doc, "organism_name"), stringFromSummary(doc, "taxname"), stringFromSummary(doc, "organism"))
	projectType := stringFromSummary(doc, "project_type")
	dataType := stringFromSummary(doc, "project_data_type")
	description := firstNonEmpty(title, accession)
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, title, organism, doc)
	setExtraColumns(extra,
		"ncbi_bioproject_accession", accession,
		"ncbi_project_title", title,
		"ncbi_project_type", projectType,
		"ncbi_project_data_type", dataType,
		"ncbi_project_status", stringFromSummary(doc, "status"),
		"ncbi_project_relevance", stringFromSummary(doc, "relevance"),
		"ncbi_taxonomy_id", firstNonEmpty(stringFromSummary(doc, "taxid"), stringFromNestedSummary(doc, "organism", "taxid")),
		"ncbi_submission_date", firstNonEmpty(stringFromSummary(doc, "submissiondate"), stringFromSummary(doc, "createdate")),
		"ncbi_project_description", firstNonEmpty(stringFromSummary(doc, "description"), title),
	)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		LabelName:           title,
		LabelNameType:       "ncbi project title",
		GeneIdentifier:      accession,
		Genome:              organism,
		Description:         description,
		Comments:            joinNonEmpty("; ", projectType, stringFromSummary(doc, "relevance"), dataType),
		AutoDefine:          joinNonEmpty("; ", title, accession, projectType),
		SequenceHeaderLabel: organism,
		ExtraColumns:        extra,
	}
}

func biosampleKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	accession := firstNonEmpty(stringFromSummary(doc, "accession"), stringFromSummary(doc, "sampleaccn"), stringFromSummary(doc, "caption"), strings.TrimSpace(id))
	title := firstNonEmpty(stringFromSummary(doc, "title"), stringFromSummary(doc, "sample_name"), accession)
	organism := firstNonEmpty(stringFromSummary(doc, "organism"), stringFromSummary(doc, "taxonomy"), stringFromSummary(doc, "taxname"))
	attributeBag := parseBioSampleAttributeBag(firstNonEmpty(stringFromSummary(doc, "sampledata"), stringFromSummary(doc, "sample_data")))
	isolationSource := firstNonEmpty(stringFromSummary(doc, "isolation_source"), attributeBag["isolation_source"], attributeBag["isolation source"])
	host := firstNonEmpty(stringFromSummary(doc, "host"), attributeBag["host"])
	geoLoc := firstNonEmpty(stringFromSummary(doc, "geo_loc_name"), attributeBag["geo_loc_name"], attributeBag["geo loc name"])
	collectionDate := firstNonEmpty(stringFromSummary(doc, "collection_date"), attributeBag["collection_date"], attributeBag["collection date"])
	strain := firstNonEmpty(stringFromSummary(doc, "strain"), attributeBag["strain"])
	cultivar := firstNonEmpty(stringFromSummary(doc, "cultivar"), attributeBag["cultivar"])
	tissue := firstNonEmpty(stringFromSummary(doc, "tissue"), attributeBag["tissue"])
	developmentalStage := firstNonEmpty(stringFromSummary(doc, "developmental_stage"), attributeBag["developmental_stage"], attributeBag["developmental stage"])
	description := firstNonEmpty(title, accession)
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, title, organism, doc)
	setExtraColumns(extra,
		"ncbi_biosample_accession", accession,
		"ncbi_sample_name", firstNonEmpty(stringFromSummary(doc, "sample_name"), title),
		"ncbi_biosample_model", stringFromSummary(doc, "biosamplemodel"),
		"ncbi_sample_owner", stringFromSummary(doc, "owner"),
		"ncbi_sample_attributes", stringFromSummary(doc, "sampledata"),
		"ncbi_isolation_source", isolationSource,
		"ncbi_host", host,
		"ncbi_geo_loc_name", geoLoc,
		"ncbi_collection_date", collectionDate,
		"ncbi_strain", strain,
		"ncbi_cultivar", cultivar,
		"ncbi_tissue", tissue,
		"ncbi_developmental_stage", developmentalStage,
		"ncbi_taxonomy_id", firstNonEmpty(stringFromSummary(doc, "taxid"), stringFromNestedSummary(doc, "organism", "taxid")),
		"ncbi_bioproject_accession", firstNonEmpty(stringFromSummary(doc, "bioprojectaccn"), stringFromSummary(doc, "project_acc"), stringFromSummary(doc, "projectaccession")),
	)
	attachNormalizedAttributeExtras(extra, "ncbi_biosample_attr_", attributeBag)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		LabelName:           title,
		LabelNameType:       "ncbi sample title",
		GeneIdentifier:      accession,
		Genome:              organism,
		Description:         description,
		Comments:            joinNonEmpty("; ", isolationSource, host, geoLoc, stringFromSummary(doc, "biosamplemodel"), stringFromSummary(doc, "owner")),
		AutoDefine:          joinNonEmpty("; ", title, accession, stringFromSummary(doc, "biosamplemodel")),
		SequenceHeaderLabel: organism,
		ExtraColumns:        extra,
	}
}

func taxonomyKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	taxID := firstNonEmpty(stringFromSummary(doc, "taxid"), stringFromSummary(doc, "uid"), strings.TrimSpace(id))
	scientificName := firstNonEmpty(stringFromSummary(doc, "scientificname"), stringFromSummary(doc, "title"), stringFromSummary(doc, "name"))
	commonName := firstNonEmpty(stringFromSummary(doc, "commonname"), stringFromSummary(doc, "genbankcommonname"))
	rank := firstNonEmpty(stringFromSummary(doc, "rank"), stringFromSummary(doc, "division"))
	description := firstNonEmpty(scientificName, commonName, taxID)
	extra := buildGenericSummaryExtra(spec, id, index, total, taxID, scientificName, scientificName, doc)
	setExtraColumns(extra,
		"ncbi_taxonomy_id", taxID,
		"ncbi_scientific_name", scientificName,
		"ncbi_common_name", commonName,
		"ncbi_rank", rank,
		"ncbi_lineage_summary", stringFromSummary(doc, "lineage"),
		"ncbi_division", stringFromSummary(doc, "division"),
		"ncbi_parent_taxonomy_id", stringFromSummary(doc, "parenttaxid"),
	)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		LabelName:           scientificName,
		LabelNameType:       "ncbi scientific name",
		GeneIdentifier:      taxID,
		Genome:              scientificName,
		Description:         description,
		Comments:            joinNonEmpty("; ", rank, commonName, stringFromSummary(doc, "lineage")),
		AutoDefine:          joinNonEmpty("; ", scientificName, commonName, rank),
		SequenceHeaderLabel: scientificName,
		ExtraColumns:        extra,
	}
}

func sraKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	accession := firstNonEmpty(
		stringFromSummary(doc, "accession"),
		stringFromSummary(doc, "caption"),
		stringFromSummary(doc, "runs"),
		strings.TrimSpace(id),
	)
	title := firstNonEmpty(stringFromSummary(doc, "title"), stringFromSummary(doc, "desc"), accession)
	experimentMeta := parseSRAExperimentXML(stringFromSummary(doc, "expxml"))
	runMeta := parseSRARuns(stringFromSummary(doc, "runs"))
	organism := firstNonEmpty(stringFromSummary(doc, "organism"), stringFromSummary(doc, "taxname"), stringFromSummary(doc, "expxml"))
	libraryStrategy := firstNonEmpty(stringFromSummary(doc, "library_strategy"), experimentMeta.LibraryStrategy)
	librarySource := firstNonEmpty(stringFromSummary(doc, "library_source"), experimentMeta.LibrarySource)
	platform := firstNonEmpty(stringFromSummary(doc, "platform"), experimentMeta.Platform, runMeta.Platform)
	description := firstNonEmpty(title, accession)
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, title, organism, doc)
	setExtraColumns(extra,
		"ncbi_sra_accession", accession,
		"ncbi_sra_title", title,
		"ncbi_library_strategy", libraryStrategy,
		"ncbi_library_source", librarySource,
		"ncbi_platform", platform,
		"ncbi_bioproject_accession", firstNonEmpty(stringFromSummary(doc, "bioprojectaccn"), stringFromSummary(doc, "project_acc"), stringFromSummary(doc, "projectaccession"), experimentMeta.BioProjectAccession),
		"ncbi_biosample_accession", firstNonEmpty(stringFromSummary(doc, "biosampleaccn"), stringFromSummary(doc, "sampleaccn"), experimentMeta.BioSampleAccession, runMeta.BioSampleAccession),
		"ncbi_instrument_model", firstNonEmpty(stringFromSummary(doc, "instrument_model"), experimentMeta.InstrumentModel, runMeta.InstrumentModel),
		"ncbi_spots", firstNonEmpty(stringFromSummary(doc, "spots"), runMeta.Spots),
		"ncbi_bases", firstNonEmpty(stringFromSummary(doc, "bases"), runMeta.Bases),
		"ncbi_layout", firstNonEmpty(stringFromSummary(doc, "layout"), experimentMeta.Layout),
		"ncbi_study_accession", firstNonEmpty(stringFromSummary(doc, "study_accession"), experimentMeta.StudyAccession, runMeta.StudyAccession),
		"ncbi_experiment_accession", firstNonEmpty(stringFromSummary(doc, "experiment_accession"), experimentMeta.ExperimentAccession, runMeta.ExperimentAccession),
		"ncbi_run_accession", firstNonEmpty(stringFromSummary(doc, "run_accession"), runMeta.RunAccession),
		"ncbi_experiment_xml", stringFromSummary(doc, "expxml"),
		"ncbi_runs", stringFromSummary(doc, "runs"),
	)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		LabelName:           title,
		LabelNameType:       "ncbi sra title",
		GeneIdentifier:      accession,
		Genome:              organism,
		Description:         description,
		Comments:            joinNonEmpty("; ", libraryStrategy, librarySource, platform, firstNonEmpty(runMeta.RunAccession, stringFromSummary(doc, "runs"))),
		AutoDefine:          joinNonEmpty("; ", title, accession, libraryStrategy),
		SequenceHeaderLabel: organism,
		ExtraColumns:        extra,
	}
}

func clinvarKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	accession := firstNonEmpty(
		stringFromSummary(doc, "accession"),
		stringFromSummary(doc, "vacc"),
		stringFromSummary(doc, "rcvaccession"),
		stringFromSummary(doc, "title"),
		strings.TrimSpace(id),
	)
	title := firstNonEmpty(
		stringFromSummary(doc, "title"),
		stringFromSummary(doc, "variation_name"),
		stringFromSummary(doc, "commonname"),
		accession,
	)
	organism := firstNonEmpty(stringFromSummary(doc, "organism"), stringFromSummary(doc, "taxname"))
	description := firstNonEmpty(title, accession)
	geneID := firstNonEmpty(stringFromSummary(doc, "gene"), stringFromSummary(doc, "gid"), stringFromSummary(doc, "geneid"))
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, title, organism, doc)
	setExtraColumns(extra,
		"ncbi_clinvar_accession", accession,
		"ncbi_variation_name", title,
		"ncbi_gene_id", geneID,
		"ncbi_clinical_significance", stringFromSummary(doc, "clinicalsignificance"),
		"ncbi_review_status", stringFromSummary(doc, "reviewstatus"),
		"ncbi_condition", stringFromSummary(doc, "traitset"),
		"ncbi_variant_type", firstNonEmpty(stringFromSummary(doc, "variant_type"), stringFromSummary(doc, "variation_set_name")),
	)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		LabelName:           title,
		LabelNameType:       "ncbi clinvar title",
		GeneIdentifier:      firstNonEmpty(geneID, accession),
		Genome:              organism,
		Description:         description,
		Comments:            joinNonEmpty("; ", stringFromSummary(doc, "clinicalsignificance"), stringFromSummary(doc, "reviewstatus"), stringFromSummary(doc, "traitset")),
		AutoDefine:          joinNonEmpty("; ", title, accession, stringFromSummary(doc, "clinicalsignificance")),
		SequenceHeaderLabel: organism,
		ExtraColumns:        extra,
	}
}

func snpKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	rsid := firstNonEmpty(stringFromSummary(doc, "caption"), stringFromSummary(doc, "snp_id"), strings.TrimSpace(id))
	title := firstNonEmpty(stringFromSummary(doc, "title"), rsid)
	geneID := firstNonEmpty(stringFromSummary(doc, "gene"), stringFromSummary(doc, "geneid"), stringFromSummary(doc, "gene_id"), rsid)
	organism := firstNonEmpty(stringFromSummary(doc, "organism"), stringFromSummary(doc, "taxname"))
	extra := buildGenericSummaryExtra(spec, id, index, total, rsid, title, organism, doc)
	setExtraColumns(extra,
		"ncbi_rsid", rsid,
		"ncbi_snp_title", title,
		"ncbi_gene_id", geneID,
		"ncbi_variant_type", stringFromSummary(doc, "snp_class"),
		"ncbi_variant_class", stringFromSummary(doc, "snp_class"),
		"ncbi_clinical_significance", stringFromSummary(doc, "clinical_significance"),
		"ncbi_taxonomy_id", firstNonEmpty(stringFromSummary(doc, "taxid"), stringFromNestedSummary(doc, "organism", "taxid")),
		"ncbi_chromosome", stringFromSummary(doc, "chr"),
		"ncbi_chrpos", firstNonEmpty(stringFromSummary(doc, "chrpos"), stringFromSummary(doc, "position")),
	)
	return model.KeywordResultRow{
		SourceDatabase: "ncbi",
		SearchTerm:     keyword,
		SearchType:     "NCBI " + spec.Label,
		LabelName:      rsid,
		LabelNameType:  "ncbi rsid",
		GeneIdentifier: geneID,
		Genome:         organism,
		Description:    title,
		Comments:       joinNonEmpty("; ", stringFromSummary(doc, "snp_class"), stringFromSummary(doc, "clinical_significance")),
		AutoDefine:     joinNonEmpty("; ", rsid, title, stringFromSummary(doc, "snp_class")),
		ExtraColumns:   extra,
	}
}

func dbvarKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	accession := firstNonEmpty(stringFromSummary(doc, "accession"), stringFromSummary(doc, "caption"), strings.TrimSpace(id))
	title := firstNonEmpty(stringFromSummary(doc, "title"), accession)
	geneID := firstNonEmpty(stringFromSummary(doc, "gene"), stringFromSummary(doc, "geneid"), stringFromSummary(doc, "gene_id"), accession)
	organism := firstNonEmpty(stringFromSummary(doc, "organism"), stringFromSummary(doc, "taxname"))
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, title, organism, doc)
	setExtraColumns(extra,
		"ncbi_dbvar_accession", accession,
		"ncbi_variant_type", stringFromSummary(doc, "variant_type"),
		"ncbi_gene_id", geneID,
		"ncbi_phenotype", firstNonEmpty(stringFromSummary(doc, "phenotype"), stringFromSummary(doc, "traitset")),
		"ncbi_clinical_assertion", stringFromSummary(doc, "clinical_assertion"),
		"ncbi_bioproject_accession", firstNonEmpty(stringFromSummary(doc, "bioprojectaccn"), stringFromSummary(doc, "project_acc"), stringFromSummary(doc, "projectaccession")),
		"ncbi_biosample_accession", firstNonEmpty(stringFromSummary(doc, "biosampleaccn"), stringFromSummary(doc, "sampleaccn")),
	)
	return model.KeywordResultRow{
		SourceDatabase: "ncbi",
		SearchTerm:     keyword,
		SearchType:     "NCBI " + spec.Label,
		LabelName:      accession,
		LabelNameType:  "ncbi dbvar accession",
		GeneIdentifier: geneID,
		Genome:         organism,
		Description:    title,
		Comments:       joinNonEmpty("; ", stringFromSummary(doc, "variant_type"), stringFromSummary(doc, "phenotype"), stringFromSummary(doc, "clinical_assertion")),
		AutoDefine:     joinNonEmpty("; ", accession, title, stringFromSummary(doc, "variant_type")),
		ExtraColumns:   extra,
	}
}

func medgenKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	accession := firstNonEmpty(stringFromSummary(doc, "conceptid"), stringFromSummary(doc, "uid"), strings.TrimSpace(id))
	title := firstNonEmpty(stringFromSummary(doc, "title"), stringFromSummary(doc, "name"), accession)
	geneID := firstNonEmpty(stringFromSummary(doc, "gene"), stringFromSummary(doc, "geneid"), accession)
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, title, "", doc)
	setExtraColumns(extra,
		"ncbi_medgen_id", accession,
		"ncbi_preferred_title", title,
		"ncbi_condition_summary", firstNonEmpty(stringFromSummary(doc, "summary"), stringFromSummary(doc, "definition")),
		"ncbi_related_gene_summary", stringFromSummary(doc, "gene"),
		"ncbi_gene_id", geneID,
		"ncbi_definition", stringFromSummary(doc, "definition"),
		"ncbi_source", stringFromSummary(doc, "source"),
	)
	return model.KeywordResultRow{
		SourceDatabase: "ncbi",
		SearchTerm:     keyword,
		SearchType:     "NCBI " + spec.Label,
		LabelName:      title,
		LabelNameType:  "ncbi medgen preferred title",
		GeneIdentifier: geneID,
		Description:    title,
		Comments:       joinNonEmpty("; ", stringFromSummary(doc, "definition"), stringFromSummary(doc, "source")),
		AutoDefine:     joinNonEmpty("; ", title, accession),
		ExtraColumns:   extra,
	}
}

func gtrKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	accession := firstNonEmpty(stringFromSummary(doc, "accession"), stringFromSummary(doc, "caption"), strings.TrimSpace(id))
	title := firstNonEmpty(stringFromSummary(doc, "title"), accession)
	geneID := firstNonEmpty(stringFromSummary(doc, "gene"), stringFromSummary(doc, "geneid"), accession)
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, title, "", doc)
	setExtraColumns(extra,
		"ncbi_gtr_accession", accession,
		"ncbi_test_name", title,
		"ncbi_condition", firstNonEmpty(stringFromSummary(doc, "condition"), stringFromSummary(doc, "traitset")),
		"ncbi_gene_id", geneID,
		"ncbi_test_type", stringFromSummary(doc, "testtype"),
		"ncbi_method", stringFromSummary(doc, "method"),
		"ncbi_lab", stringFromSummary(doc, "labname"),
	)
	return model.KeywordResultRow{
		SourceDatabase: "ncbi",
		SearchTerm:     keyword,
		SearchType:     "NCBI " + spec.Label,
		LabelName:      title,
		LabelNameType:  "ncbi gtr test name",
		GeneIdentifier: geneID,
		Description:    title,
		Comments:       joinNonEmpty("; ", stringFromSummary(doc, "condition"), stringFromSummary(doc, "testtype"), stringFromSummary(doc, "method"), stringFromSummary(doc, "labname")),
		AutoDefine:     joinNonEmpty("; ", title, accession, stringFromSummary(doc, "labname")),
		ExtraColumns:   extra,
	}
}

func omimKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	accession := firstNonEmpty(stringFromSummary(doc, "omimid"), stringFromSummary(doc, "uid"), strings.TrimSpace(id))
	title := firstNonEmpty(stringFromSummary(doc, "title"), stringFromSummary(doc, "name"), accession)
	geneID := firstNonEmpty(stringFromSummary(doc, "gene"), stringFromSummary(doc, "geneid"), accession)
	extra := buildGenericSummaryExtra(spec, id, index, total, accession, title, "", doc)
	setExtraColumns(extra,
		"ncbi_omim_id", accession,
		"ncbi_omim_title", title,
		"ncbi_condition_summary", firstNonEmpty(stringFromSummary(doc, "summary"), stringFromSummary(doc, "text")),
		"ncbi_related_gene_summary", stringFromSummary(doc, "gene"),
		"ncbi_gene_id", geneID,
		"ncbi_omim_text", stringFromSummary(doc, "text"),
	)
	return model.KeywordResultRow{
		SourceDatabase: "ncbi",
		SearchTerm:     keyword,
		SearchType:     "NCBI " + spec.Label,
		LabelName:      title,
		LabelNameType:  "ncbi omim title",
		GeneIdentifier: geneID,
		Description:    title,
		Comments:       joinNonEmpty("; ", stringFromSummary(doc, "summary"), stringFromSummary(doc, "text")),
		AutoDefine:     joinNonEmpty("; ", title, accession),
		ExtraColumns:   extra,
	}
}

func pubmedKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	pmid := firstNonEmpty(stringFromSummary(doc, "uid"), stringFromSummary(doc, "articleids"), strings.TrimSpace(id))
	title := firstNonEmpty(stringFromSummary(doc, "title"), stringFromSummary(doc, "sorttitle"), pmid)
	journal := firstNonEmpty(stringFromSummary(doc, "fulljournalname"), stringFromSummary(doc, "source"))
	pubdate := firstNonEmpty(stringFromSummary(doc, "pubdate"), stringFromSummary(doc, "epubdate"), stringFromSummary(doc, "sortpubdate"))
	authors := firstNonEmpty(stringFromSummary(doc, "authors"), stringFromSummary(doc, "lastauthor"))
	extra := buildGenericSummaryExtra(spec, id, index, total, pmid, title, journal, doc)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		GeneIdentifier:      pmid,
		Genome:              journal,
		Description:         title,
		Comments:            joinNonEmpty("; ", pubdate, authors),
		AutoDefine:          joinNonEmpty("; ", title, journal, pubdate),
		SequenceHeaderLabel: journal,
		ExtraColumns:        extra,
	}
}

func pmcKeywordRowFromSummary(spec SearchType, keyword string, id string, index int, total int, doc map[string]any) model.KeywordResultRow {
	pmcID := firstNonEmpty(stringFromSummary(doc, "uid"), stringFromSummary(doc, "pmcid"), strings.TrimSpace(id))
	title := firstNonEmpty(stringFromSummary(doc, "title"), stringFromSummary(doc, "sorttitle"), pmcID)
	journal := firstNonEmpty(stringFromSummary(doc, "fulljournalname"), stringFromSummary(doc, "source"))
	pubdate := firstNonEmpty(stringFromSummary(doc, "pubdate"), stringFromSummary(doc, "epubdate"), stringFromSummary(doc, "sortpubdate"))
	authors := firstNonEmpty(stringFromSummary(doc, "authors"), stringFromSummary(doc, "lastauthor"))
	extra := buildGenericSummaryExtra(spec, id, index, total, pmcID, title, journal, doc)
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          keyword,
		SearchType:          "NCBI " + spec.Label,
		GeneIdentifier:      pmcID,
		Genome:              journal,
		Description:         title,
		Comments:            joinNonEmpty("; ", pubdate, authors),
		AutoDefine:          joinNonEmpty("; ", title, journal, pubdate),
		SequenceHeaderLabel: journal,
		ExtraColumns:        extra,
	}
}

func buildGenericSummaryExtra(spec SearchType, id string, index int, total int, accession string, title string, organism string, doc map[string]any) map[string]string {
	extra := map[string]string{
		"ncbi_search_type_id":         spec.ID,
		"ncbi_entrez_database":        spec.EntrezDB,
		"ncbi_record_type":            spec.RecordType,
		"ncbi_result_domain":          spec.ResultDomain,
		"ncbi_eutilities_base_url":    eutilsBaseURL,
		"ncbi_engine_schema":          genericRecordSchema,
		"ncbi_uid":                    strings.TrimSpace(id),
		"ncbi_accession":              accession,
		"ncbi_title":                  title,
		"ncbi_organism":               organism,
		"ncbi_generic_rank":           strconv.Itoa(index),
		"ncbi_generic_total":          strconv.Itoa(total),
		"ncbi_summary_json_compact":   compactSummaryJSON(doc),
		"ncbi_summary_primary_fields": summarizeSummaryFields(doc),
	}
	for _, key := range []string{
		"status", "replacedby", "createdate", "updatedate", "taxid", "assemblyaccession",
		"bioprojectaccn", "biosampleaccn", "slen", "moltype", "biomol", "subtype", "subname",
		"rsid", "clinicalsignificance", "variation_set_name", "pubdate", "authors",
		"pmcrefcount", "bookname", "meshheadinglist", "genome", "sra", "expxml", "runs",
		"assemblyname", "assemblystatus", "ftppath_refseq", "ftppath_genbank",
		"project_acc", "projectaccession", "project_type", "project_data_type",
		"accession", "sampleaccn", "sample_name", "sampledata", "biosamplemodel", "owner",
		"scientificname", "commonname", "genbankcommonname", "rank", "division", "lineage",
		"desc", "relevance", "taxonomy", "refseq_category", "submissiondate", "submitter",
		"isolation_source", "host", "geo_loc_name", "collection_date", "strain", "cultivar",
		"tissue", "developmental_stage", "library_strategy", "library_source", "platform",
		"instrument_model", "spots", "bases", "layout", "study_accession", "experiment_accession",
		"run_accession", "clinical_significance", "reviewstatus", "traitset", "variant_type",
		"clinical_assertion", "phenotype", "gene", "geneid", "gene_id", "testtype",
		"method", "labname", "condition", "conceptid", "source", "definition", "text",
		"parenttaxid", "assemblylevel", "organism_name",
	} {
		if value := stringFromSummary(doc, key); value != "" {
			extra["ncbi_"+normalizeSummaryKey(key)] = value
		}
	}
	if replacedBy := stringFromSummary(doc, "replacedby"); replacedBy != "" {
		extra["ncbi_replaced_by"] = replacedBy
	}
	attachStaticJumpMetadata(extra, spec)
	return extra
}

func attachStaticJumpMetadata(extra map[string]string, spec SearchType) {
	if extra == nil {
		return
	}
	targets := ncbiLinkTargetsBySearchType[spec.ID]
	if len(targets) == 0 {
		return
	}
	labels := make([]string, 0, len(targets))
	for i, target := range targets {
		n := strconv.Itoa(i + 1)
		extra["ncbi_jump_"+n+"_dbto"] = target.DBTo
		extra["ncbi_jump_"+n+"_linkname"] = target.LinkName
		extra["ncbi_jump_"+n+"_label"] = target.DisplayLabel
		labels = append(labels, target.DisplayLabel+"("+target.LinkName+")")
	}
	extra["ncbi_jump_targets"] = strings.Join(labels, "; ")
}

func (c *Client) linkedFallbacksForSearchType(searchTypeID string) []linkedKeywordFallback {
	switch strings.ToLower(strings.TrimSpace(searchTypeID)) {
	case "nuccore", "nucleotide":
		return []linkedKeywordFallback{{
			SourceSpec: SearchTypeByID("gene"),
			Targets: []ncbiLinkTarget{
				{DBTo: "nuccore", LinkName: "gene_nuccore_refseqrna", DisplayLabel: "nuccore"},
				{DBTo: "nuccore", LinkName: "gene_nuccore_refseqgenomic", DisplayLabel: "nuccore"},
			},
		}}
	case "clinvar":
		return []linkedKeywordFallback{{
			SourceSpec: SearchTypeByID("gene"),
			Targets: []ncbiLinkTarget{
				{DBTo: "clinvar", LinkName: "gene_clinvar", DisplayLabel: "clinvar"},
			},
		}}
	case "pmc":
		return []linkedKeywordFallback{{
			SourceSpec: SearchTypeByID("pubmed"),
			Targets: []ncbiLinkTarget{
				{DBTo: "pmc", LinkName: "pubmed_pmc", DisplayLabel: "pmc"},
			},
		}}
	case "gene":
		return []linkedKeywordFallback{
			{
				SourceSpec: SearchTypeByID("clinvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gene", LinkName: "clinvar_gene", DisplayLabel: "gene"},
				},
			},
			{
				SourceSpec: SearchTypeByID("medgen"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gene", LinkName: "medgen_gene_diseases", DisplayLabel: "gene"},
				},
			},
			{
				SourceSpec: SearchTypeByID("gtr"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gene", LinkName: "gtr_gene", DisplayLabel: "gene"},
				},
			},
			{
				SourceSpec: SearchTypeByID("omim"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gene", LinkName: "omim_gene", DisplayLabel: "gene"},
				},
			},
			{
				SourceSpec: SearchTypeByID("geoprofiles"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gene", LinkName: "geoprofiles_gene", DisplayLabel: "gene"},
				},
			},
			{
				SourceSpec: SearchTypeByID("pubmed"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gene", LinkName: "pubmed_gene", DisplayLabel: "gene"},
				},
			},
			{
				SourceSpec: SearchTypeByID("pmc"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gene", LinkName: "pmc_gene", DisplayLabel: "gene"},
				},
			},
			{
				SourceSpec: SearchTypeByID("books"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gene", LinkName: "books_gene", DisplayLabel: "gene"},
				},
			},
		}
	case "medgen":
		return []linkedKeywordFallback{
			{
				SourceSpec: SearchTypeByID("clinvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "medgen", LinkName: "clinvar_medgen", DisplayLabel: "medgen"},
				},
			},
			{
				SourceSpec: SearchTypeByID("gtr"),
				Targets: []ncbiLinkTarget{
					{DBTo: "medgen", LinkName: "gtr_medgen", DisplayLabel: "medgen"},
				},
			},
			{
				SourceSpec: SearchTypeByID("omim"),
				Targets: []ncbiLinkTarget{
					{DBTo: "medgen", LinkName: "omim_medgen", DisplayLabel: "medgen"},
				},
			},
			{
				SourceSpec: SearchTypeByID("mesh"),
				Targets: []ncbiLinkTarget{
					{DBTo: "medgen", LinkName: "mesh_medgen", DisplayLabel: "medgen"},
				},
			},
			{
				SourceSpec: SearchTypeByID("books"),
				Targets: []ncbiLinkTarget{
					{DBTo: "medgen", LinkName: "books_medgen", DisplayLabel: "medgen"},
				},
			},
			{
				SourceSpec: SearchTypeByID("pubmed"),
				Targets: []ncbiLinkTarget{
					{DBTo: "medgen", LinkName: "pubmed_medgen", DisplayLabel: "medgen"},
				},
			},
			{
				SourceSpec: SearchTypeByID("pmc"),
				Targets: []ncbiLinkTarget{
					{DBTo: "medgen", LinkName: "pmc_medgen", DisplayLabel: "medgen"},
				},
			},
		}
	case "gtr":
		return []linkedKeywordFallback{
			{
				SourceSpec: SearchTypeByID("clinvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gtr", LinkName: "clinvar_gtr", DisplayLabel: "gtr"},
				},
			},
			{
				SourceSpec: SearchTypeByID("medgen"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gtr", LinkName: "medgen_gtr", DisplayLabel: "gtr"},
				},
			},
			{
				SourceSpec: SearchTypeByID("omim"),
				Targets: []ncbiLinkTarget{
					{DBTo: "gtr", LinkName: "omim_gtr", DisplayLabel: "gtr"},
				},
			},
		}
	case "dbvar":
		return []linkedKeywordFallback{
			{
				SourceSpec: SearchTypeByID("clinvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "dbvar", LinkName: "clinvar_dbvar", DisplayLabel: "dbvar"},
				},
			},
			{
				SourceSpec: SearchTypeByID("snp"),
				Targets: []ncbiLinkTarget{
					{DBTo: "dbvar", LinkName: "snp_dbvar", DisplayLabel: "dbvar"},
				},
			},
			{
				SourceSpec: SearchTypeByID("omim"),
				Targets: []ncbiLinkTarget{
					{DBTo: "dbvar", LinkName: "omim_dbvar", DisplayLabel: "dbvar"},
				},
			},
			{
				SourceSpec: SearchTypeByID("gds"),
				Targets: []ncbiLinkTarget{
					{DBTo: "dbvar", LinkName: "gds_dbvar", DisplayLabel: "dbvar"},
				},
			},
		}
	case "snp":
		return []linkedKeywordFallback{
			{
				SourceSpec: SearchTypeByID("clinvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "snp", LinkName: "clinvar_snp", DisplayLabel: "snp"},
				},
			},
			{
				SourceSpec: SearchTypeByID("dbvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "snp", LinkName: "dbvar_snp", DisplayLabel: "snp"},
				},
			},
			{
				SourceSpec: SearchTypeByID("pubmed"),
				Targets: []ncbiLinkTarget{
					{DBTo: "snp", LinkName: "pubmed_snp", DisplayLabel: "snp"},
				},
			},
			{
				SourceSpec: SearchTypeByID("pmc"),
				Targets: []ncbiLinkTarget{
					{DBTo: "snp", LinkName: "pmc_snp", DisplayLabel: "snp"},
				},
			},
		}
	case "omim":
		return []linkedKeywordFallback{
			{
				SourceSpec: SearchTypeByID("clinvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "omim", LinkName: "clinvar_omim", DisplayLabel: "omim"},
				},
			},
			{
				SourceSpec: SearchTypeByID("medgen"),
				Targets: []ncbiLinkTarget{
					{DBTo: "omim", LinkName: "medgen_omim", DisplayLabel: "omim"},
				},
			},
			{
				SourceSpec: SearchTypeByID("gtr"),
				Targets: []ncbiLinkTarget{
					{DBTo: "omim", LinkName: "gtr_omim", DisplayLabel: "omim"},
				},
			},
			{
				SourceSpec: SearchTypeByID("dbvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "omim", LinkName: "dbvar_omim", DisplayLabel: "omim"},
				},
			},
			{
				SourceSpec: SearchTypeByID("geoprofiles"),
				Targets: []ncbiLinkTarget{
					{DBTo: "omim", LinkName: "geoprofiles_omim", DisplayLabel: "omim"},
				},
			},
			{
				SourceSpec: SearchTypeByID("pubmed"),
				Targets: []ncbiLinkTarget{
					{DBTo: "omim", LinkName: "pubmed_omim_cited", DisplayLabel: "omim"},
				},
			},
			{
				SourceSpec: SearchTypeByID("books"),
				Targets: []ncbiLinkTarget{
					{DBTo: "omim", LinkName: "books_omim", DisplayLabel: "omim"},
				},
			},
		}
	case "pubmed":
		return []linkedKeywordFallback{
			{
				SourceSpec: SearchTypeByID("pmc"),
				Targets: []ncbiLinkTarget{
					{DBTo: "pubmed", LinkName: "pmc_pubmed", DisplayLabel: "pubmed"},
				},
			},
			{
				SourceSpec: SearchTypeByID("clinvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "pubmed", LinkName: "clinvar_pubmed", DisplayLabel: "pubmed"},
				},
			},
			{
				SourceSpec: SearchTypeByID("dbvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "pubmed", LinkName: "dbvar_pubmed", DisplayLabel: "pubmed"},
				},
			},
			{
				SourceSpec: SearchTypeByID("medgen"),
				Targets: []ncbiLinkTarget{
					{DBTo: "pubmed", LinkName: "medgen_pubmed", DisplayLabel: "pubmed"},
				},
			},
			{
				SourceSpec: SearchTypeByID("omim"),
				Targets: []ncbiLinkTarget{
					{DBTo: "pubmed", LinkName: "omim_pubmed_cited", DisplayLabel: "pubmed"},
				},
			},
			{
				SourceSpec: SearchTypeByID("gds"),
				Targets: []ncbiLinkTarget{
					{DBTo: "pubmed", LinkName: "gds_pubmed", DisplayLabel: "pubmed"},
				},
			},
			{
				SourceSpec: SearchTypeByID("geoprofiles"),
				Targets: []ncbiLinkTarget{
					{DBTo: "pubmed", LinkName: "geoprofiles_pubmed", DisplayLabel: "pubmed"},
				},
			},
			{
				SourceSpec: SearchTypeByID("books"),
				Targets: []ncbiLinkTarget{
					{DBTo: "pubmed", LinkName: "books_pubmed_refs", DisplayLabel: "pubmed"},
				},
			},
		}
	case "bioproject":
		return []linkedKeywordFallback{
			{
				SourceSpec: SearchTypeByID("assembly"),
				Targets: []ncbiLinkTarget{
					{DBTo: "bioproject", LinkName: "assembly_bioproject", DisplayLabel: "bioproject"},
				},
			},
			{
				SourceSpec: SearchTypeByID("biosample"),
				Targets: []ncbiLinkTarget{
					{DBTo: "bioproject", LinkName: "biosample_bioproject", DisplayLabel: "bioproject"},
				},
			},
			{
				SourceSpec: SearchTypeByID("dbvar"),
				Targets: []ncbiLinkTarget{
					{DBTo: "bioproject", LinkName: "dbvar_bioproject", DisplayLabel: "bioproject"},
				},
			},
			{
				SourceSpec: SearchTypeByID("gds"),
				Targets: []ncbiLinkTarget{
					{DBTo: "bioproject", LinkName: "gds_bioproject", DisplayLabel: "bioproject"},
				},
			},
			{
				SourceSpec: SearchTypeByID("pmc"),
				Targets: []ncbiLinkTarget{
					{DBTo: "bioproject", LinkName: "pmc_bioproject", DisplayLabel: "bioproject"},
				},
			},
		}
	case "books":
		return []linkedKeywordFallback{
			{
				SourceSpec: SearchTypeByID("medgen"),
				Targets: []ncbiLinkTarget{
					{DBTo: "books", LinkName: "medgen_books", DisplayLabel: "books"},
				},
			},
			{
				SourceSpec: SearchTypeByID("omim"),
				Targets: []ncbiLinkTarget{
					{DBTo: "books", LinkName: "omim_books", DisplayLabel: "books"},
				},
			},
		}
	case "geoprofiles":
		return []linkedKeywordFallback{
			{
				SourceSpec: SearchTypeByID("gds"),
				Targets: []ncbiLinkTarget{
					{DBTo: "geoprofiles", LinkName: "gds_geoprofiles", DisplayLabel: "geoprofiles"},
				},
			},
		}
	default:
		return nil
	}
}

func (c *Client) searchLinkedKeywordRows(ctx context.Context, targetSpec SearchType, keyword string, fallback linkedKeywordFallback) ([]model.KeywordResultRow, error) {
	sourceIDs, _, _, _, err := c.searchGenericIDs(ctx, fallback.SourceSpec, keyword, defaultRetMax)
	if err != nil {
		return nil, err
	}
	if len(sourceIDs) == 0 {
		return nil, nil
	}
	targetIDs, linkName, err := c.fetchLinkedIDsForTargets(ctx, fallback.SourceSpec, targetSpec, sourceIDs, fallback.Targets)
	if err != nil {
		return nil, err
	}
	if len(targetIDs) == 0 {
		return nil, nil
	}
	if targetSpec.ID == "protein" {
		records, err := c.fetchProteinRecords(ctx, targetIDs)
		if err != nil {
			return nil, err
		}
		rows := make([]model.KeywordResultRow, 0, len(records))
		for _, record := range records {
			row := keywordRowFromProteinRecord(keyword, record)
			if strings.TrimSpace(row.SequenceID) == "" || strings.TrimSpace(record.Sequence) == "" {
				continue
			}
			rows = append(rows, row)
		}
		return annotateLinkedKeywordRows(rows, targetSpec, fallback.SourceSpec, linkName, sourceIDs, targetIDs, keyword), nil
	}
	rows, err := c.fetchGenericSummaryRows(ctx, targetSpec, keyword, targetIDs, "", "", len(targetIDs))
	if err != nil {
		return nil, err
	}
	return annotateLinkedKeywordRows(rows, targetSpec, fallback.SourceSpec, linkName, sourceIDs, targetIDs, keyword), nil
}

func (c *Client) fetchLinkedIDsForTargets(ctx context.Context, sourceSpec SearchType, targetSpec SearchType, sourceIDs []string, targets []ncbiLinkTarget) ([]string, string, error) {
	for _, target := range targets {
		if !strings.EqualFold(strings.TrimSpace(target.DBTo), targetSpec.EntrezDB) {
			continue
		}
		ids, linkName, err := c.fetchLinkedIDs(ctx, sourceSpec.EntrezDB, targetSpec.EntrezDB, sourceIDs, []string{target.LinkName})
		if err != nil {
			return nil, "", err
		}
		if len(ids) > 0 {
			return ids, linkName, nil
		}
	}
	return nil, "", nil
}

func (c *Client) fetchLinkedIDs(ctx context.Context, dbFrom string, dbTo string, sourceIDs []string, linkNames []string) ([]string, string, error) {
	sourceIDs = uniqueNonEmpty(sourceIDs)
	linkNames = uniqueNonEmpty(linkNames)
	if len(sourceIDs) == 0 {
		return nil, "", nil
	}
	if len(linkNames) == 0 {
		linkNames = []string{""}
	}
	for _, linkName := range linkNames {
		values := url.Values{}
		values.Set("dbfrom", strings.TrimSpace(dbFrom))
		values.Set("db", strings.TrimSpace(dbTo))
		values.Set("id", strings.Join(sourceIDs, ","))
		values.Set("retmode", "json")
		if strings.TrimSpace(linkName) != "" {
			values.Set("linkname", strings.TrimSpace(linkName))
		}
		var payload eLinkResponse
		if err := c.getJSON(ctx, "elink.fcgi", values, &payload); err != nil {
			return nil, "", err
		}
		linked := extractELinkTargetIDs(payload, dbTo, linkName)
		if len(linked) > 0 {
			return linked, firstNonEmpty(strings.TrimSpace(linkName), dbFrom+"->"+dbTo), nil
		}
	}
	return nil, "", nil
}

func extractELinkTargetIDs(payload eLinkResponse, dbTo string, linkName string) []string {
	out := make([]string, 0)
	for _, linkSet := range payload.LinkSets {
		for _, db := range linkSet.LinkSetDBs {
			if strings.TrimSpace(dbTo) != "" && !strings.EqualFold(strings.TrimSpace(db.DBTo), strings.TrimSpace(dbTo)) {
				continue
			}
			if strings.TrimSpace(linkName) != "" && !strings.EqualFold(strings.TrimSpace(db.LinkName), strings.TrimSpace(linkName)) {
				continue
			}
			for _, id := range db.Links {
				if value := strings.TrimSpace(fmt.Sprint(id)); value != "" {
					out = append(out, value)
				}
			}
		}
	}
	return uniqueNonEmpty(out)
}

func annotateLinkedKeywordRows(rows []model.KeywordResultRow, targetSpec SearchType, sourceSpec SearchType, linkName string, sourceIDs []string, targetIDs []string, keyword string) []model.KeywordResultRow {
	out := append([]model.KeywordResultRow(nil), rows...)
	for i := range out {
		if out[i].ExtraColumns == nil {
			out[i].ExtraColumns = make(map[string]string)
		}
		out[i].SearchType = "NCBI " + sourceSpec.Label + " -> " + targetSpec.Label
		out[i].SearchTerm = keyword
		out[i].ExtraColumns["ncbi_link_resolution"] = "elink"
		out[i].ExtraColumns["ncbi_linked_from_db"] = sourceSpec.EntrezDB
		out[i].ExtraColumns["ncbi_linked_to_db"] = targetSpec.EntrezDB
		out[i].ExtraColumns["ncbi_linked_from_search_type_id"] = sourceSpec.ID
		out[i].ExtraColumns["ncbi_linked_to_search_type_id"] = targetSpec.ID
		out[i].ExtraColumns["ncbi_linkname"] = strings.TrimSpace(linkName)
		out[i].ExtraColumns["ncbi_link_source_ids"] = strings.Join(uniqueNonEmpty(sourceIDs), ",")
		out[i].ExtraColumns["ncbi_link_target_ids"] = strings.Join(uniqueNonEmpty(targetIDs), ",")
		out[i].ExtraColumns["ncbi_link_source_keyword"] = strings.TrimSpace(keyword)
	}
	return out
}

func summaryLengthText(doc map[string]any) string {
	length := firstNonEmpty(stringFromSummary(doc, "slen"), stringFromSummary(doc, "length"))
	if length == "" {
		return ""
	}
	return length + " bp"
}

func joinNonEmpty(sep string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(uniqueNonEmpty(parts), sep)
}

func stringFromSummary(doc map[string]any, key string) string {
	if doc == nil {
		return ""
	}
	value, ok := doc[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			switch v := item.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					parts = append(parts, strings.TrimSpace(v))
				}
			case map[string]any:
				if name := firstNonEmpty(stringFromSummary(v, "name"), stringFromSummary(v, "caption"), stringFromSummary(v, "value")); name != "" {
					parts = append(parts, name)
				}
			}
		}
		return strings.Join(uniqueNonEmpty(parts), "; ")
	case map[string]any:
		return firstNonEmpty(
			stringFromSummary(typed, "name"),
			stringFromSummary(typed, "value"),
			stringFromSummary(typed, "caption"),
			stringFromSummary(typed, "scientificname"),
		)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func stringFromNestedSummary(doc map[string]any, parent string, key string) string {
	if doc == nil {
		return ""
	}
	value, ok := doc[parent]
	if !ok || value == nil {
		return ""
	}
	typed, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return stringFromSummary(typed, key)
}

func compactSummaryJSON(doc map[string]any) string {
	if doc == nil {
		return ""
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	return string(data)
}

func summarizeSummaryFields(doc map[string]any) string {
	keys := []string{
		"uid", "caption", "title", "name", "description", "organism", "taxname", "accessionversion",
		"assemblyaccession", "status", "createdate", "updatedate", "pubdate", "rsid", "clinicalsignificance",
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := stringFromSummary(doc, key); value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	return strings.Join(parts, "; ")
}

type sraExperimentMeta struct {
	StudyAccession      string
	ExperimentAccession string
	BioProjectAccession string
	BioSampleAccession  string
	LibraryStrategy     string
	LibrarySource       string
	Platform            string
	InstrumentModel     string
	Layout              string
}

type sraRunMeta struct {
	RunAccession        string
	ExperimentAccession string
	StudyAccession      string
	BioSampleAccession  string
	InstrumentModel     string
	Platform            string
	Spots               string
	Bases               string
}

func parseBioSampleAttributeBag(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == '|' || r == '\n' || r == '\r' || r == '\t'
	})
	out := make(map[string]string)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var key, value string
		switch {
		case strings.Contains(part, "="):
			split := strings.SplitN(part, "=", 2)
			key, value = split[0], split[1]
		case strings.Contains(part, ":"):
			split := strings.SplitN(part, ":", 2)
			key, value = split[0], split[1]
		default:
			continue
		}
		key = normalizeSummaryKey(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = value
		}
	}
	return out
}

func attachNormalizedAttributeExtras(extra map[string]string, prefix string, attrs map[string]string) {
	if extra == nil || len(attrs) == 0 {
		return
	}
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.TrimSpace(attrs[key])
		if value == "" {
			continue
		}
		extra[prefix+normalizeSummaryKey(key)] = value
	}
}

type sraExperimentPackage struct {
	Experiment struct {
		Accession string `xml:"accession,attr"`
	} `xml:"EXPERIMENT"`
	Study struct {
		Accession string `xml:"accession,attr"`
	} `xml:"STUDY"`
	Sample struct {
		Accession string `xml:"accession,attr"`
	} `xml:"SAMPLE"`
	ExperimentRef struct {
		Name string `xml:"refname,attr"`
	} `xml:"EXPERIMENT_REF"`
	StudyRef struct {
		Name string `xml:"refname,attr"`
	} `xml:"STUDY_REF"`
	SampleRef struct {
		Name string `xml:"refname,attr"`
	} `xml:"SAMPLE_REF"`
	Submission struct {
		Accession string `xml:"accession,attr"`
	} `xml:"SUBMISSION"`
	LibraryDescriptor struct {
		LibraryStrategy string `xml:"LIBRARY_STRATEGY"`
		LibrarySource   string `xml:"LIBRARY_SOURCE"`
		LibraryLayout   struct {
			Single *struct{} `xml:"SINGLE"`
			Paired *struct{} `xml:"PAIRED"`
		} `xml:"LIBRARY_LAYOUT"`
	} `xml:"LIBRARY_DESCRIPTOR"`
	Platform struct {
		ILLUMINA *struct {
			InstrumentModel string `xml:"INSTRUMENT_MODEL"`
		} `xml:"ILLUMINA"`
		LS454 *struct {
			InstrumentModel string `xml:"INSTRUMENT_MODEL"`
		} `xml:"LS454"`
		IONTORRENT *struct {
			InstrumentModel string `xml:"INSTRUMENT_MODEL"`
		} `xml:"ION_TORRENT"`
		PACBIO_SMRT *struct {
			InstrumentModel string `xml:"INSTRUMENT_MODEL"`
		} `xml:"PACBIO_SMRT"`
		OXFORD_NANOPORE *struct {
			InstrumentModel string `xml:"INSTRUMENT_MODEL"`
		} `xml:"OXFORD_NANOPORE"`
		BGISEQ *struct {
			InstrumentModel string `xml:"INSTRUMENT_MODEL"`
		} `xml:"BGISEQ"`
	} `xml:"PLATFORM"`
}

type sraRunSet struct {
	Runs []struct {
		Accession       string `xml:"accession,attr"`
		ExperimentAcc   string `xml:"experiment_ref,attr"`
		StudyAcc        string `xml:"study_ref,attr"`
		SampleAcc       string `xml:"sample_name,attr"`
		InstrumentModel string `xml:"instrument_model,attr"`
		RunCenter       string `xml:"run_center,attr"`
		Spots           string `xml:"total_spots,attr"`
		Bases           string `xml:"total_bases,attr"`
	} `xml:"RUN"`
}

type clinVarSet struct {
	Title                     string `xml:"Title"`
	ReferenceClinVarAssertion struct {
		ClinicalSignificance struct {
			Description  string `xml:"Description"`
			ReviewStatus string `xml:"ReviewStatus"`
		} `xml:"ClinicalSignificance"`
		TraitSet struct {
			Traits []struct {
				Name []struct {
					ElementValue struct {
						Value string `xml:",chardata"`
					} `xml:"ElementValue"`
				} `xml:"Name"`
			} `xml:"Trait"`
		} `xml:"TraitSet"`
	} `xml:"ReferenceClinVarAssertion"`
	MeasureSet struct {
		Type string `xml:"Type,attr"`
	} `xml:"MeasureSet"`
}

type gtrTestReport struct {
	Test struct {
		Name           string `xml:"Name"`
		ClinicalDomain struct {
			Diseases []struct {
				Name string `xml:"Name"`
			} `xml:"Disease"`
		} `xml:"ClinicalDomain"`
		Method struct {
			MethodCategory string `xml:"MethodCategory"`
		} `xml:"Method"`
		Laboratory struct {
			Name string `xml:"Name"`
		} `xml:"Laboratory"`
	} `xml:"Test"`
}

type bioProjectRecordSet struct {
	DocumentSummary struct {
		Project struct {
			ProjectID struct {
				ArchiveID struct {
					Accession string `xml:"accession,attr"`
					ID        string `xml:"id,attr"`
				} `xml:"ArchiveID"`
			} `xml:"ProjectID"`
			ProjectDescr struct {
				Name        string `xml:"Name"`
				Title       string `xml:"Title"`
				Description string `xml:"Description"`
			} `xml:"ProjectDescr"`
			ProjectType struct {
				ProjectTypeSubmission struct {
					Target struct {
						Capture     string `xml:"capture,attr"`
						Material    string `xml:"material,attr"`
						SampleScope string `xml:"sample_scope,attr"`
						Organism    struct {
							TaxID        string `xml:"taxID,attr"`
							OrganismName string `xml:"OrganismName"`
						} `xml:"Organism"`
					} `xml:"Target"`
					Method struct {
						MethodType string `xml:"method_type,attr"`
					} `xml:"Method"`
					Objectives struct {
						Data struct {
							DataType string `xml:"data_type,attr"`
							Value    string `xml:",chardata"`
						} `xml:"Data"`
					} `xml:"Objectives"`
					ProjectDataTypeSet struct {
						DataType []string `xml:"DataType"`
					} `xml:"ProjectDataTypeSet"`
				} `xml:"ProjectTypeSubmission"`
			} `xml:"ProjectType"`
		} `xml:"Project"`
		Submission struct {
			Submitted   string `xml:"submitted,attr"`
			Description struct {
				Organization struct {
					Name string `xml:"Name"`
				} `xml:"Organization"`
				Access string `xml:"Access"`
			} `xml:"Description"`
		} `xml:"Submission"`
	} `xml:"DocumentSummary"`
}

func parseSRAExperimentXML(raw string) sraExperimentMeta {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sraExperimentMeta{}
	}
	wrapped := raw
	if !strings.Contains(raw, "<EXPERIMENT_PACKAGE") {
		wrapped = "<EXPERIMENT_PACKAGE>" + raw + "</EXPERIMENT_PACKAGE>"
	}
	var pkg sraExperimentPackage
	if err := xml.Unmarshal([]byte(wrapped), &pkg); err != nil {
		return sraExperimentMeta{}
	}
	meta := sraExperimentMeta{
		StudyAccession:      firstNonEmpty(pkg.Study.Accession, pkg.StudyRef.Name),
		ExperimentAccession: firstNonEmpty(pkg.Experiment.Accession, pkg.ExperimentRef.Name),
		BioSampleAccession:  firstNonEmpty(pkg.Sample.Accession, pkg.SampleRef.Name),
		LibraryStrategy:     strings.TrimSpace(pkg.LibraryDescriptor.LibraryStrategy),
		LibrarySource:       strings.TrimSpace(pkg.LibraryDescriptor.LibrarySource),
	}
	switch {
	case pkg.LibraryDescriptor.LibraryLayout.Paired != nil:
		meta.Layout = "PAIRED"
	case pkg.LibraryDescriptor.LibraryLayout.Single != nil:
		meta.Layout = "SINGLE"
	}
	switch {
	case pkg.Platform.ILLUMINA != nil:
		meta.Platform = "ILLUMINA"
		meta.InstrumentModel = strings.TrimSpace(pkg.Platform.ILLUMINA.InstrumentModel)
	case pkg.Platform.LS454 != nil:
		meta.Platform = "LS454"
		meta.InstrumentModel = strings.TrimSpace(pkg.Platform.LS454.InstrumentModel)
	case pkg.Platform.IONTORRENT != nil:
		meta.Platform = "ION_TORRENT"
		meta.InstrumentModel = strings.TrimSpace(pkg.Platform.IONTORRENT.InstrumentModel)
	case pkg.Platform.PACBIO_SMRT != nil:
		meta.Platform = "PACBIO_SMRT"
		meta.InstrumentModel = strings.TrimSpace(pkg.Platform.PACBIO_SMRT.InstrumentModel)
	case pkg.Platform.OXFORD_NANOPORE != nil:
		meta.Platform = "OXFORD_NANOPORE"
		meta.InstrumentModel = strings.TrimSpace(pkg.Platform.OXFORD_NANOPORE.InstrumentModel)
	case pkg.Platform.BGISEQ != nil:
		meta.Platform = "BGISEQ"
		meta.InstrumentModel = strings.TrimSpace(pkg.Platform.BGISEQ.InstrumentModel)
	}
	return meta
}

func parseSRARuns(raw string) sraRunMeta {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sraRunMeta{}
	}
	wrapped := raw
	if !strings.Contains(raw, "<RUN_SET") {
		wrapped = "<RUN_SET>" + raw + "</RUN_SET>"
	}
	var set sraRunSet
	if err := xml.Unmarshal([]byte(wrapped), &set); err != nil {
		return sraRunMeta{}
	}
	if len(set.Runs) == 0 {
		return sraRunMeta{}
	}
	first := set.Runs[0]
	return sraRunMeta{
		RunAccession:        strings.TrimSpace(first.Accession),
		ExperimentAccession: strings.TrimSpace(first.ExperimentAcc),
		StudyAccession:      strings.TrimSpace(first.StudyAcc),
		BioSampleAccession:  strings.TrimSpace(first.SampleAcc),
		InstrumentModel:     strings.TrimSpace(first.InstrumentModel),
		Platform:            strings.TrimSpace(first.RunCenter),
		Spots:               strings.TrimSpace(first.Spots),
		Bases:               strings.TrimSpace(first.Bases),
	}
}

func parseClinVarSetXML(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var set clinVarSet
	if err := xml.Unmarshal([]byte(raw), &set); err != nil {
		return nil
	}
	traits := make([]string, 0)
	for _, trait := range set.ReferenceClinVarAssertion.TraitSet.Traits {
		for _, name := range trait.Name {
			if value := strings.TrimSpace(name.ElementValue.Value); value != "" {
				traits = append(traits, value)
			}
		}
	}
	out := map[string]string{
		"ncbi_efetch_title":                 strings.TrimSpace(set.Title),
		"ncbi_efetch_clinical_significance": strings.TrimSpace(set.ReferenceClinVarAssertion.ClinicalSignificance.Description),
		"ncbi_efetch_review_status":         strings.TrimSpace(set.ReferenceClinVarAssertion.ClinicalSignificance.ReviewStatus),
		"ncbi_efetch_condition":             joinNonEmpty("; ", traits...),
		"ncbi_efetch_variant_type":          strings.TrimSpace(set.MeasureSet.Type),
	}
	return out
}

func parseGTRTestReportXML(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var report gtrTestReport
	if err := xml.Unmarshal([]byte(raw), &report); err != nil {
		return nil
	}
	diseases := make([]string, 0, len(report.Test.ClinicalDomain.Diseases))
	for _, disease := range report.Test.ClinicalDomain.Diseases {
		if value := strings.TrimSpace(disease.Name); value != "" {
			diseases = append(diseases, value)
		}
	}
	return map[string]string{
		"ncbi_efetch_test_name": strings.TrimSpace(report.Test.Name),
		"ncbi_efetch_condition": joinNonEmpty("; ", diseases...),
		"ncbi_efetch_method":    strings.TrimSpace(report.Test.Method.MethodCategory),
		"ncbi_efetch_lab":       strings.TrimSpace(report.Test.Laboratory.Name),
	}
}

func parseBioProjectXML(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var record bioProjectRecordSet
	if err := xml.Unmarshal([]byte(raw), &record); err != nil {
		return nil
	}
	submission := record.DocumentSummary.Project.ProjectType.ProjectTypeSubmission
	target := submission.Target
	return map[string]string{
		"ncbi_efetch_bioproject_accession": record.DocumentSummary.Project.ProjectID.ArchiveID.Accession,
		"ncbi_efetch_project_title":        strings.TrimSpace(record.DocumentSummary.Project.ProjectDescr.Title),
		"ncbi_efetch_project_description":  strings.TrimSpace(record.DocumentSummary.Project.ProjectDescr.Description),
		"ncbi_efetch_organism":             strings.TrimSpace(target.Organism.OrganismName),
		"ncbi_efetch_taxonomy_id":          strings.TrimSpace(target.Organism.TaxID),
		"ncbi_efetch_project_material":     strings.TrimSpace(target.Material),
		"ncbi_efetch_project_scope":        strings.TrimSpace(target.SampleScope),
		"ncbi_efetch_project_capture":      strings.TrimSpace(target.Capture),
		"ncbi_efetch_project_method_type":  strings.TrimSpace(submission.Method.MethodType),
		"ncbi_efetch_project_data_type":    firstNonEmpty(strings.TrimSpace(submission.Objectives.Data.DataType), joinNonEmpty("; ", submission.ProjectDataTypeSet.DataType...)),
		"ncbi_efetch_submission_date":      strings.TrimSpace(record.DocumentSummary.Submission.Submitted),
		"ncbi_efetch_submitter":            strings.TrimSpace(record.DocumentSummary.Submission.Description.Organization.Name),
		"ncbi_efetch_project_access":       strings.TrimSpace(record.DocumentSummary.Submission.Description.Access),
	}
}

func setExtraColumns(extra map[string]string, kv ...string) {
	if extra == nil {
		return
	}
	for i := 0; i+1 < len(kv); i += 2 {
		key := strings.TrimSpace(kv[i])
		value := strings.TrimSpace(kv[i+1])
		if key == "" || value == "" {
			continue
		}
		extra[key] = value
	}
}

func (c *Client) enrichClinVarRowsWithEFetch(ctx context.Context, rows []model.KeywordResultRow) []model.KeywordResultRow {
	out := append([]model.KeywordResultRow(nil), rows...)
	for i := range out {
		accession := firstNonEmpty(
			strings.TrimSpace(out[i].ExtraColumns["ncbi_clinvar_accession"]),
			strings.TrimSpace(out[i].ExtraColumns["ncbi_accession"]),
			strings.TrimSpace(out[i].GeneIdentifier),
		)
		if accession == "" {
			continue
		}
		raw, err := c.fetchTextCached(ctx, "clinvar-xml", []string{accession}, url.Values{"db": {"clinvar"}, "rettype": {"clinvarset"}, "retmode": {"xml"}})
		if err != nil {
			continue
		}
		enriched := parseClinVarSetXML(raw)
		if len(enriched) == 0 {
			continue
		}
		if out[i].ExtraColumns == nil {
			out[i].ExtraColumns = make(map[string]string)
		}
		for key, value := range enriched {
			if strings.TrimSpace(value) != "" {
				out[i].ExtraColumns[key] = strings.TrimSpace(value)
			}
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_clinical_significance"], out[i].ExtraColumns["ncbi_clinical_significance"]); value != "" {
			out[i].ExtraColumns["ncbi_clinical_significance"] = value
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_review_status"], out[i].ExtraColumns["ncbi_review_status"]); value != "" {
			out[i].ExtraColumns["ncbi_review_status"] = value
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_condition"], out[i].ExtraColumns["ncbi_condition"]); value != "" {
			out[i].ExtraColumns["ncbi_condition"] = value
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_variant_type"], out[i].ExtraColumns["ncbi_variant_type"]); value != "" {
			out[i].ExtraColumns["ncbi_variant_type"] = value
		}
	}
	return out
}

func (c *Client) enrichGTRRowsWithEFetch(ctx context.Context, rows []model.KeywordResultRow) []model.KeywordResultRow {
	out := append([]model.KeywordResultRow(nil), rows...)
	for i := range out {
		accession := firstNonEmpty(
			strings.TrimSpace(out[i].ExtraColumns["ncbi_gtr_accession"]),
			strings.TrimSpace(out[i].ExtraColumns["ncbi_accession"]),
			strings.TrimSpace(out[i].GeneIdentifier),
		)
		if accession == "" {
			continue
		}
		raw, err := c.fetchTextCached(ctx, "gtr-xml", []string{accession}, url.Values{"db": {"gtr"}, "rettype": {"gtracc"}, "retmode": {"xml"}})
		if err != nil {
			continue
		}
		enriched := parseGTRTestReportXML(raw)
		if len(enriched) == 0 {
			continue
		}
		if out[i].ExtraColumns == nil {
			out[i].ExtraColumns = make(map[string]string)
		}
		for key, value := range enriched {
			if strings.TrimSpace(value) != "" {
				out[i].ExtraColumns[key] = strings.TrimSpace(value)
			}
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_test_name"], out[i].LabelName); value != "" && strings.TrimSpace(out[i].LabelName) == "" {
			out[i].LabelName = value
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_condition"], out[i].ExtraColumns["ncbi_condition"]); value != "" {
			out[i].ExtraColumns["ncbi_condition"] = value
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_method"], out[i].ExtraColumns["ncbi_method"]); value != "" {
			out[i].ExtraColumns["ncbi_method"] = value
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_lab"], out[i].ExtraColumns["ncbi_lab"]); value != "" {
			out[i].ExtraColumns["ncbi_lab"] = value
		}
	}
	return out
}

func (c *Client) enrichBioProjectRowsWithEFetch(ctx context.Context, rows []model.KeywordResultRow) []model.KeywordResultRow {
	out := append([]model.KeywordResultRow(nil), rows...)
	for i := range out {
		accession := firstNonEmpty(
			strings.TrimSpace(out[i].ExtraColumns["ncbi_bioproject_accession"]),
			strings.TrimSpace(out[i].ExtraColumns["ncbi_accession"]),
			strings.TrimSpace(out[i].GeneIdentifier),
		)
		if accession == "" {
			continue
		}
		raw, err := c.fetchTextCached(ctx, "bioproject-xml", []string{accession}, url.Values{"db": {"bioproject"}, "retmode": {"xml"}})
		if err != nil {
			continue
		}
		enriched := parseBioProjectXML(raw)
		if len(enriched) == 0 {
			continue
		}
		if out[i].ExtraColumns == nil {
			out[i].ExtraColumns = make(map[string]string)
		}
		for key, value := range enriched {
			if strings.TrimSpace(value) != "" {
				out[i].ExtraColumns[key] = strings.TrimSpace(value)
			}
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_project_data_type"], out[i].ExtraColumns["ncbi_project_data_type"]); value != "" {
			out[i].ExtraColumns["ncbi_project_data_type"] = value
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_project_description"], out[i].ExtraColumns["ncbi_project_description"]); value != "" {
			out[i].ExtraColumns["ncbi_project_description"] = value
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_taxonomy_id"], out[i].ExtraColumns["ncbi_taxonomy_id"]); value != "" {
			out[i].ExtraColumns["ncbi_taxonomy_id"] = value
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_submission_date"], out[i].ExtraColumns["ncbi_submission_date"]); value != "" {
			out[i].ExtraColumns["ncbi_submission_date"] = value
		}
		if value := firstNonEmpty(enriched["ncbi_efetch_submitter"], out[i].ExtraColumns["ncbi_submitter"]); value != "" {
			out[i].ExtraColumns["ncbi_submitter"] = value
		}
	}
	return out
}

func normalizeSummaryKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = strings.ReplaceAll(key, " ", "_")
	key = strings.ReplaceAll(key, "-", "_")
	return key
}

func (c *Client) SubmitBlast(ctx context.Context, req model.BlastRequest) (model.BlastJob, error) {
	_ = ctx
	_ = req
	return model.BlastJob{}, fmt.Errorf("NCBI protein source supports keyword/FASTA retrieval only")
}

func (c *Client) WaitForBlastResults(ctx context.Context, jobID string, pollInterval time.Duration, timeout time.Duration) (model.BlastResult, error) {
	_ = ctx
	_ = jobID
	_ = pollInterval
	_ = timeout
	return model.BlastResult{}, fmt.Errorf("NCBI protein source supports keyword/FASTA retrieval only")
}

func (c *Client) searchProteinIDs(ctx context.Context, term string, limit int) ([]string, error) {
	key := strings.Join([]string{"esearch", term, strconv.Itoa(limit)}, "|")
	if cached, ok := readCachedJSON[[]string]("search-ids", key); ok {
		return cached, nil
	}
	values := url.Values{}
	values.Set("db", "protein")
	values.Set("term", term)
	values.Set("retmode", "json")
	values.Set("retmax", strconv.Itoa(limit))
	var payload eSearchResponse
	if err := c.getJSON(ctx, "esearch.fcgi", values, &payload); err != nil {
		return nil, err
	}
	ids := uniqueNonEmpty(payload.SearchResult.IDs)
	writeCachedJSON("search-ids", key, ids)
	return ids, nil
}

func (c *Client) searchNucleotideIDs(ctx context.Context, term string, limit int) ([]string, error) {
	key := strings.Join([]string{"esearch-nuccore", term, strconv.Itoa(limit)}, "|")
	if cached, ok := readCachedJSON[[]string]("search-ids", key); ok {
		return cached, nil
	}
	values := url.Values{}
	values.Set("db", "nuccore")
	values.Set("term", term)
	values.Set("retmode", "json")
	values.Set("retmax", strconv.Itoa(limit))
	var payload eSearchResponse
	if err := c.getJSON(ctx, "esearch.fcgi", values, &payload); err != nil {
		return nil, err
	}
	ids := uniqueNonEmpty(payload.SearchResult.IDs)
	writeCachedJSON("search-ids", key, ids)
	return ids, nil
}

func (c *Client) fetchNucleotideCDSRecords(ctx context.Context, ids []string) ([]nucleotideCDSRecord, error) {
	ids = uniqueNonEmpty(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	gbText, err := c.fetchTextCached(ctx, "nucleotide-genbank", ids, url.Values{"db": {"nuccore"}, "rettype": {"gb"}, "retmode": {"text"}})
	if err != nil {
		return nil, err
	}
	return parseNucleotideCDSRecords(gbText), nil
}

func (c *Client) fetchProteinRecords(ctx context.Context, ids []string) ([]proteinRecord, error) {
	ids = uniqueNonEmpty(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	records := make([]proteinRecord, 0, len(ids))
	missing := make([]string, 0, len(ids))
	for _, id := range ids {
		if record, ok := c.cachedProteinRecord(id); ok {
			records = append(records, record)
		} else {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return records, nil
	}

	fetched, err := c.fetchProteinRecordsRemote(ctx, missing)
	if err != nil {
		return nil, err
	}
	records = append(records, fetched...)
	return records, nil
}

func (c *Client) cachedProteinRecord(id string) (proteinRecord, bool) {
	memoryKey := strings.ToUpper(strings.TrimSpace(id))
	if memoryKey == "" {
		return proteinRecord{}, false
	}
	c.mu.RLock()
	if cached, ok := c.proteinByID[memoryKey]; ok && strings.TrimSpace(cached.Sequence) != "" {
		c.mu.RUnlock()
		return cached, true
	}
	c.mu.RUnlock()
	if cached, ok := readCachedJSON[proteinRecord]("protein-records", proteinRecordCacheKey(id)); ok && strings.TrimSpace(cached.Sequence) != "" {
		c.storeProteinRecord(cached)
		return cached, true
	}
	return proteinRecord{}, false
}

func (c *Client) storeProteinRecord(record proteinRecord) {
	keys := []string{record.UID, record.Accession}
	c.mu.Lock()
	for _, key := range keys {
		key = strings.ToUpper(strings.TrimSpace(key))
		if key != "" {
			c.proteinByID[key] = record
		}
	}
	if strings.TrimSpace(record.Sequence) != "" && strings.TrimSpace(record.Accession) != "" {
		c.sequenceCache[strings.ToUpper(record.Accession)] = model.ProteinSequenceData{
			Sequence:       strings.TrimSpace(record.Sequence),
			OriginalHeader: strings.TrimSpace(record.FastaHeader),
		}
	}
	c.mu.Unlock()
	for _, key := range keys {
		key = proteinRecordCacheKey(key)
		if key != "" {
			writeCachedJSON("protein-records", key, record)
		}
	}
}

func proteinRecordCacheKey(id string) string {
	id = strings.ToUpper(strings.TrimSpace(id))
	if id == "" {
		return ""
	}
	return proteinRecordSchema + "|" + id
}

func (c *Client) fetchProteinRecordsRemote(ctx context.Context, ids []string) ([]proteinRecord, error) {
	summaries, err := c.fetchProteinSummaries(ctx, ids)
	if err != nil {
		return nil, err
	}
	fastaText, err := c.fetchTextCached(ctx, "fasta", ids, url.Values{"db": {"protein"}, "rettype": {"fasta"}, "retmode": {"text"}})
	if err != nil {
		return nil, err
	}
	gbText, err := c.fetchTextCached(ctx, "genpept", ids, url.Values{"db": {"protein"}, "rettype": {"gb"}, "retmode": {"text"}})
	if err != nil {
		return nil, err
	}
	fastaByAccession := parseFastaRecords(fastaText)
	gbByAccession := parseGenPeptRecords(gbText)

	geneIDs := make([]string, 0, len(ids))
	for _, gb := range gbByAccession {
		if strings.TrimSpace(gb.GeneID) != "" {
			geneIDs = append(geneIDs, gb.GeneID)
		}
	}
	geneSummaries := map[string]geneSummary{}
	if len(geneIDs) > 0 {
		geneSummaries, _ = c.fetchGeneSummaries(ctx, geneIDs)
	}

	out := make([]proteinRecord, 0, len(summaries))
	for _, summary := range summaries {
		record := proteinRecord{
			UID:        summary.UID,
			Accession:  firstNonEmpty(summary.AccessionVersion, accessionFromExtra(summary.Extra), summary.Caption),
			Title:      strings.TrimSpace(summary.Title),
			Organism:   strings.TrimSpace(summary.Organism),
			TaxID:      summary.TaxID,
			Length:     summary.Length,
			SourceDB:   strings.TrimSpace(summary.SourceDB),
			Status:     firstNonEmpty(summary.Status, replacedStatus(summary.ReplacedBy)),
			ReplacedBy: strings.TrimSpace(summary.ReplacedBy),
			CreatedAt:  strings.TrimSpace(summary.CreateDate),
			UpdatedAt:  strings.TrimSpace(summary.UpdateDate),
			Definition: strings.TrimSpace(summary.Title),
		}
		fasta := fastaByAccession[normalizeAccessionKey(record.Accession)]
		if fasta.Header == "" {
			fasta = fastaByAccession[normalizeAccessionKey(summary.Caption)]
		}
		record.FastaHeader = fasta.Header
		record.Sequence = fasta.Sequence
		record.RawFasta = fasta.Raw
		if gb := gbByAccession[normalizeAccessionKey(record.Accession)]; gb.Accession != "" {
			record.Definition = firstNonEmpty(gb.Definition, record.Definition)
			record.GeneID = gb.GeneID
			record.GeneName = gb.GeneName
			record.LocusTag = gb.LocusTag
			record.Product = gb.Product
			record.CodedBy = gb.CodedBy
			record.RawGenPept = gb.Raw
		}
		if gs, ok := geneSummaries[record.GeneID]; ok {
			record.GeneSummary = gs
		}
		record.LabelAliases = ncbiLabelAliases(record)
		record.LocusAliases = ncbiLocusAliases(record)
		record.GeneLocus = chooseGeneLocus(record)
		c.storeProteinRecord(record)
		out = append(out, record)
	}
	return out, nil
}

func (c *Client) fetchProteinSummaries(ctx context.Context, ids []string) ([]eSummaryProteinDoc, error) {
	key := strings.Join(append([]string{"protein-summary"}, ids...), "|")
	if cached, ok := readCachedJSON[[]eSummaryProteinDoc]("summaries", key); ok {
		return cached, nil
	}
	values := url.Values{}
	values.Set("db", "protein")
	values.Set("id", strings.Join(ids, ","))
	values.Set("retmode", "json")
	var payload eSummaryProteinResponse
	if err := c.getJSON(ctx, "esummary.fcgi", values, &payload); err != nil {
		return nil, err
	}
	out := make([]eSummaryProteinDoc, 0, len(ids))
	for _, id := range ids {
		raw := payload.Result[id]
		if len(raw) == 0 {
			continue
		}
		var doc eSummaryProteinDoc
		if err := json.Unmarshal(raw, &doc); err != nil {
			continue
		}
		if strings.TrimSpace(doc.UID) == "" {
			doc.UID = id
		}
		out = append(out, doc)
	}
	if len(out) == 0 {
		var uids []string
		_ = json.Unmarshal(payload.Result["uids"], &uids)
		for _, uid := range uids {
			raw := payload.Result[uid]
			if len(raw) == 0 {
				continue
			}
			var doc eSummaryProteinDoc
			if err := json.Unmarshal(raw, &doc); err != nil {
				continue
			}
			if strings.TrimSpace(doc.UID) == "" {
				doc.UID = uid
			}
			out = append(out, doc)
		}
	}
	writeCachedJSON("summaries", key, out)
	return out, nil
}

func (c *Client) fetchGeneSummaries(ctx context.Context, ids []string) (map[string]geneSummary, error) {
	ids = uniqueNonEmpty(ids)
	key := strings.Join(append([]string{"gene-summary"}, ids...), "|")
	if cached, ok := readCachedJSON[map[string]geneSummary]("summaries", key); ok {
		return cached, nil
	}
	values := url.Values{}
	values.Set("db", "gene")
	values.Set("id", strings.Join(ids, ","))
	values.Set("retmode", "json")
	var payload eSummaryGeneResponse
	if err := c.getJSON(ctx, "esummary.fcgi", values, &payload); err != nil {
		return nil, err
	}
	out := make(map[string]geneSummary, len(ids))
	for _, id := range ids {
		raw := payload.Result[id]
		if len(raw) == 0 {
			continue
		}
		var doc eSummaryGeneDoc
		if err := json.Unmarshal(raw, &doc); err != nil {
			continue
		}
		out[id] = geneSummary{
			UID:               firstNonEmpty(doc.UID, id),
			Name:              strings.TrimSpace(doc.Name),
			Description:       strings.TrimSpace(doc.Description),
			OtherAliases:      strings.TrimSpace(doc.OtherAliases),
			OtherDesignations: strings.TrimSpace(doc.OtherDesignations),
			Nomenclature:      strings.TrimSpace(doc.Nomenclature),
			Chromosome:        strings.TrimSpace(doc.Chromosome),
			MapLocation:       strings.TrimSpace(doc.MapLocation),
			Summary:           strings.TrimSpace(doc.Summary),
			Organism:          strings.TrimSpace(doc.Organism.ScientificName),
			TaxID:             doc.Organism.TaxID,
		}
	}
	writeCachedJSON("summaries", key, out)
	return out, nil
}

func (c *Client) fetchTextCached(ctx context.Context, group string, ids []string, values url.Values) (string, error) {
	key := strings.Join(append([]string{group}, ids...), "|")
	if cached, ok := readCachedText(group, key); ok && strings.TrimSpace(cached) != "" {
		return cached, nil
	}
	values = cloneValues(values)
	values.Set("id", strings.Join(ids, ","))
	body, err := c.getText(ctx, "efetch.fcgi", values)
	if err != nil {
		return "", err
	}
	writeCachedText(group, key, body)
	return body, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, values url.Values, target any) error {
	body, err := c.getText(ctx, endpoint, values)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(body), target); err != nil {
		return fmt.Errorf("decode NCBI %s response: %w", endpoint, err)
	}
	return nil
}

func (c *Client) getText(ctx context.Context, endpoint string, values url.Values) (string, error) {
	values = cloneValues(values)
	values.Set("tool", "phytozome-go")
	if email := strings.TrimSpace(os.Getenv("NCBI_EMAIL")); email != "" {
		values.Set("email", email)
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 900 * time.Millisecond)
		}
		requestValues := cloneValues(values)
		apiKey := c.effectiveAPIKey()
		if apiKey != "" {
			requestValues.Set("api_key", apiKey)
		} else {
			requestValues.Del("api_key")
		}
		requestURL := eutilsBaseURL + "/" + endpoint + "?" + requestValues.Encode()
		if err := throttleNCBI(apiKey != ""); err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return "", fmt.Errorf("create NCBI request: %w", err)
		}
		req.Header.Set("User-Agent", "phytozome-go/NCBI-E-utilities")
		resp, err := c.baseHTTP.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fetch NCBI %s: %w", endpoint, err)
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxNCBITextResponse+1))
		closeErr := resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}
		if len(data) > maxNCBITextResponse {
			return "", fmt.Errorf("NCBI %s response exceeds %d bytes", endpoint, maxNCBITextResponse)
		}
		body := string(data)
		if resp.StatusCode == http.StatusOK {
			return body, nil
		}
		trimmed := strings.TrimSpace(body)
		lastErr = fmt.Errorf("fetch NCBI %s: status %s body %s", endpoint, resp.Status, trimmed)
		if apiKey != "" && isInvalidNCBIAPIKeyResponse(resp.StatusCode, trimmed) {
			c.disableAPIKey()
			continue
		}
		if !isRetryableNCBIResponse(resp.StatusCode, trimmed) {
			return "", lastErr
		}
	}
	return "", lastErr
}

func ncbiAPIKey() string {
	if value := strings.TrimSpace(os.Getenv("NCBI_API_KEY")); value != "" {
		return strings.ToLower(value)
	}
	return strings.ToLower(defaultNCBIAPIKey)
}

func (c *Client) effectiveAPIKey() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.apiKeyInvalid {
		return ""
	}
	return strings.TrimSpace(c.apiKey)
}

func (c *Client) disableAPIKey() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.apiKeyInvalid = true
	c.mu.Unlock()
}

func isInvalidNCBIAPIKeyResponse(statusCode int, body string) bool {
	body = strings.ToLower(strings.TrimSpace(body))
	if !strings.Contains(body, "api") || !strings.Contains(body, "key") {
		return false
	}
	return statusCode == http.StatusBadRequest ||
		statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusForbidden ||
		strings.Contains(body, "invalid") ||
		strings.Contains(body, "not found") ||
		strings.Contains(body, "revoked") ||
		strings.Contains(body, "expired")
}

func isRetryableNCBIResponse(statusCode int, body string) bool {
	if statusCode == http.StatusTooManyRequests || statusCode >= 500 {
		return true
	}
	body = strings.ToLower(strings.TrimSpace(body))
	return strings.Contains(body, "rate limit") ||
		strings.Contains(body, "too many requests") ||
		strings.Contains(body, "temporarily unavailable")
}

func throttleNCBI(hasAPIKey bool) error {
	delay := 350 * time.Millisecond
	if hasAPIKey {
		delay = 100 * time.Millisecond
	}
	ncbiThrottleMu.Lock()
	defer ncbiThrottleMu.Unlock()
	if !ncbiLastRequest.IsZero() {
		wait := delay - time.Since(ncbiLastRequest)
		if wait > 0 {
			time.Sleep(wait)
		}
	}
	ncbiLastRequest = time.Now()
	return nil
}

type fastaRecord struct {
	Accession string
	Header    string
	Sequence  string
	Raw       string
}

func parseFastaRecords(text string) map[string]fastaRecord {
	out := map[string]fastaRecord{}
	var current fastaRecord
	var seq strings.Builder
	var raw strings.Builder
	flush := func() {
		if current.Header == "" {
			return
		}
		current.Sequence = strings.TrimSpace(seq.String())
		current.Raw = strings.TrimSpace(raw.String())
		out[normalizeAccessionKey(current.Accession)] = current
	}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		if strings.HasPrefix(line, ">") {
			flush()
			header := strings.TrimSpace(line)
			fields := strings.Fields(strings.TrimPrefix(header, ">"))
			accession := ""
			if len(fields) > 0 {
				accession = fields[0]
			}
			current = fastaRecord{Accession: accession, Header: header}
			seq.Reset()
			raw.Reset()
			raw.WriteString(header)
			raw.WriteString("\n")
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		seq.WriteString(trimmed)
		raw.WriteString(trimmed)
		raw.WriteString("\n")
	}
	flush()
	return out
}

type genPeptRecord struct {
	Accession  string
	Definition string
	GeneID     string
	GeneName   string
	LocusTag   string
	Product    string
	CodedBy    string
	Raw        string
}

type nucleotideCDSRecord struct {
	NucleotideAccession string
	Definition          string
	Organism            string
	GeneID              string
	GeneName            string
	LocusTag            string
	Product             string
	ProteinID           string
	ProteinSequence     string
	CDSIndex            int
	Raw                 string
}

func parseGenPeptRecords(text string) map[string]genPeptRecord {
	out := map[string]genPeptRecord{}
	normalized := strings.ReplaceAll(text, "\r", "")
	for _, rawRecord := range strings.Split(normalized, "\n//") {
		rawRecord = strings.TrimSpace(rawRecord)
		if rawRecord == "" {
			continue
		}
		record := genPeptRecord{Raw: rawRecord + "\n//"}
		record.Accession = parseGenPeptVersion(rawRecord)
		record.Definition = parseGenPeptDefinition(rawRecord)
		if m := geneIDPattern.FindStringSubmatch(rawRecord); len(m) > 1 {
			record.GeneID = strings.TrimSpace(m[1])
		}
		for _, line := range strings.Split(rawRecord, "\n") {
			line = strings.TrimSpace(line)
			m := qualifierPattern.FindStringSubmatch(line)
			if len(m) < 3 {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(m[1]))
			value := strings.Trim(strings.TrimSpace(m[2]), `"`)
			switch key {
			case "gene":
				record.GeneName = firstNonEmpty(record.GeneName, value)
			case "locus_tag":
				record.LocusTag = firstNonEmpty(record.LocusTag, value)
			case "product":
				record.Product = firstNonEmpty(record.Product, value)
			case "coded_by":
				record.CodedBy = firstNonEmpty(record.CodedBy, value)
			}
		}
		if record.Accession != "" {
			out[normalizeAccessionKey(record.Accession)] = record
		}
	}
	return out
}

func parseGenPeptVersion(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "VERSION") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ACCESSION") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return ""
}

func parseGenPeptDefinition(raw string) string {
	start := strings.Index(raw, "DEFINITION")
	if start < 0 {
		return ""
	}
	rest := raw[start+len("DEFINITION"):]
	end := strings.Index(rest, "\nACCESSION")
	if end >= 0 {
		rest = rest[:end]
	}
	return strings.Join(strings.Fields(rest), " ")
}

func parseNucleotideCDSRecords(text string) []nucleotideCDSRecord {
	out := []nucleotideCDSRecord{}
	normalized := strings.ReplaceAll(text, "\r", "")
	for _, rawRecord := range strings.Split(normalized, "\n//") {
		rawRecord = strings.TrimSpace(rawRecord)
		if rawRecord == "" {
			continue
		}
		nucleotideAccession := parseGenPeptVersion(rawRecord)
		if nucleotideAccession == "" {
			continue
		}
		definition := parseGenPeptDefinition(rawRecord)
		organism := parseSourceOrganism(rawRecord)
		cdsBlocks := genbankFeatureBlocks(rawRecord, "CDS")
		for i, block := range cdsBlocks {
			qualifiers := parseGenBankQualifiers(block)
			translation := sanitizeProteinSequence(firstQualifier(qualifiers, "translation"))
			proteinID := firstQualifier(qualifiers, "protein_id")
			if translation == "" && proteinID == "" {
				continue
			}
			record := nucleotideCDSRecord{
				NucleotideAccession: nucleotideAccession,
				Definition:          definition,
				Organism:            organism,
				GeneID:              geneIDFromQualifiers(qualifiers),
				GeneName:            firstQualifier(qualifiers, "gene"),
				LocusTag:            firstQualifier(qualifiers, "locus_tag"),
				Product:             firstQualifier(qualifiers, "product"),
				ProteinID:           proteinID,
				ProteinSequence:     translation,
				CDSIndex:            i + 1,
				Raw:                 strings.TrimSpace(block),
			}
			out = append(out, record)
		}
	}
	return out
}

func parseSourceOrganism(raw string) string {
	for _, block := range genbankFeatureBlocks(raw, "source") {
		qualifiers := parseGenBankQualifiers(block)
		if organism := firstQualifier(qualifiers, "organism"); organism != "" {
			return organism
		}
	}
	return ""
}

func genbankFeatureBlocks(raw string, feature string) []string {
	feature = strings.ToLower(strings.TrimSpace(feature))
	if feature == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	inFeatures := false
	currentFeature := ""
	current := []string{}
	out := []string{}
	flush := func() {
		if strings.EqualFold(currentFeature, feature) && len(current) > 0 {
			out = append(out, strings.Join(current, "\n"))
		}
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "FEATURES") {
			inFeatures = true
			continue
		}
		if !inFeatures {
			continue
		}
		if strings.HasPrefix(line, "ORIGIN") {
			flush()
			break
		}
		if key, ok := genbankFeatureKey(line); ok {
			flush()
			currentFeature = key
			current = []string{line}
			continue
		}
		if currentFeature != "" {
			current = append(current, line)
		}
	}
	return out
}

func genbankFeatureKey(line string) (string, bool) {
	if len(line) < 21 || !strings.HasPrefix(line, "     ") {
		return "", false
	}
	key := strings.TrimSpace(line[5:21])
	if key == "" || strings.HasPrefix(key, "/") || strings.ContainsAny(key, " \t") {
		return "", false
	}
	return key, true
}

func parseGenBankQualifiers(block string) map[string][]string {
	qualifiers := map[string][]string{}
	key := ""
	var value strings.Builder
	flush := func() {
		if key == "" {
			return
		}
		cleaned := strings.TrimSpace(strings.Trim(value.String(), `"`))
		if cleaned != "" {
			qualifiers[key] = append(qualifiers[key], cleaned)
		}
		key = ""
		value.Reset()
	}
	appendPart := func(part string) {
		part = strings.TrimSpace(strings.Trim(part, `"`))
		if part == "" {
			return
		}
		if key != "translation" && value.Len() > 0 {
			value.WriteString(" ")
		}
		value.WriteString(part)
	}
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "/") {
			flush()
			trimmed = strings.TrimPrefix(trimmed, "/")
			if idx := strings.Index(trimmed, "="); idx >= 0 {
				key = strings.ToLower(strings.TrimSpace(trimmed[:idx]))
				appendPart(trimmed[idx+1:])
			} else {
				key = strings.ToLower(strings.TrimSpace(trimmed))
			}
			continue
		}
		if key != "" {
			appendPart(trimmed)
		}
	}
	flush()
	return qualifiers
}

func firstQualifier(qualifiers map[string][]string, key string) string {
	values := qualifiers[strings.ToLower(strings.TrimSpace(key))]
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func geneIDFromQualifiers(qualifiers map[string][]string) string {
	for _, value := range qualifiers["db_xref"] {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(strings.ToLower(value), "geneid:") {
			return strings.TrimSpace(value[len("GeneID:"):])
		}
	}
	return ""
}

func sanitizeProteinSequence(sequence string) string {
	sequence = strings.TrimSpace(sequence)
	if sequence == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(sequence))
	for _, ch := range sequence {
		switch {
		case ch >= 'A' && ch <= 'Z':
			b.WriteRune(ch)
		case ch >= 'a' && ch <= 'z':
			b.WriteRune(ch - ('a' - 'A'))
		case ch == '*' || ch == '-' || ch == '.':
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func keywordRowFromProteinRecord(searchTerm string, record proteinRecord) model.KeywordResultRow {
	accession := strings.TrimSpace(record.Accession)
	aliases := strings.Join(record.LabelAliases, "; ")
	extra := map[string]string{
		"ncbi_entrez_database":     "protein",
		"ncbi_record_type":         "protein",
		"ncbi_eutilities_base_url": eutilsBaseURL,
		"ncbi_engine_schema":       "ncbiprotein-v3",
		"ncbi_uid":                 record.UID,
		"ncbi_accession":           accession,
		"ncbi_taxid":               intString(record.TaxID),
		"ncbi_source_db":           record.SourceDB,
		"ncbi_status":              record.Status,
		"ncbi_replaced_by":         record.ReplacedBy,
		"ncbi_created":             record.CreatedAt,
		"ncbi_updated":             record.UpdatedAt,
		"ncbi_length":              intString(record.Length),
		"ncbi_gene_id":             record.GeneID,
		"ncbi_gene_name":           record.GeneName,
		"ncbi_locus_tag":           record.LocusTag,
		"ncbi_product":             record.Product,
		"ncbi_coded_by":            record.CodedBy,
		"ncbi_gene_description":    record.GeneSummary.Description,
		"ncbi_other_aliases":       record.GeneSummary.OtherAliases,
		"ncbi_other_designations":  record.GeneSummary.OtherDesignations,
		"ncbi_gene_locus_aliases":  strings.Join(record.LocusAliases, "; "),
		"ncbi_fasta_header":        record.FastaHeader,
		"ncbi_protein_sequence":    record.Sequence,
		"ncbi_fasta":               record.RawFasta,
	}
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          searchTerm,
		ProteinID:           accession,
		GeneLocus:           record.GeneLocus,
		GeneIdentifier:      firstNonEmpty(record.GeneSummary.Name, record.GeneName, record.GeneID),
		Genome:              record.Organism,
		Location:            formatNCBIGeneLocation(record),
		Aliases:             aliases,
		Symbols:             firstNonEmpty(record.GeneSummary.Nomenclature, record.GeneName),
		Synonyms:            record.GeneSummary.OtherAliases,
		Description:         firstNonEmpty(record.Product, record.Definition, record.Title),
		Comments:            record.GeneSummary.Summary,
		AutoDefine:          record.GeneSummary.OtherDesignations,
		SequenceHeaderLabel: record.Organism,
		SequenceID:          accession,
		ExtraColumns:        extra,
	}
}

func keywordRowFromNucleotideCDSRecord(searchTerm string, record nucleotideCDSRecord) model.KeywordResultRow {
	sequenceID := strings.TrimSpace(record.ProteinID)
	if sequenceID == "" {
		sequenceID = fmt.Sprintf("%s:CDS%d", strings.TrimSpace(record.NucleotideAccession), record.CDSIndex)
	}
	header := ">" + sequenceID
	if product := strings.TrimSpace(firstNonEmpty(record.Product, record.Definition)); product != "" {
		header += " " + product
	}
	extra := map[string]string{
		"ncbi_entrez_database":      "nuccore",
		"ncbi_record_type":          "nucleotide CDS translation",
		"ncbi_eutilities_base_url":  eutilsBaseURL,
		"ncbi_engine_schema":        "ncbiprotein-v4",
		"ncbi_accession":            sequenceID,
		"ncbi_nucleotide_accession": record.NucleotideAccession,
		"ncbi_protein_id":           record.ProteinID,
		"ncbi_gene_id":              record.GeneID,
		"ncbi_gene_name":            record.GeneName,
		"ncbi_locus_tag":            record.LocusTag,
		"ncbi_product":              record.Product,
		"ncbi_fallback_source":      "nuccore CDS translation",
		"ncbi_fasta_header":         header,
		"ncbi_protein_sequence":     record.ProteinSequence,
		"ncbi_fasta":                header + "\n" + record.ProteinSequence,
	}
	return model.KeywordResultRow{
		SourceDatabase:      "ncbi",
		SearchTerm:          searchTerm,
		SearchType:          ncbiprotein.SearchTypeNucleotideFallback,
		ProteinID:           sequenceID,
		TranscriptID:        strings.TrimSpace(record.NucleotideAccession),
		GeneLocus:           firstNonEmpty(record.LocusTag, record.GeneName),
		GeneIdentifier:      firstNonEmpty(record.GeneName, record.GeneID),
		Genome:              record.Organism,
		Location:            "nuccore " + strings.TrimSpace(record.NucleotideAccession),
		Symbols:             record.GeneName,
		Description:         firstNonEmpty(record.Product, record.Definition),
		AutoDefine:          firstNonEmpty(record.Product, record.Definition),
		SequenceHeaderLabel: record.Organism,
		SequenceID:          sequenceID,
		ExtraColumns:        extra,
	}
}

func formatNCBIGeneLocation(record proteinRecord) string {
	parts := make([]string, 0, 3)
	if chr := strings.TrimSpace(record.GeneSummary.Chromosome); chr != "" {
		parts = append(parts, "chromosome "+chr)
	}
	if loc := strings.TrimSpace(record.GeneSummary.MapLocation); loc != "" {
		parts = append(parts, loc)
	}
	if codedBy := strings.TrimSpace(record.CodedBy); codedBy != "" {
		parts = append(parts, "coded_by "+codedBy)
	}
	return strings.Join(parts, "; ")
}

func ncbiLabelAliases(record proteinRecord) []string {
	aliases := make([]string, 0, 16)
	aliases = append(aliases, splitAliasList(record.GeneSummary.Nomenclature)...)
	aliases = append(aliases, splitAliasList(record.GeneSummary.OtherAliases)...)
	aliases = append(aliases, splitAliasList(record.GeneName)...)
	return uniqueNonEmpty(aliases)
}

func ncbiLocusAliases(record proteinRecord) []string {
	values := make([]string, 0, 16)
	values = append(values, splitDesignationList(record.GeneSummary.OtherDesignations)...)
	values = append(values, splitAliasList(record.GeneSummary.OtherAliases)...)
	values = append(values, record.LocusTag, record.GeneName, record.GeneSummary.Name)
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || locNumberPattern.MatchString(value) {
			continue
		}
		if locusLikePattern.MatchString(value) {
			value = strings.TrimPrefix(value, "LOC_")
			out = append(out, value)
		}
	}
	return uniqueNonEmpty(out)
}

func chooseGeneLocus(record proteinRecord) string {
	for _, value := range record.LocusAliases {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	for _, value := range []string{record.LocusTag, record.GeneName, record.GeneSummary.Name} {
		value = strings.TrimSpace(value)
		if value != "" && !locNumberPattern.MatchString(value) {
			return value
		}
	}
	return ""
}

func splitAliasList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ';', ',', '|', '\t', '\n', '\r':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func splitDesignationList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '|' || r == ';' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func accessionFromExtra(value string) string {
	parts := strings.Split(value, "|")
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(strings.TrimSpace(parts[i]), "ref") || strings.EqualFold(strings.TrimSpace(parts[i]), "gb") || strings.EqualFold(strings.TrimSpace(parts[i]), "emb") || strings.EqualFold(strings.TrimSpace(parts[i]), "dbj") {
			return strings.TrimSpace(parts[i+1])
		}
	}
	return ""
}

func replacedStatus(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return "replaced by " + value
}

func normalizeAccessionKey(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	return strings.TrimPrefix(value, ">")
}

func cloneValues(values url.Values) url.Values {
	out := make(url.Values, len(values))
	for key, list := range values {
		out[key] = append([]string(nil), list...)
	}
	return out
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func intString(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
