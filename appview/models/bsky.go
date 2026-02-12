package models

import (
	"time"

	apibsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

type BskyPost struct {
	Rkey       string
	Text       string
	CreatedAt  time.Time
	Langs      []string
	Tags       []string
	Embed      *apibsky.FeedDefs_PostView_Embed
	Facets     []*apibsky.RichtextFacet
	Labels     *apibsky.FeedPost_Labels
	Reply      *apibsky.FeedPost_ReplyRef
	LikeCount  int64
	ReplyCount int64
	RepostCount int64
	QuoteCount int64
}

func NewBskyPostFromView(postView *apibsky.FeedDefs_PostView) (*BskyPost, error) {
	atUri, err := syntax.ParseATURI(postView.Uri)
	if err != nil {
		return nil, err
	}

	// decode the record to get FeedPost
	feedPost, ok := postView.Record.Val.(*apibsky.FeedPost)
	if !ok {
		return nil, err
	}

	createdAt, err := time.Parse(time.RFC3339, feedPost.CreatedAt)
	if err != nil {
		return nil, err
	}

	var likeCount, replyCount, repostCount, quoteCount int64
	if postView.LikeCount != nil {
		likeCount = *postView.LikeCount
	}
	if postView.ReplyCount != nil {
		replyCount = *postView.ReplyCount
	}
	if postView.RepostCount != nil {
		repostCount = *postView.RepostCount
	}
	if postView.QuoteCount != nil {
		quoteCount = *postView.QuoteCount
	}

	return &BskyPost{
		Rkey:        atUri.RecordKey().String(),
		Text:        feedPost.Text,
		CreatedAt:   createdAt,
		Langs:       feedPost.Langs,
		Tags:        feedPost.Tags,
		Embed:       postView.Embed,
		Facets:      feedPost.Facets,
		Labels:      feedPost.Labels,
		Reply:       feedPost.Reply,
		LikeCount:   likeCount,
		ReplyCount:  replyCount,
		RepostCount: repostCount,
		QuoteCount:  quoteCount,
	}, nil
}
