package models

import "tangled.org/core/appview/pagination"

type IssueSearchOptions struct {
	Keywords    []string
	Phrases     []string
	RepoAt      string
	IsOpen      *bool
	AuthorDid   string
	Labels      []string
	LabelValues []string

	NegatedKeywords    []string
	NegatedPhrases     []string
	NegatedLabels      []string
	NegatedLabelValues []string
	NegatedAuthorDids  []string

	Page pagination.Page
}

func (o *IssueSearchOptions) HasSearchFilters() bool {
	return len(o.Keywords) > 0 || len(o.Phrases) > 0 ||
		o.AuthorDid != "" || len(o.NegatedAuthorDids) > 0 ||
		len(o.Labels) > 0 || len(o.NegatedLabels) > 0 ||
		len(o.LabelValues) > 0 || len(o.NegatedLabelValues) > 0 ||
		len(o.NegatedKeywords) > 0 || len(o.NegatedPhrases) > 0
}

type PullSearchOptions struct {
	Keywords    []string
	Phrases     []string
	RepoAt      string
	State       *PullState
	AuthorDid   string
	Labels      []string
	LabelValues []string

	NegatedKeywords    []string
	NegatedPhrases     []string
	NegatedLabels      []string
	NegatedLabelValues []string
	NegatedAuthorDids  []string

	Page pagination.Page
}

func (o *PullSearchOptions) HasSearchFilters() bool {
	return len(o.Keywords) > 0 || len(o.Phrases) > 0 ||
		o.AuthorDid != "" || len(o.NegatedAuthorDids) > 0 ||
		len(o.Labels) > 0 || len(o.NegatedLabels) > 0 ||
		len(o.LabelValues) > 0 || len(o.NegatedLabelValues) > 0 ||
		len(o.NegatedKeywords) > 0 || len(o.NegatedPhrases) > 0
}
