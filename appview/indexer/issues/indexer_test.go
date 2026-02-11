package issues_indexer

import (
	"context"
	"os"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tangled.org/core/appview/models"
	"tangled.org/core/appview/pagination"
	"tangled.org/core/appview/searchquery"
)

func setupTestIndexer(t *testing.T) (*Indexer, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "issue_indexer_test")
	require.NoError(t, err)

	ix := NewIndexer(tmpDir)

	mapping, err := generateIssueIndexMapping()
	require.NoError(t, err)

	indexer, err := bleve.New(tmpDir, mapping)
	require.NoError(t, err)
	ix.indexer = indexer

	cleanup := func() {
		ix.indexer.Close()
		os.RemoveAll(tmpDir)
	}

	return ix, cleanup
}

func boolPtr(b bool) *bool { return &b }

func makeLabelState(labels ...string) models.LabelState {
	state := models.NewLabelState()
	for _, label := range labels {
		state.Inner()[label] = make(map[string]struct{})
		state.SetName(label, label)
	}
	return state
}

func TestSearchFilters(t *testing.T) {
	ix, cleanup := setupTestIndexer(t)
	defer cleanup()

	ctx := context.Background()

	err := ix.Index(ctx,
		models.Issue{Id: 1, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Fix login bug", Body: "Users cannot login", Open: true, Did: "did:plc:alice", Labels: makeLabelState("bug")},
		models.Issue{Id: 2, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Add dark mode", Body: "Implement dark theme", Open: true, Did: "did:plc:bob", Labels: makeLabelState("feature")},
		models.Issue{Id: 3, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Fix login timeout", Body: "Login takes too long", Open: false, Did: "did:plc:alice", Labels: makeLabelState("bug")},
	)
	require.NoError(t, err)

	opts := func() models.IssueSearchOptions {
		return models.IssueSearchOptions{
			RepoAt: "at://did:plc:test/sh.tangled.repo/abc",
			IsOpen: boolPtr(true),
			Page:   pagination.Page{Limit: 10},
		}
	}

	// Keyword in title
	o := opts()
	o.Keywords = []string{"bug"}
	result, err := ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
	assert.Contains(t, result.Hits, int64(1))

	// Keyword in body
	o = opts()
	o.Keywords = []string{"theme"}
	result, err = ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
	assert.Contains(t, result.Hits, int64(2))

	// Phrase match
	o = opts()
	o.Phrases = []string{"login bug"}
	result, err = ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
	assert.Contains(t, result.Hits, int64(1))

	// Author filter
	o = opts()
	o.AuthorDid = "did:plc:alice"
	result, err = ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
	assert.Contains(t, result.Hits, int64(1))

	// Label filter
	o = opts()
	o.Labels = []string{"bug"}
	result, err = ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
	assert.Contains(t, result.Hits, int64(1))

	// State filter (closed)
	o = opts()
	o.IsOpen = boolPtr(false)
	o.Labels = []string{"bug"}
	result, err = ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
	assert.Contains(t, result.Hits, int64(3))

	// Combined: keyword + author + label
	o = opts()
	o.Keywords = []string{"login"}
	o.AuthorDid = "did:plc:alice"
	o.Labels = []string{"bug"}
	result, err = ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
	assert.Contains(t, result.Hits, int64(1))
}

func TestSearchLabelAND(t *testing.T) {
	ix, cleanup := setupTestIndexer(t)
	defer cleanup()

	ctx := context.Background()

	err := ix.Index(ctx,
		models.Issue{Id: 1, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Issue 1", Body: "Body", Open: true, Did: "did:plc:alice", Labels: makeLabelState("bug")},
		models.Issue{Id: 2, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Issue 2", Body: "Body", Open: true, Did: "did:plc:bob", Labels: makeLabelState("bug", "urgent")},
	)
	require.NoError(t, err)

	result, err := ix.Search(ctx, models.IssueSearchOptions{
		RepoAt: "at://did:plc:test/sh.tangled.repo/abc",
		IsOpen: boolPtr(true),
		Labels: []string{"bug", "urgent"},
		Page:   pagination.Page{Limit: 10},
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
	assert.Contains(t, result.Hits, int64(2))
}

func TestSearchNegation(t *testing.T) {
	ix, cleanup := setupTestIndexer(t)
	defer cleanup()

	ctx := context.Background()

	err := ix.Index(ctx,
		models.Issue{Id: 1, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Fix login bug", Body: "Users cannot login", Open: true, Did: "did:plc:alice", Labels: makeLabelState("bug")},
		models.Issue{Id: 2, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Add dark mode", Body: "Implement dark theme", Open: true, Did: "did:plc:bob", Labels: makeLabelState("feature")},
		models.Issue{Id: 3, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Fix timeout bug", Body: "Timeout on save", Open: true, Did: "did:plc:alice", Labels: makeLabelState("bug", "urgent")},
	)
	require.NoError(t, err)

	opts := func() models.IssueSearchOptions {
		return models.IssueSearchOptions{
			RepoAt: "at://did:plc:test/sh.tangled.repo/abc",
			IsOpen: boolPtr(true),
			Page:   pagination.Page{Limit: 10},
		}
	}

	// Negated label: exclude "bug", should only return issue 2
	o := opts()
	o.NegatedLabels = []string{"bug"}
	result, err := ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
	assert.Contains(t, result.Hits, int64(2))

	// Negated keyword: exclude "login", should return issues 2 and 3
	o = opts()
	o.NegatedKeywords = []string{"login"}
	result, err = ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), result.Total)
	assert.Contains(t, result.Hits, int64(2))
	assert.Contains(t, result.Hits, int64(3))

	// Positive label + negated label: bug but not urgent
	o = opts()
	o.Labels = []string{"bug"}
	o.NegatedLabels = []string{"urgent"}
	result, err = ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
	assert.Contains(t, result.Hits, int64(1))

	// Negated phrase
	o = opts()
	o.NegatedPhrases = []string{"dark theme"}
	result, err = ix.Search(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), result.Total)
	assert.NotContains(t, result.Hits, int64(2))
}

func TestSearchNegatedPhraseParsed(t *testing.T) {
	ix, cleanup := setupTestIndexer(t)
	defer cleanup()

	ctx := context.Background()

	err := ix.Index(ctx,
		models.Issue{Id: 1, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Fix login bug", Body: "Users cannot login", Open: true, Did: "did:plc:alice"},
		models.Issue{Id: 2, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Add dark mode", Body: "Implement dark theme", Open: true, Did: "did:plc:bob"},
		models.Issue{Id: 3, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Fix timeout bug", Body: "Timeout on save", Open: true, Did: "did:plc:alice"},
	)
	require.NoError(t, err)

	// Parse a query with a negated quoted phrase, as the handler would
	query := searchquery.Parse(`-"dark theme"`)
	var negatedPhrases []string
	for _, item := range query.Items() {
		if item.Kind == searchquery.KindQuoted && item.Negated {
			negatedPhrases = append(negatedPhrases, item.Value)
		}
	}
	require.Equal(t, []string{"dark theme"}, negatedPhrases)

	result, err := ix.Search(ctx, models.IssueSearchOptions{
		RepoAt:         "at://did:plc:test/sh.tangled.repo/abc",
		IsOpen:         boolPtr(true),
		NegatedPhrases: negatedPhrases,
		Page:           pagination.Page{Limit: 10},
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(2), result.Total)
	assert.NotContains(t, result.Hits, int64(2))
}

func TestSearchNoResults(t *testing.T) {
	ix, cleanup := setupTestIndexer(t)
	defer cleanup()

	ctx := context.Background()

	err := ix.Index(ctx,
		models.Issue{Id: 1, RepoAt: "at://did:plc:test/sh.tangled.repo/abc", Title: "Issue", Body: "Body", Open: true, Did: "did:plc:alice"},
	)
	require.NoError(t, err)

	result, err := ix.Search(ctx, models.IssueSearchOptions{
		Keywords: []string{"nonexistent"},
		RepoAt:   "at://did:plc:test/sh.tangled.repo/abc",
		IsOpen:   boolPtr(true),
		Page:     pagination.Page{Limit: 10},
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(0), result.Total)
	assert.Empty(t, result.Hits)
}
