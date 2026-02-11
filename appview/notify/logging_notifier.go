package notify

import (
	"context"
	"log/slog"

	"tangled.org/core/appview/models"
	tlog "tangled.org/core/log"

	"github.com/bluesky-social/indigo/atproto/syntax"
)

type loggingNotifier struct {
	inner  Notifier
	logger *slog.Logger
}

func NewLoggingNotifier(inner Notifier, logger *slog.Logger) Notifier {
	return &loggingNotifier{
		inner,
		logger,
	}
}

var _ Notifier = &loggingNotifier{}

func (l *loggingNotifier) NewRepo(ctx context.Context, repo *models.Repo) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "NewRepo"))
	l.inner.NewRepo(ctx, repo)
}

func (l *loggingNotifier) NewStar(ctx context.Context, star *models.Star) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "NewStar"))
	l.inner.NewStar(ctx, star)
}

func (l *loggingNotifier) DeleteStar(ctx context.Context, star *models.Star) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "DeleteStar"))
	l.inner.DeleteStar(ctx, star)
}

func (l *loggingNotifier) NewIssue(ctx context.Context, issue *models.Issue, mentions []syntax.DID) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "NewIssue"))
	l.inner.NewIssue(ctx, issue, mentions)
}

func (l *loggingNotifier) NewIssueComment(ctx context.Context, comment *models.IssueComment, mentions []syntax.DID) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "NewIssueComment"))
	l.inner.NewIssueComment(ctx, comment, mentions)
}

func (l *loggingNotifier) NewIssueState(ctx context.Context, actor syntax.DID, issue *models.Issue) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "NewIssueState"))
	l.inner.NewIssueState(ctx, actor, issue)
}

func (l *loggingNotifier) DeleteIssue(ctx context.Context, issue *models.Issue) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "DeleteIssue"))
	l.inner.DeleteIssue(ctx, issue)
}

func (l *loggingNotifier) NewFollow(ctx context.Context, follow *models.Follow) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "NewFollow"))
	l.inner.NewFollow(ctx, follow)
}

func (l *loggingNotifier) DeleteFollow(ctx context.Context, follow *models.Follow) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "DeleteFollow"))
	l.inner.DeleteFollow(ctx, follow)
}

func (l *loggingNotifier) NewPull(ctx context.Context, pull *models.Pull) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "NewPull"))
	l.inner.NewPull(ctx, pull)
}

func (l *loggingNotifier) NewPullComment(ctx context.Context, comment *models.PullComment, mentions []syntax.DID) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "NewPullComment"))
	l.inner.NewPullComment(ctx, comment, mentions)
}

func (l *loggingNotifier) NewPullState(ctx context.Context, actor syntax.DID, pull *models.Pull) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "NewPullState"))
	l.inner.NewPullState(ctx, actor, pull)
}

func (l *loggingNotifier) UpdateProfile(ctx context.Context, profile *models.Profile) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "UpdateProfile"))
	l.inner.UpdateProfile(ctx, profile)
}

func (l *loggingNotifier) NewString(ctx context.Context, s *models.String) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "NewString"))
	l.inner.NewString(ctx, s)
}

func (l *loggingNotifier) EditString(ctx context.Context, s *models.String) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "EditString"))
	l.inner.EditString(ctx, s)
}

func (l *loggingNotifier) DeleteString(ctx context.Context, did, rkey string) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "DeleteString"))
	l.inner.DeleteString(ctx, did, rkey)
}

func (l *loggingNotifier) Push(ctx context.Context, repo *models.Repo, ref, oldSha, newSha, committerDid string) {
	ctx = tlog.IntoContext(ctx, tlog.SubLogger(l.logger, "Push"))
	l.inner.Push(ctx, repo, ref, oldSha, newSha, committerDid)
}
