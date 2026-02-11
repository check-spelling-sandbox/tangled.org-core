package db

import (
	"context"
	"slices"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"tangled.org/core/api/tangled"
	"tangled.org/core/appview/db"
	"tangled.org/core/appview/models"
	"tangled.org/core/appview/notify"
	"tangled.org/core/idresolver"
	"tangled.org/core/log"
	"tangled.org/core/orm"
	"tangled.org/core/sets"
)

const (
	maxMentions = 8
)

type databaseNotifier struct {
	db  *db.DB
	res *idresolver.Resolver
}

func NewDatabaseNotifier(database *db.DB, resolver *idresolver.Resolver) notify.Notifier {
	return &databaseNotifier{
		db:  database,
		res: resolver,
	}
}

var _ notify.Notifier = &databaseNotifier{}

func (n *databaseNotifier) NewRepo(ctx context.Context, repo *models.Repo) {
	// no-op for now
}

func (n *databaseNotifier) NewStar(ctx context.Context, star *models.Star) {
	l := log.FromContext(ctx)

	if star.RepoAt.Collection().String() != tangled.RepoNSID {
		// skip string stars for now
		return
	}
	var err error
	repo, err := db.GetRepo(n.db, orm.FilterEq("at_uri", string(star.RepoAt)))
	if err != nil {
		l.Error("failed to get repos", "err", err)
		return
	}

	actorDid := syntax.DID(star.Did)
	recipients := sets.Singleton(syntax.DID(repo.Did))
	eventType := models.NotificationTypeRepoStarred
	entityType := "repo"
	entityId := star.RepoAt.String()
	repoId := &repo.Id
	var issueId *int64
	var pullId *int64

	n.notifyEvent(
		ctx,
		actorDid,
		recipients,
		eventType,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
}

func (n *databaseNotifier) DeleteStar(ctx context.Context, star *models.Star) {
	// no-op
}

func (n *databaseNotifier) NewIssue(ctx context.Context, issue *models.Issue, mentions []syntax.DID) {
	l := log.FromContext(ctx)

	collaborators, err := db.GetCollaborators(n.db, orm.FilterEq("repo_at", issue.Repo.RepoAt()))
	if err != nil {
		l.Error("failed to fetch collaborators", "err", err)
		return
	}

	// build the recipients list
	// - owner of the repo
	// - collaborators in the repo
	// - remove users already mentioned
	recipients := sets.Singleton(syntax.DID(issue.Repo.Did))
	for _, c := range collaborators {
		recipients.Insert(c.SubjectDid)
	}
	for _, m := range mentions {
		recipients.Remove(m)
	}

	actorDid := syntax.DID(issue.Did)
	entityType := "issue"
	entityId := issue.AtUri().String()
	repoId := &issue.Repo.Id
	issueId := &issue.Id
	var pullId *int64

	n.notifyEvent(
		ctx,
		actorDid,
		recipients,
		models.NotificationTypeIssueCreated,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
	n.notifyEvent(
		ctx,
		actorDid,
		sets.Collect(slices.Values(mentions)),
		models.NotificationTypeUserMentioned,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
}

func (n *databaseNotifier) NewIssueComment(ctx context.Context, comment *models.IssueComment, mentions []syntax.DID) {
	l := log.FromContext(ctx)

	issues, err := db.GetIssues(n.db, orm.FilterEq("at_uri", comment.IssueAt))
	if err != nil {
		l.Error("failed to get issues", "err", err)
		return
	}
	if len(issues) == 0 {
		l.Error("no issue found for", "err", comment.IssueAt)
		return
	}
	issue := issues[0]

	// built the recipients list:
	// - the owner of the repo
	// - | if the comment is a reply -> everybody on that thread
	//   | if the comment is a top level -> just the issue owner
	// - remove mentioned users from the recipients list
	recipients := sets.Singleton(syntax.DID(issue.Repo.Did))

	if comment.IsReply() {
		// if this comment is a reply, then notify everybody in that thread
		parentAtUri := *comment.ReplyTo

		// find the parent thread, and add all DIDs from here to the recipient list
		for _, t := range issue.CommentList() {
			if t.Self.AtUri().String() == parentAtUri {
				for _, p := range t.Participants() {
					recipients.Insert(p)
				}
			}
		}
	} else {
		// not a reply, notify just the issue author
		recipients.Insert(syntax.DID(issue.Did))
	}

	for _, m := range mentions {
		recipients.Remove(m)
	}

	actorDid := syntax.DID(comment.Did)
	entityType := "issue"
	entityId := issue.AtUri().String()
	repoId := &issue.Repo.Id
	issueId := &issue.Id
	var pullId *int64

	n.notifyEvent(
		ctx,
		actorDid,
		recipients,
		models.NotificationTypeIssueCommented,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
	n.notifyEvent(
		ctx,
		actorDid,
		sets.Collect(slices.Values(mentions)),
		models.NotificationTypeUserMentioned,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
}

func (n *databaseNotifier) DeleteIssue(ctx context.Context, issue *models.Issue) {
	// no-op for now
}

func (n *databaseNotifier) NewFollow(ctx context.Context, follow *models.Follow) {
	actorDid := syntax.DID(follow.UserDid)
	recipients := sets.Singleton(syntax.DID(follow.SubjectDid))
	eventType := models.NotificationTypeFollowed
	entityType := "follow"
	entityId := follow.UserDid
	var repoId, issueId, pullId *int64

	n.notifyEvent(
		ctx,
		actorDid,
		recipients,
		eventType,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
}

func (n *databaseNotifier) DeleteFollow(ctx context.Context, follow *models.Follow) {
	// no-op
}

func (n *databaseNotifier) NewPull(ctx context.Context, pull *models.Pull) {
	l := log.FromContext(ctx)

	repo, err := db.GetRepo(n.db, orm.FilterEq("at_uri", string(pull.RepoAt)))
	if err != nil {
		l.Error("failed to get repos", "err", err)
		return
	}
	collaborators, err := db.GetCollaborators(n.db, orm.FilterEq("repo_at", repo.RepoAt()))
	if err != nil {
		l.Error("failed to fetch collaborators", "err", err)
		return
	}

	// build the recipients list
	// - owner of the repo
	// - collaborators in the repo
	recipients := sets.Singleton(syntax.DID(repo.Did))
	for _, c := range collaborators {
		recipients.Insert(c.SubjectDid)
	}

	actorDid := syntax.DID(pull.OwnerDid)
	eventType := models.NotificationTypePullCreated
	entityType := "pull"
	entityId := pull.AtUri().String()
	repoId := &repo.Id
	var issueId *int64
	p := int64(pull.ID)
	pullId := &p

	n.notifyEvent(
		ctx,
		actorDid,
		recipients,
		eventType,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
}

func (n *databaseNotifier) NewPullComment(ctx context.Context, comment *models.PullComment, mentions []syntax.DID) {
	l := log.FromContext(ctx)

	pull, err := db.GetPull(n.db,
		syntax.ATURI(comment.RepoAt),
		comment.PullId,
	)
	if err != nil {
		l.Error("failed to get pulls", "err", err)
		return
	}

	repo, err := db.GetRepo(n.db, orm.FilterEq("at_uri", comment.RepoAt))
	if err != nil {
		l.Error("failed to get repos", "err", err)
		return
	}

	// build up the recipients list:
	// - repo owner
	// - all pull participants
	// - remove those already mentioned
	recipients := sets.Singleton(syntax.DID(repo.Did))
	for _, p := range pull.Participants() {
		recipients.Insert(syntax.DID(p))
	}
	for _, m := range mentions {
		recipients.Remove(m)
	}

	actorDid := syntax.DID(comment.OwnerDid)
	eventType := models.NotificationTypePullCommented
	entityType := "pull"
	entityId := pull.AtUri().String()
	repoId := &repo.Id
	var issueId *int64
	p := int64(pull.ID)
	pullId := &p

	n.notifyEvent(
		ctx,
		actorDid,
		recipients,
		eventType,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
	n.notifyEvent(
		ctx,
		actorDid,
		sets.Collect(slices.Values(mentions)),
		models.NotificationTypeUserMentioned,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
}

func (n *databaseNotifier) UpdateProfile(ctx context.Context, profile *models.Profile) {
	// no-op
}

func (n *databaseNotifier) DeleteString(ctx context.Context, did, rkey string) {
	// no-op
}

func (n *databaseNotifier) EditString(ctx context.Context, string *models.String) {
	// no-op
}

func (n *databaseNotifier) NewString(ctx context.Context, string *models.String) {
	// no-op
}

func (n *databaseNotifier) Push(ctx context.Context, repo *models.Repo, ref, oldSha, newSha, committerDid string) {
	// no-op for now; webhooks are handled by the webhook notifier
}

func (n *databaseNotifier) NewIssueState(ctx context.Context, actor syntax.DID, issue *models.Issue) {
	l := log.FromContext(ctx)

	collaborators, err := db.GetCollaborators(n.db, orm.FilterEq("repo_at", issue.Repo.RepoAt()))
	if err != nil {
		l.Error("failed to fetch collaborators", "err", err)
		return
	}

	// build up the recipients list:
	// - repo owner
	// - repo collaborators
	// - all issue participants
	recipients := sets.Singleton(syntax.DID(issue.Repo.Did))
	for _, c := range collaborators {
		recipients.Insert(c.SubjectDid)
	}
	for _, p := range issue.Participants() {
		recipients.Insert(syntax.DID(p))
	}

	entityType := "issue"
	entityId := issue.AtUri().String()
	repoId := &issue.Repo.Id
	issueId := &issue.Id
	var pullId *int64
	var eventType models.NotificationType

	if issue.Open {
		eventType = models.NotificationTypeIssueReopen
	} else {
		eventType = models.NotificationTypeIssueClosed
	}

	n.notifyEvent(
		ctx,
		actor,
		recipients,
		eventType,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
}

func (n *databaseNotifier) NewPullState(ctx context.Context, actor syntax.DID, pull *models.Pull) {
	l := log.FromContext(ctx)

	// Get repo details
	repo, err := db.GetRepo(n.db, orm.FilterEq("at_uri", string(pull.RepoAt)))
	if err != nil {
		l.Error("failed to get repos", "err", err)
		return
	}

	collaborators, err := db.GetCollaborators(n.db, orm.FilterEq("repo_at", repo.RepoAt()))
	if err != nil {
		l.Error("failed to fetch collaborators", "err", err)
		return
	}

	// build up the recipients list:
	// - repo owner
	// - all pull participants
	recipients := sets.Singleton(syntax.DID(repo.Did))
	for _, c := range collaborators {
		recipients.Insert(c.SubjectDid)
	}
	for _, p := range pull.Participants() {
		recipients.Insert(syntax.DID(p))
	}

	entityType := "pull"
	entityId := pull.AtUri().String()
	repoId := &repo.Id
	var issueId *int64
	var eventType models.NotificationType
	switch pull.State {
	case models.PullClosed:
		eventType = models.NotificationTypePullClosed
	case models.PullOpen:
		eventType = models.NotificationTypePullReopen
	case models.PullMerged:
		eventType = models.NotificationTypePullMerged
	default:
		l.Error("unexpected new PR state", "state", pull.State)
		return
	}
	p := int64(pull.ID)
	pullId := &p

	n.notifyEvent(
		ctx,
		actor,
		recipients,
		eventType,
		entityType,
		entityId,
		repoId,
		issueId,
		pullId,
	)
}

func (n *databaseNotifier) notifyEvent(
	ctx context.Context,
	actorDid syntax.DID,
	recipients sets.Set[syntax.DID],
	eventType models.NotificationType,
	entityType string,
	entityId string,
	repoId *int64,
	issueId *int64,
	pullId *int64,
) {
	l := log.FromContext(ctx)

	// if the user is attempting to mention >maxMentions users, this is probably spam, do not mention anybody
	if eventType == models.NotificationTypeUserMentioned && recipients.Len() > maxMentions {
		return
	}

	recipients.Remove(actorDid)

	prefMap, err := db.GetNotificationPreferences(
		n.db,
		orm.FilterIn("user_did", slices.Collect(recipients.All())),
	)
	if err != nil {
		// failed to get prefs for users
		return
	}

	// create a transaction for bulk notification storage
	tx, err := n.db.Begin()
	if err != nil {
		// failed to start tx
		return
	}
	defer tx.Rollback()

	// filter based on preferences
	for recipientDid := range recipients.All() {
		prefs, ok := prefMap[recipientDid]
		if !ok {
			prefs = models.DefaultNotificationPreferences(recipientDid)
		}

		// skip users who don’t want this type
		if !prefs.ShouldNotify(eventType) {
			continue
		}

		// create notification
		notif := &models.Notification{
			RecipientDid: recipientDid.String(),
			ActorDid:     actorDid.String(),
			Type:         eventType,
			EntityType:   entityType,
			EntityId:     entityId,
			RepoId:       repoId,
			IssueId:      issueId,
			PullId:       pullId,
		}

		if err := db.CreateNotification(tx, notif); err != nil {
			l.Error("failed to create notification", "recipientDid", recipientDid, "err", err)
		}
	}

	if err := tx.Commit(); err != nil {
		// failed to commit
		return
	}
}
