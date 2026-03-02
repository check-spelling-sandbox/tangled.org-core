// heavily inspired by gitea's model (basically copy-pasted)
package issues_indexer

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/token/camelcase"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unicodenorm"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/index/upsidedown"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"tangled.org/core/appview/db"
	"tangled.org/core/appview/indexer/base36"
	bleveutil "tangled.org/core/appview/indexer/bleve"
	"tangled.org/core/appview/models"
	"tangled.org/core/appview/pagination"
	tlog "tangled.org/core/log"
)

const (
	issueIndexerAnalyzer = "issueIndexer"
	issueIndexerDocType  = "issueIndexerDocType"

	unicodeNormalizeName = "uicodeNormalize"

	// Bump this when the index mapping changes to trigger a rebuild.
	issueIndexerVersion = 3
)

type Indexer struct {
	indexer bleve.Index
	path    string
}

func NewIndexer(indexDir string) *Indexer {
	return &Indexer{
		path: indexDir,
	}
}

// Init initializes the indexer
func (ix *Indexer) Init(ctx context.Context, e db.Execer) {
	l := tlog.FromContext(ctx)
	existed, err := ix.intialize(ctx)
	if err != nil {
		log.Fatalln("failed to initialize issue indexer", err)
	}
	if !existed {
		l.Debug("Populating the issue indexer")
		err := PopulateIndexer(ctx, ix, e)
		if err != nil {
			log.Fatalln("failed to populate issue indexer", err)
		}
	}

	count, _ := ix.indexer.DocCount()
	l.Info("Initialized the issue indexer", "docCount", count)
}

func generateIssueIndexMapping() (mapping.IndexMapping, error) {
	mapping := bleve.NewIndexMapping()
	docMapping := bleve.NewDocumentMapping()

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Store = false
	textFieldMapping.IncludeInAll = false

	boolFieldMapping := bleve.NewBooleanFieldMapping()
	boolFieldMapping.Store = false
	boolFieldMapping.IncludeInAll = false

	keywordFieldMapping := bleve.NewKeywordFieldMapping()
	keywordFieldMapping.Store = false
	keywordFieldMapping.IncludeInAll = false

	// numericFieldMapping := bleve.NewNumericFieldMapping()

	docMapping.AddFieldMappingsAt("title", textFieldMapping)
	docMapping.AddFieldMappingsAt("body", textFieldMapping)

	docMapping.AddFieldMappingsAt("repo_at", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("is_open", boolFieldMapping)
	docMapping.AddFieldMappingsAt("author_did", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("labels", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("label_values", keywordFieldMapping)

	err := mapping.AddCustomTokenFilter(unicodeNormalizeName, map[string]any{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	})
	if err != nil {
		return nil, err
	}

	err = mapping.AddCustomAnalyzer(issueIndexerAnalyzer, map[string]any{
		"type":          custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{unicodeNormalizeName, camelcase.Name, lowercase.Name},
	})
	if err != nil {
		return nil, err
	}

	mapping.DefaultAnalyzer = issueIndexerAnalyzer
	mapping.AddDocumentMapping(issueIndexerDocType, docMapping)
	mapping.AddDocumentMapping("_all", bleve.NewDocumentDisabledMapping())
	mapping.DefaultMapping = bleve.NewDocumentDisabledMapping()

	return mapping, nil
}

func (ix *Indexer) intialize(ctx context.Context) (bool, error) {
	if ix.indexer != nil {
		return false, errors.New("indexer is already initialized")
	}

	indexer, err := openIndexer(ctx, ix.path, issueIndexerVersion)
	if err != nil {
		return false, err
	}
	if indexer != nil {
		ix.indexer = indexer
		return true, nil
	}

	mapping, err := generateIssueIndexMapping()
	if err != nil {
		return false, err
	}
	indexer, err = bleve.New(ix.path, mapping)
	if err != nil {
		return false, err
	}
	indexer.SetInternal([]byte("mapping_version"), []byte{byte(issueIndexerVersion)})

	ix.indexer = indexer

	return false, nil
}

func openIndexer(ctx context.Context, path string, version int) (bleve.Index, error) {
	l := tlog.FromContext(ctx)
	indexer, err := bleve.Open(path)
	if err != nil {
		if errors.Is(err, upsidedown.IncompatibleVersion) {
			l.Info("Indexer was built with a previous version of bleve, deleting and rebuilding")
			return nil, os.RemoveAll(path)
		}
		return nil, nil
	}

	storedVersion, _ := indexer.GetInternal([]byte("mapping_version"))
	if storedVersion == nil || int(storedVersion[0]) != version {
		l.Info("Indexer mapping version changed, deleting and rebuilding")
		indexer.Close()
		return nil, os.RemoveAll(path)
	}

	return indexer, nil
}

func PopulateIndexer(ctx context.Context, ix *Indexer, e db.Execer) error {
	l := tlog.FromContext(ctx)
	count := 0
	err := pagination.IterateAll(
		func(page pagination.Page) ([]models.Issue, error) {
			return db.GetIssuesPaginated(e, page)
		},
		func(issues []models.Issue) error {
			count += len(issues)
			return ix.Index(ctx, issues...)
		},
	)
	l.Info("issues indexed", "count", count)
	return err
}

type issueData struct {
	ID          int64    `json:"id"`
	RepoAt      string   `json:"repo_at"`
	IssueID     int      `json:"issue_id"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	IsOpen      bool     `json:"is_open"`
	AuthorDid   string   `json:"author_did"`
	Labels      []string `json:"labels"`
	LabelValues []string `json:"label_values"`

	Comments []IssueCommentData `json:"comments"`
}

func makeIssueData(issue *models.Issue) *issueData {
	return &issueData{
		ID:          issue.Id,
		RepoAt:      issue.RepoAt.String(),
		IssueID:     issue.IssueId,
		Title:       issue.Title,
		Body:        issue.Body,
		IsOpen:      issue.Open,
		AuthorDid:   issue.Did,
		Labels:      issue.Labels.LabelNames(),
		LabelValues: issue.Labels.LabelNameValues(),
	}
}

// Type returns the document type, for bleve's mapping.Classifier interface.
func (i *issueData) Type() string {
	return issueIndexerDocType
}

type IssueCommentData struct {
	Body string `json:"body"`
}

type SearchResult struct {
	Hits  []int64
	Total uint64
}

const maxBatchSize = 20

func (ix *Indexer) Index(ctx context.Context, issues ...models.Issue) error {
	batch := bleveutil.NewFlushingBatch(ix.indexer, maxBatchSize)
	for _, issue := range issues {
		issueData := makeIssueData(&issue)
		if err := batch.Index(base36.Encode(issue.Id), issueData); err != nil {
			return err
		}
	}
	return batch.Flush()
}

func (ix *Indexer) Delete(ctx context.Context, issueId int64) error {
	return ix.indexer.Delete(base36.Encode(issueId))
}

func (ix *Indexer) Search(ctx context.Context, opts models.IssueSearchOptions) (*SearchResult, error) {
	var musts []query.Query
	var mustNots []query.Query

	for _, keyword := range opts.Keywords {
		musts = append(musts, bleve.NewDisjunctionQuery(
			bleveutil.MatchAndQuery("title", keyword, issueIndexerAnalyzer, 0),
			bleveutil.MatchAndQuery("body", keyword, issueIndexerAnalyzer, 0),
		))
	}

	for _, phrase := range opts.Phrases {
		musts = append(musts, bleve.NewDisjunctionQuery(
			bleveutil.MatchPhraseQuery("title", phrase, issueIndexerAnalyzer),
			bleveutil.MatchPhraseQuery("body", phrase, issueIndexerAnalyzer),
		))
	}

	for _, keyword := range opts.NegatedKeywords {
		mustNots = append(mustNots, bleve.NewDisjunctionQuery(
			bleveutil.MatchAndQuery("title", keyword, issueIndexerAnalyzer, 0),
			bleveutil.MatchAndQuery("body", keyword, issueIndexerAnalyzer, 0),
		))
	}

	for _, phrase := range opts.NegatedPhrases {
		mustNots = append(mustNots, bleve.NewDisjunctionQuery(
			bleveutil.MatchPhraseQuery("title", phrase, issueIndexerAnalyzer),
			bleveutil.MatchPhraseQuery("body", phrase, issueIndexerAnalyzer),
		))
	}

	musts = append(musts, bleveutil.KeywordFieldQuery("repo_at", opts.RepoAt))
	if opts.IsOpen != nil {
		musts = append(musts, bleveutil.BoolFieldQuery("is_open", *opts.IsOpen))
	}

	if opts.AuthorDid != "" {
		musts = append(musts, bleveutil.KeywordFieldQuery("author_did", opts.AuthorDid))
	}

	for _, label := range opts.Labels {
		musts = append(musts, bleveutil.KeywordFieldQuery("labels", label))
	}

	for _, did := range opts.NegatedAuthorDids {
		mustNots = append(mustNots, bleveutil.KeywordFieldQuery("author_did", did))
	}

	for _, label := range opts.NegatedLabels {
		mustNots = append(mustNots, bleveutil.KeywordFieldQuery("labels", label))
	}

	for _, lv := range opts.LabelValues {
		musts = append(musts, bleveutil.KeywordFieldQuery("label_values", lv))
	}

	for _, lv := range opts.NegatedLabelValues {
		mustNots = append(mustNots, bleveutil.KeywordFieldQuery("label_values", lv))
	}

	indexerQuery := bleve.NewBooleanQuery()
	indexerQuery.AddMust(musts...)
	indexerQuery.AddMustNot(mustNots...)
	searchReq := bleve.NewSearchRequestOptions(indexerQuery, opts.Page.Limit, opts.Page.Offset, false)
	res, err := ix.indexer.SearchInContext(ctx, searchReq)
	if err != nil {
		return nil, nil
	}
	ret := &SearchResult{
		Total: res.Total,
		Hits:  make([]int64, len(res.Hits)),
	}
	for i, hit := range res.Hits {
		id, err := base36.Decode(hit.ID)
		if err != nil {
			return nil, err
		}
		ret.Hits[i] = id
	}
	return ret, nil
}
