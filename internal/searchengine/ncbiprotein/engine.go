package ncbiprotein

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KiriKirby/phytozome-go/internal/model"
	phygoboost "github.com/KiriKirby/phytozome-go/internal/phygoboost"
	"golang.org/x/sync/singleflight"
)

const (
	SearchTypeProteinAccession   = "NCBI protein accession"
	SearchTypeProteinKeyword     = "NCBI protein keyword"
	SearchTypeNucleotideFallback = "NCBI nucleotide fallback"

	cacheSchemaVersion = "ncbiprotein-v5"
)

var transientSearchRetryDelay = 10 * time.Second

type ProteinFinder interface {
	SearchProteinRows(ctx context.Context, species model.SpeciesCandidate, term string, limit int) ([]model.KeywordResultRow, error)
}

type Engine struct {
	finder ProteinFinder
	mu     sync.RWMutex
	cache  map[string][]model.KeywordResultRow
	sf     singleflight.Group
}

func New(finder ProteinFinder) *Engine {
	return &Engine{
		finder: finder,
		cache:  make(map[string][]model.KeywordResultRow),
	}
}

func (e *Engine) SearchKeywordRows(ctx context.Context, species model.SpeciesCandidate, keyword string) ([]model.KeywordResultRow, error) {
	return phygoboost.RunTaskSpecValue(ctx, phygoboost.TaskSpec{
		Level:       phygoboost.ExecManaged,
		Domain:      "eutils.ncbi.nlm.nih.gov",
		Description: "search ncbi protein keyword engine rows",
	}, func(runCtx context.Context) ([]model.KeywordResultRow, error) {
		return e.searchKeywordRows(runCtx, species, keyword, 20)
	})
}

func (e *Engine) searchKeywordRows(ctx context.Context, species model.SpeciesCandidate, keyword string, limit int) ([]model.KeywordResultRow, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	searchType := SearchTypeProteinKeyword
	if looksLikeProteinAccession(keyword) {
		searchType = SearchTypeProteinAccession
	}
	cacheKey := e.cacheKey(species, keyword, searchType, limit)

	e.mu.RLock()
	if cached, ok := e.cache[cacheKey]; ok && cacheableResult(cached, keyword) {
		rows := cloneRows(cached)
		e.mu.RUnlock()
		return rows, nil
	}
	e.mu.RUnlock()
	if cached, ok := readCachedJSON[[]model.KeywordResultRow]("rows", cacheKey); ok && cacheableResult(cached, keyword) {
		rows := cloneRows(cached)
		e.mu.Lock()
		e.cache[cacheKey] = cloneRows(cached)
		e.mu.Unlock()
		return rows, nil
	}

	value, err, _ := e.sf.Do("keyword-rows:"+cacheKey, func() (any, error) {
		e.mu.RLock()
		if cached, ok := e.cache[cacheKey]; ok && cacheableResult(cached, keyword) {
			rows := cloneRows(cached)
			e.mu.RUnlock()
			return rows, nil
		}
		e.mu.RUnlock()
		if cached, ok := readCachedJSON[[]model.KeywordResultRow]("rows", cacheKey); ok && cacheableResult(cached, keyword) {
			rows := cloneRows(cached)
			e.mu.Lock()
			e.cache[cacheKey] = cloneRows(cached)
			e.mu.Unlock()
			return rows, nil
		}

		rows, err := e.searchProteinRowsWithTransientRetry(ctx, species, keyword, limit)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			rows[i].SearchTerm = keyword
			if strings.TrimSpace(rows[i].SearchType) == "" {
				rows[i].SearchType = searchType
			}
		}
		if len(rows) > 0 {
			e.mu.Lock()
			e.cache[cacheKey] = cloneRows(rows)
			e.mu.Unlock()
			writeCachedJSON("rows", cacheKey, rows)
		}
		return rows, nil
	})
	if err != nil {
		return nil, err
	}
	return cloneRows(value.([]model.KeywordResultRow)), nil
}

func (e *Engine) searchProteinRowsWithTransientRetry(ctx context.Context, species model.SpeciesCandidate, keyword string, limit int) ([]model.KeywordResultRow, error) {
	rows, err := e.finder.SearchProteinRows(ctx, species, keyword, limit)
	if (err == nil && len(rows) > 0) || isContextError(err) {
		return rows, err
	}
	if delay := transientSearchRetryDelay; delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return e.finder.SearchProteinRows(ctx, species, keyword, limit)
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (e *Engine) cacheKey(species model.SpeciesCandidate, term string, searchType string, limit int) string {
	return strings.Join([]string{
		cacheSchemaVersion,
		strconv.Itoa(species.ProteomeID),
		strings.TrimSpace(species.JBrowseName),
		strings.ToLower(strings.TrimSpace(term)),
		searchType,
		strconv.Itoa(limit),
	}, "|")
}

func looksLikeProteinAccession(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, ch := range value {
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z':
			hasLetter = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		case ch == '_' || ch == '.' || ch == '-':
		default:
			return false
		}
	}
	return hasLetter && hasDigit
}

func cacheableResult(rows []model.KeywordResultRow, keyword string) bool {
	if len(rows) == 0 {
		return false
	}
	for _, row := range rows {
		if strings.TrimSpace(row.SearchType) == "" {
			return false
		}
		if strings.TrimSpace(row.SearchTerm) == "" && strings.TrimSpace(keyword) != "" {
			return false
		}
	}
	return true
}

func cloneRows(rows []model.KeywordResultRow) []model.KeywordResultRow {
	out := append([]model.KeywordResultRow(nil), rows...)
	for i := range out {
		if out[i].ExtraColumns == nil {
			continue
		}
		extra := make(map[string]string, len(out[i].ExtraColumns))
		for key, value := range out[i].ExtraColumns {
			extra[key] = value
		}
		out[i].ExtraColumns = extra
	}
	return out
}
