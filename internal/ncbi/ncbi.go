package ncbi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
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
	return []model.SpeciesCandidate{{
		ProteomeID:  0,
		JBrowseName: "ncbi_protein",
		GenomeLabel: "NCBI Protein",
		CommonName:  "global protein database",
		SearchAlias: "NCBI Protein E-utilities",
	}}, nil
}

func (c *Client) SearchKeywordRows(ctx context.Context, species model.SpeciesCandidate, keyword string) ([]model.KeywordResultRow, error) {
	if c.keywordEngine == nil {
		c.keywordEngine = ncbiprotein.New(c)
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
	return c.searchProteinRowsFromNucleotideDB(ctx, term, limit)
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
		SourceJBrowseName:   "ncbi_protein",
		SourceGenomeLabel:   "NCBI Protein",
		ProteinID:           strings.TrimSpace(identifier),
		Annotation:          strings.TrimPrefix(strings.TrimSpace(data.OriginalHeader), ">"),
	}, nil
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
		return value
	}
	return defaultNCBIAPIKey
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
		GeneReportURL:       "https://www.ncbi.nlm.nih.gov/protein/" + url.PathEscape(accession),
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
		GeneReportURL:       "https://www.ncbi.nlm.nih.gov/nuccore/" + url.PathEscape(record.NucleotideAccession),
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
