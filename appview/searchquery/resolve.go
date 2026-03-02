package searchquery

import (
	"context"
	"log/slog"
	"strings"
)

// IdentResolver converts a handle/identifier string to a DID string.
type IdentResolver func(ctx context.Context, ident string) (string, error)

// ResolveAuthor extracts the "author" tag from the query and resolves
// both the positive and negated values to DIDs using the provided resolver.
// Returns empty string / nil on missing tags or resolution failure.
func ResolveAuthor(ctx context.Context, q *Query, resolve IdentResolver, log *slog.Logger) (authorDid string, negatedAuthorDids []string) {
	if authorHandle := q.Get("author"); authorHandle != nil {
		did, err := resolve(ctx, *authorHandle)
		if err != nil {
			log.Debug("failed to resolve author handle", "handle", *authorHandle, "err", err)
		} else {
			authorDid = did
		}
	}

	for _, handle := range q.GetAllNegated("author") {
		did, err := resolve(ctx, handle)
		if err != nil {
			log.Debug("failed to resolve negated author handle", "handle", handle, "err", err)
			continue
		}
		negatedAuthorDids = append(negatedAuthorDids, did)
	}

	return authorDid, negatedAuthorDids
}

// TextFilters holds the keyword and phrase filters extracted from a query.
type TextFilters struct {
	Keywords        []string
	NegatedKeywords []string
	Phrases         []string
	NegatedPhrases  []string
}

// ExtractTextFilters extracts keyword and quoted-phrase items from the query,
// separated into positive and negated slices.
func ExtractTextFilters(q *Query) TextFilters {
	var tf TextFilters
	for _, item := range q.Items() {
		switch item.Kind {
		case KindKeyword:
			if item.Negated {
				tf.NegatedKeywords = append(tf.NegatedKeywords, item.Value)
			} else {
				tf.Keywords = append(tf.Keywords, item.Value)
			}
		case KindQuoted:
			if item.Negated {
				tf.NegatedPhrases = append(tf.NegatedPhrases, item.Value)
			} else {
				tf.Phrases = append(tf.Phrases, item.Value)
			}
		}
	}
	return tf
}

// ResolveDIDLabelValues resolves handle values to DIDs for dynamic label
// tags whose name appears in didLabels. Tags not in didLabels are returned
// unchanged. On resolution failure the original value is kept.
func ResolveDIDLabelValues(
	ctx context.Context,
	tags []string,
	didLabels map[string]bool,
	resolve IdentResolver,
	log *slog.Logger,
) []string {
	resolved := make([]string, 0, len(tags))
	for _, tag := range tags {
		idx := strings.Index(tag, ":")
		if idx < 0 {
			resolved = append(resolved, tag)
			continue
		}
		name, val := tag[:idx], tag[idx+1:]
		if didLabels[name] {
			did, err := resolve(ctx, val)
			if err != nil {
				log.Debug("failed to resolve DID label value", "label", name, "value", val, "err", err)
				resolved = append(resolved, tag)
				continue
			}
			resolved = append(resolved, name+":"+did)
		} else {
			resolved = append(resolved, tag)
		}
	}
	return resolved
}
