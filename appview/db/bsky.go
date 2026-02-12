package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"tangled.org/core/appview/models"
)

func InsertBlueskyPosts(e Execer, posts []models.BskyPost) error {
	if len(posts) == 0 {
		return nil
	}

	stmt, err := e.Prepare(`
		insert or replace into bluesky_posts (rkey, text, created_at, langs, facets, embed, like_count, reply_count, repost_count, quote_count)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, post := range posts {
		var langsJSON, facetsJSON, embedJSON []byte

		if len(post.Langs) > 0 {
			langsJSON, _ = json.Marshal(post.Langs)
		}
		if len(post.Facets) > 0 {
			facetsJSON, _ = json.Marshal(post.Facets)
		}
		if post.Embed != nil {
			embedJSON, _ = json.Marshal(post.Embed)
		}

		_, err := stmt.Exec(
			post.Rkey,
			post.Text,
			post.CreatedAt.Format(time.RFC3339),
			nullString(langsJSON),
			nullString(facetsJSON),
			nullString(embedJSON),
			post.LikeCount,
			post.ReplyCount,
			post.RepostCount,
			post.QuoteCount,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func nullString(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

func GetBlueskyPosts(e Execer, limit int) ([]models.BskyPost, error) {
	query := `
		select rkey, text, created_at, langs, facets, embed, like_count, reply_count, repost_count, quote_count
		from bluesky_posts
		order by created_at desc
		limit ?
	`

	rows, err := e.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.BskyPost
	for rows.Next() {
		var rkey, text, createdAt string
		var langs, facets, embed sql.Null[string]
		var likeCount, replyCount, repostCount, quoteCount int64

		err := rows.Scan(&rkey, &text, &createdAt, &langs, &facets, &embed, &likeCount, &replyCount, &repostCount, &quoteCount)
		if err != nil {
			return nil, err
		}

		post := models.BskyPost{
			Rkey:        rkey,
			Text:        text,
			LikeCount:   likeCount,
			ReplyCount:  replyCount,
			RepostCount: repostCount,
			QuoteCount:  quoteCount,
		}

		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			post.CreatedAt = t
		}
		if langs.Valid && langs.V != "" {
			json.Unmarshal([]byte(langs.V), &post.Langs)
		}
		if facets.Valid && facets.V != "" {
			json.Unmarshal([]byte(facets.V), &post.Facets)
		}
		if embed.Valid && embed.V != "" {
			json.Unmarshal([]byte(embed.V), &post.Embed)
		}

		posts = append(posts, post)
	}

	return posts, rows.Err()
}
