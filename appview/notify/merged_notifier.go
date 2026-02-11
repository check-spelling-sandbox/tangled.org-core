package notify

import (
	"context"
	"sync"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"tangled.org/core/appview/models"
)

type mergedNotifier struct {
	notifiers []Notifier
}

func NewMergedNotifier(notifiers []Notifier) Notifier {
	return &mergedNotifier{notifiers}
}

var _ Notifier = &mergedNotifier{}

// fanout calls the same method on all notifiers concurrently
func (m *mergedNotifier) fanout(callback func(Notifier)) {
	var wg sync.WaitGroup
	for _, n := range m.notifiers {
		wg.Add(1)
		go func(notifier Notifier) {
			defer wg.Done()
			callback(n)
		}(n)
	}
}

func (m *mergedNotifier) NewRepo(ctx context.Context, repo *models.Repo) {
	m.fanout(func(n Notifier) { n.NewRepo(ctx, repo) })
}

func (m *mergedNotifier) NewStar(ctx context.Context, star *models.Star) {
	m.fanout(func(n Notifier) { n.NewStar(ctx, star) })
}

func (m *mergedNotifier) DeleteStar(ctx context.Context, star *models.Star) {
	m.fanout(func(n Notifier) { n.DeleteStar(ctx, star) })
}

func (m *mergedNotifier) NewIssue(ctx context.Context, issue *models.Issue, mentions []syntax.DID) {
	m.fanout(func(n Notifier) { n.NewIssue(ctx, issue, mentions) })
}

func (m *mergedNotifier) NewIssueComment(ctx context.Context, comment *models.IssueComment, mentions []syntax.DID) {
	m.fanout(func(n Notifier) { n.NewIssueComment(ctx, comment, mentions) })
}

func (m *mergedNotifier) NewIssueState(ctx context.Context, actor syntax.DID, issue *models.Issue) {
	m.fanout(func(n Notifier) { n.NewIssueState(ctx, actor, issue) })
}

func (m *mergedNotifier) DeleteIssue(ctx context.Context, issue *models.Issue) {
	m.fanout(func(n Notifier) { n.DeleteIssue(ctx, issue) })
}

func (m *mergedNotifier) NewFollow(ctx context.Context, follow *models.Follow) {
	m.fanout(func(n Notifier) { n.NewFollow(ctx, follow) })
}

func (m *mergedNotifier) DeleteFollow(ctx context.Context, follow *models.Follow) {
	m.fanout(func(n Notifier) { n.DeleteFollow(ctx, follow) })
}

func (m *mergedNotifier) NewPull(ctx context.Context, pull *models.Pull) {
	m.fanout(func(n Notifier) { n.NewPull(ctx, pull) })
}

func (m *mergedNotifier) NewPullComment(ctx context.Context, comment *models.PullComment, mentions []syntax.DID) {
	m.fanout(func(n Notifier) { n.NewPullComment(ctx, comment, mentions) })
}

func (m *mergedNotifier) NewPullState(ctx context.Context, actor syntax.DID, pull *models.Pull) {
	m.fanout(func(n Notifier) { n.NewPullState(ctx, actor, pull) })
}

func (m *mergedNotifier) UpdateProfile(ctx context.Context, profile *models.Profile) {
	m.fanout(func(n Notifier) { n.UpdateProfile(ctx, profile) })
}

func (m *mergedNotifier) NewString(ctx context.Context, s *models.String) {
	m.fanout(func(n Notifier) { n.NewString(ctx, s) })
}

func (m *mergedNotifier) EditString(ctx context.Context, s *models.String) {
	m.fanout(func(n Notifier) { n.EditString(ctx, s) })
}

func (m *mergedNotifier) DeleteString(ctx context.Context, did, rkey string) {
	m.fanout(func(n Notifier) { n.DeleteString(ctx, did, rkey) })
}

func (m *mergedNotifier) Push(ctx context.Context, repo *models.Repo, ref, oldSha, newSha, committerDid string) {
	m.fanout(func(n Notifier) { n.Push(ctx, repo, ref, oldSha, newSha, committerDid) })
}
