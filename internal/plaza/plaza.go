// Package plaza resolves PLAZA gene-locus records from its public, versioned
// data releases. It deliberately avoids the Cloudflare-protected web UI.
package plaza

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/KiriKirby/phytozome-go/internal/appfs"
	"github.com/KiriKirby/phytozome-go/internal/model"
	"github.com/KiriKirby/phytozome-go/internal/netconfig"
	"golang.org/x/sync/singleflight"
)

const (
	searchTypeGeneLocusPriority = "PLAZA Gene locus priority"
	maxPLAZADownloadBytes       = 512 << 20
)

type instance struct {
	Name string
	Base string
}

type speciesInfo struct {
	ID         string
	CommonName string
	TaxID      string
	Source     string
	Instance   instance
}

type geneCandidate struct {
	Species speciesInfo
	GeneID  string
}

// Client is safe for concurrent use. Data files are cached under .cache/plaza
// so repeat batch searches do not repeat the release downloads.
type Client struct {
	httpClient *http.Client
	instances  []instance

	mu        sync.RWMutex
	species   map[string]speciesInfo
	geneIndex map[string][]geneCandidate
	lookup    map[string][]model.KeywordResultRow
	sf        singleflight.Group
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = netconfig.DefaultHTTPClient()
	}
	return &Client{
		httpClient: httpClient,
		instances: []instance{
			{Name: "dicots_05", Base: "https://ftp.psb.ugent.be/pub/plaza/plaza_public_dicots_05"},
			{Name: "monocots_05", Base: "https://ftp.psb.ugent.be/pub/plaza/plaza_public_monocots_05"},
			{Name: "diatoms_01", Base: "https://ftp.psb.ugent.be/pub/plaza/plaza_diatoms_01"},
			{Name: "pico_03", Base: "https://ftp.psb.ugent.be/pub/plaza/plaza_pico_03"},
		},
		species:   make(map[string]speciesInfo),
		geneIndex: make(map[string][]geneCandidate),
		lookup:    make(map[string][]model.KeywordResultRow),
	}
}

// SearchGeneLocus returns PLAZA rows whose canonical gene identifier equals
// locus. A no-result lookup is normal and lets the caller use its NCBI path.
func (c *Client) SearchGeneLocus(ctx context.Context, locus string) ([]model.KeywordResultRow, error) {
	locus = strings.TrimSpace(locus)
	if locus == "" {
		return nil, nil
	}
	c.mu.RLock()
	if rows, ok := c.lookup[strings.ToLower(locus)]; ok {
		c.mu.RUnlock()
		return cloneRows(rows), nil
	}
	c.mu.RUnlock()
	if rows, ok := c.readLookupCache(locus); ok {
		c.mu.Lock()
		c.lookup[strings.ToLower(locus)] = cloneRows(rows)
		c.mu.Unlock()
		return rows, nil
	}

	value, err, _ := c.sf.Do("lookup:"+strings.ToLower(locus), func() (any, error) {
		if err := c.ensureGeneIndex(ctx); err != nil {
			return nil, err
		}
		c.mu.RLock()
		candidates := append([]geneCandidate(nil), c.geneIndex[strings.ToLower(locus)]...)
		c.mu.RUnlock()
		rows := make([]model.KeywordResultRow, 0, len(candidates))
		for _, candidate := range candidates {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			row, err := c.buildRow(ctx, candidate, locus)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
		sort.SliceStable(rows, func(i, j int) bool {
			return strings.Compare(rows[i].Genome, rows[j].Genome) < 0
		})
		c.mu.Lock()
		c.lookup[strings.ToLower(locus)] = cloneRows(rows)
		c.mu.Unlock()
		c.writeLookupCache(locus, rows)
		return rows, nil
	})
	if err != nil {
		return nil, err
	}
	return cloneRows(value.([]model.KeywordResultRow)), nil
}

func (c *Client) ensureGeneIndex(ctx context.Context) error {
	c.mu.RLock()
	ready := len(c.geneIndex) > 0
	c.mu.RUnlock()
	if ready {
		return nil
	}
	_, err, _ := c.sf.Do("gene-index", func() (any, error) {
		c.mu.RLock()
		if len(c.geneIndex) > 0 {
			c.mu.RUnlock()
			return nil, nil
		}
		c.mu.RUnlock()

		type instanceData struct {
			species map[string]speciesInfo
			genes   map[string][]geneCandidate
			err     error
		}
		results := make(chan instanceData, len(c.instances))
		var wg sync.WaitGroup
		for _, spec := range c.instances {
			spec := spec
			wg.Add(1)
			go func() {
				defer wg.Done()
				species, genes, err := c.loadInstanceIndex(ctx, spec)
				results <- instanceData{species: species, genes: genes, err: err}
			}()
		}
		wg.Wait()
		close(results)
		mergedSpecies := make(map[string]speciesInfo)
		mergedGenes := make(map[string][]geneCandidate)
		for result := range results {
			if result.err != nil {
				return nil, result.err
			}
			for id, info := range result.species {
				mergedSpecies[speciesKey(info.Instance.Name, id)] = info
			}
			for locus, candidates := range result.genes {
				mergedGenes[locus] = append(mergedGenes[locus], candidates...)
			}
		}
		c.mu.Lock()
		c.species = mergedSpecies
		c.geneIndex = mergedGenes
		c.mu.Unlock()
		return nil, nil
	})
	return err
}

func (c *Client) loadInstanceIndex(ctx context.Context, spec instance) (map[string]speciesInfo, map[string][]geneCandidate, error) {
	speciesText, err := c.readGzipText(ctx, spec, "SpeciesInformation/species_information.csv.gz")
	if err != nil {
		return nil, nil, err
	}
	species := parseSpeciesInformation(speciesText, spec)
	genesText, err := c.readGzipText(ctx, spec, "GeneFamilies/genefamily_data.HOMFAM.csv.gz")
	if err != nil {
		return nil, nil, err
	}
	genes := make(map[string][]geneCandidate)
	for _, fields := range tabularRows(genesText, spec) {
		if len(fields) < 3 {
			continue
		}
		info, ok := species[fields[1]]
		if !ok {
			continue
		}
		geneID := strings.TrimSpace(fields[2])
		if geneID == "" {
			continue
		}
		key := strings.ToLower(geneID)
		genes[key] = append(genes[key], geneCandidate{Species: info, GeneID: geneID})
	}
	return species, genes, nil
}

func (c *Client) buildRow(ctx context.Context, candidate geneCandidate, requestedLocus string) (model.KeywordResultRow, error) {
	base := candidate.Species.Instance.Base
	idConversionPath := "IdConversion/id_conversion." + url.PathEscape(candidate.Species.ID) + ".csv.gz"
	idText, err := c.readGzipText(ctx, candidate.Species.Instance, idConversionPath)
	if err != nil {
		return model.KeywordResultRow{}, err
	}
	aliases, symbols, transcriptID, uniprot := parseIDConversion(idText, candidate.Species.Instance, candidate.GeneID)
	if transcriptID == "" {
		transcriptID = candidate.GeneID
	}
	descriptionPath := "Descriptions/gene_description." + url.PathEscape(candidate.Species.ID) + ".csv.gz"
	descriptionText, err := c.readGzipText(ctx, candidate.Species.Instance, descriptionPath)
	if err != nil {
		return model.KeywordResultRow{}, err
	}
	description := parseDescription(descriptionText, candidate.Species.Instance, candidate.GeneID)
	fastaPath := "Fasta/proteome.selected_transcript." + url.PathEscape(candidate.Species.ID) + ".fasta.gz"
	fastaText, err := c.readGzipText(ctx, candidate.Species.Instance, fastaPath)
	if err != nil {
		return model.KeywordResultRow{}, err
	}
	fastaHeader, sequence := findFASTARecord(fastaText, transcriptID, candidate.GeneID)
	extras := map[string]string{
		"plaza_instance":          candidate.Species.Instance.Name,
		"plaza_species_id":        candidate.Species.ID,
		"plaza_taxid":             candidate.Species.TaxID,
		"plaza_source":            candidate.Species.Source,
		"plaza_gene_id":           candidate.GeneID,
		"plaza_requested_locus":   requestedLocus,
		"plaza_id_conversion_url": joinURL(base, idConversionPath),
		"plaza_description_url":   joinURL(base, descriptionPath),
		"plaza_proteome_url":      joinURL(base, fastaPath),
	}
	if fastaHeader != "" {
		extras["plaza_fasta_header"] = fastaHeader
	}
	if sequence != "" {
		extras["plaza_protein_sequence"] = sequence
		extras["plaza_fasta"] = fastaHeader + "\n" + sequence
	}
	return model.KeywordResultRow{
		SourceDatabase:      "plaza",
		SearchType:          searchTypeGeneLocusPriority,
		ProteinID:           transcriptID,
		TranscriptID:        transcriptID,
		GeneLocus:           candidate.GeneID,
		GeneIdentifier:      candidate.GeneID,
		Genome:              candidate.Species.CommonName,
		Aliases:             strings.Join(aliases, "; "),
		Symbols:             strings.Join(symbols, "; "),
		UniProt:             uniprot,
		Description:         description,
		SequenceHeaderLabel: candidate.Species.CommonName,
		SequenceID:          transcriptID,
		ExtraColumns:        extras,
	}, nil
}

func (c *Client) readGzipText(ctx context.Context, spec instance, relativePath string) (string, error) {
	resourceURL := joinURL(spec.Base, relativePath)
	cachePath, err := plazaCachePath("release", resourceURL, ".gz")
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		data, err = c.download(ctx, resourceURL)
		if err != nil {
			return "", err
		}
		_ = appfs.WriteFileAtomic(cachePath, data, 0o644)
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode PLAZA gzip %s: %w", resourceURL, err)
	}
	defer reader.Close()
	text, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read PLAZA gzip %s: %w", resourceURL, err)
	}
	return string(text), nil
}

func (c *Client) download(ctx context.Context, resourceURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create PLAZA request: %w", err)
	}
	req.Header.Set("User-Agent", "phytozome-go/PLAZA-public-data")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch PLAZA %s: %w", resourceURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch PLAZA %s: status %s", resourceURL, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPLAZADownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read PLAZA %s: %w", resourceURL, err)
	}
	if len(data) > maxPLAZADownloadBytes {
		return nil, fmt.Errorf("PLAZA response %s exceeds %d bytes", resourceURL, maxPLAZADownloadBytes)
	}
	return data, nil
}

func parseSpeciesInformation(text string, spec instance) map[string]speciesInfo {
	out := make(map[string]speciesInfo)
	for _, fields := range tabularRows(text, spec) {
		if len(fields) < 4 {
			continue
		}
		id := strings.TrimSpace(fields[0])
		if id == "" {
			continue
		}
		out[id] = speciesInfo{ID: id, CommonName: strings.TrimSpace(fields[1]), TaxID: strings.TrimSpace(fields[2]), Source: strings.TrimSpace(fields[3]), Instance: spec}
	}
	return out
}

func parseIDConversion(text string, spec instance, geneID string) (aliases []string, symbols []string, transcriptID string, uniprot string) {
	for _, fields := range tabularRows(text, spec) {
		if len(fields) < 3 || !strings.EqualFold(strings.TrimSpace(fields[0]), geneID) {
			continue
		}
		kind, value := strings.ToLower(strings.TrimSpace(fields[1])), strings.TrimSpace(fields[2])
		switch kind {
		case "alias":
			aliases = append(aliases, splitValues(value)...)
		case "symbol":
			symbols = append(symbols, splitValues(value)...)
		case "tid", "transcript", "transcript_id":
			if transcriptID == "" {
				transcriptID = firstValue(value)
			}
		case "uniprot", "uniprotkb":
			if uniprot == "" {
				uniprot = firstValue(value)
			}
		}
	}
	return uniqueStrings(aliases), uniqueStrings(symbols), transcriptID, uniprot
}

func parseDescription(text string, spec instance, geneID string) string {
	for _, fields := range tabularRows(text, spec) {
		if len(fields) >= 3 && strings.EqualFold(strings.TrimSpace(fields[0]), geneID) && strings.EqualFold(strings.TrimSpace(fields[1]), "description") {
			return strings.TrimSpace(fields[2])
		}
	}
	return ""
}

func findFASTARecord(text string, ids ...string) (string, string) {
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			wanted[id] = true
		}
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8<<20)
	header, sequence := "", strings.Builder{}
	matched := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, ">") {
			if matched {
				return header, sequence.String()
			}
			header, sequence, matched = line, strings.Builder{}, fastaHeaderMatches(line, wanted)
			continue
		}
		if matched {
			sequence.WriteString(line)
		}
	}
	if matched {
		return header, sequence.String()
	}
	return "", ""
}

func fastaHeaderMatches(header string, wanted map[string]bool) bool {
	header = strings.TrimSpace(strings.TrimPrefix(header, ">"))
	for _, field := range strings.FieldsFunc(header, func(r rune) bool { return r == ' ' || r == '|' || r == '\t' }) {
		if wanted[field] {
			return true
		}
	}
	return false
}

func tabularRows(text string, spec instance) [][]string {
	delimiter := "\t"
	if strings.Contains(strings.ToLower(spec.Name), "diatom") || strings.Contains(strings.ToLower(spec.Name), "pico") {
		delimiter = ";"
	}
	lines := strings.Split(text, "\n")
	out := make([][]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, strings.Split(line, delimiter))
	}
	return out
}

func splitValues(value string) []string {
	values := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' })
	return uniqueStrings(values)
}

func firstValue(value string) string {
	values := splitValues(value)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func cloneRows(rows []model.KeywordResultRow) []model.KeywordResultRow {
	out := append([]model.KeywordResultRow(nil), rows...)
	for i := range out {
		if rows[i].ExtraColumns == nil {
			continue
		}
		out[i].ExtraColumns = make(map[string]string, len(rows[i].ExtraColumns))
		for key, value := range rows[i].ExtraColumns {
			out[i].ExtraColumns[key] = value
		}
	}
	return out
}

func speciesKey(instanceName, speciesID string) string {
	return strings.ToLower(strings.TrimSpace(instanceName)) + ":" + strings.ToLower(strings.TrimSpace(speciesID))
}

func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func (c *Client) readLookupCache(locus string) ([]model.KeywordResultRow, bool) {
	path, err := plazaCachePath("lookup", c.lookupCacheKey(locus), ".json")
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var rows []model.KeywordResultRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, false
	}
	return cloneRows(rows), true
}

func (c *Client) writeLookupCache(locus string, rows []model.KeywordResultRow) {
	path, err := plazaCachePath("lookup", c.lookupCacheKey(locus), ".json")
	if err != nil {
		return
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return
	}
	_ = appfs.WriteFileAtomic(path, data, 0o644)
}

func (c *Client) lookupCacheKey(locus string) string {
	bases := make([]string, 0, len(c.instances))
	for _, spec := range c.instances {
		bases = append(bases, spec.Name+":"+spec.Base)
	}
	sort.Strings(bases)
	return strings.ToLower(strings.TrimSpace(locus)) + "|" + strings.Join(bases, "|")
}

func plazaCachePath(group string, key string, suffix string) (string, error) {
	dir, err := appfs.CacheDir("plaza", group)
	if err != nil {
		return "", fmt.Errorf("ensure PLAZA cache directory: %w", err)
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+suffix), nil
}
