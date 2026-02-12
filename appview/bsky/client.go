package bsky

import (
	"context"
	"log"

	bsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	"tangled.org/core/appview/models"
	"tangled.org/core/consts"
)

func FetchPosts(ctx context.Context, c *xrpc.Client, limit int, cursor string) ([]models.BskyPost, string, error) {
	resp, err := bsky.FeedGetAuthorFeed(ctx, c, consts.TangledDid, cursor, "posts_no_replies", false, int64(limit))
	if err != nil {
		return nil, "", err
	}

	var posts []models.BskyPost
	for _, feedViewPost := range resp.Feed {
		// skip quote posts
		if feedViewPost.Post.Embed != nil && feedViewPost.Post.Embed.EmbedRecord_View != nil {
			continue
		}

		post, err := models.NewBskyPostFromView(feedViewPost.Post)
		if err != nil {
			log.Println(err)
			continue
		}

		posts = append(posts, *post)
	}

	return posts, stringPtr(resp.Cursor), nil
}

func stringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
