package ncbiprotein

import (
	"context"
	"errors"
	"testing"

	"github.com/KiriKirby/phytozome-go/internal/model"
)

type retryProteinFinder struct {
	calls int
	mode  string
}

func (f *retryProteinFinder) SearchProteinRows(ctx context.Context, species model.SpeciesCandidate, term string, limit int) ([]model.KeywordResultRow, error) {
	f.calls++
	if f.mode == "empty" && f.calls == 1 {
		return nil, nil
	}
	if f.mode != "empty" && f.calls == 1 {
		return nil, errors.New("temporary NCBI failure")
	}
	return []model.KeywordResultRow{{ProteinID: "XP_015650724.1", SequenceID: "XP_015650724.1"}}, nil
}

func TestSearchKeywordRowsRetriesTransientFinderErrorOnce(t *testing.T) {
	oldDelay := transientSearchRetryDelay
	transientSearchRetryDelay = 0
	defer func() { transientSearchRetryDelay = oldDelay }()

	finder := &retryProteinFinder{}
	engine := New(finder)
	rows, err := engine.searchProteinRowsWithTransientRetry(context.Background(), model.SpeciesCandidate{}, "XP_015650724.1", 20)
	if err != nil {
		t.Fatalf("searchProteinRowsWithTransientRetry returned error after retry: %v", err)
	}
	if finder.calls != 2 {
		t.Fatalf("finder calls = %d, want 2", finder.calls)
	}
	if len(rows) != 1 || rows[0].ProteinID != "XP_015650724.1" {
		t.Fatalf("unexpected rows after retry: %#v", rows)
	}
}

func TestSearchKeywordRowsRetriesEmptyFinderResultOnce(t *testing.T) {
	oldDelay := transientSearchRetryDelay
	transientSearchRetryDelay = 0
	defer func() { transientSearchRetryDelay = oldDelay }()

	finder := &retryProteinFinder{mode: "empty"}
	engine := New(finder)
	rows, err := engine.searchProteinRowsWithTransientRetry(context.Background(), model.SpeciesCandidate{}, "XP_015650724.1", 20)
	if err != nil {
		t.Fatalf("searchProteinRowsWithTransientRetry returned error after empty retry: %v", err)
	}
	if finder.calls != 2 {
		t.Fatalf("finder calls = %d, want 2", finder.calls)
	}
	if len(rows) != 1 || rows[0].ProteinID != "XP_015650724.1" {
		t.Fatalf("unexpected rows after empty retry: %#v", rows)
	}
}

func TestEmptyNCBIKeywordResultsAreNotCacheable(t *testing.T) {
	if cacheableResult(nil, "XP_015650724.1") {
		t.Fatal("empty NCBI keyword results must not be cacheable because they need recovery retry")
	}
}

type staticProteinFinder struct {
	rows []model.KeywordResultRow
}

func (f staticProteinFinder) SearchProteinRows(ctx context.Context, species model.SpeciesCandidate, term string, limit int) ([]model.KeywordResultRow, error) {
	return f.rows, nil
}

func TestSearchKeywordRowsPreservesFinderSearchType(t *testing.T) {
	engine := New(staticProteinFinder{rows: []model.KeywordResultRow{{
		ProteinID:  "XP_LINKED.1",
		SequenceID: "XP_LINKED.1",
		SearchType: SearchTypeNucleotideFallback,
	}}})
	rows, err := engine.searchKeywordRows(context.Background(), model.SpeciesCandidate{}, "NM_LINKED.1", 20)
	if err != nil {
		t.Fatalf("searchKeywordRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if rows[0].SearchType != SearchTypeNucleotideFallback {
		t.Fatalf("SearchType = %q, want %q", rows[0].SearchType, SearchTypeNucleotideFallback)
	}
}
