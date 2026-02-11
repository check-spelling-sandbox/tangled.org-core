package issues

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	atpclient "github.com/bluesky-social/indigo/atproto/client"
	"github.com/bluesky-social/indigo/atproto/syntax"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/go-chi/chi/v5"

	"tangled.org/core/api/tangled"
	"tangled.org/core/appview/config"
	"tangled.org/core/appview/db"
	issues_indexer "tangled.org/core/appview/indexer/issues"
	"tangled.org/core/appview/mentions"
	"tangled.org/core/appview/models"
	"tangled.org/core/appview/notify"
	"tangled.org/core/appview/oauth"
	"tangled.org/core/appview/pages"
	"tangled.org/core/appview/pages/repoinfo"
	"tangled.org/core/appview/pagination"
	"tangled.org/core/appview/reporesolver"
	"tangled.org/core/appview/searchquery"
	"tangled.org/core/appview/validator"
	"tangled.org/core/idresolver"
	"tangled.org/core/orm"
	"tangled.org/core/rbac"
	"tangled.org/core/tid"
)

type Issues struct {
	oauth            *oauth.OAuth
	repoResolver     *reporesolver.RepoResolver
	enforcer         *rbac.Enforcer
	pages            *pages.Pages
	idResolver       *idresolver.Resolver
	mentionsResolver *mentions.Resolver
	db               *db.DB
	config           *config.Config
	notifier         notify.Notifier
	logger           *slog.Logger
	validator        *validator.Validator
	indexer          *issues_indexer.Indexer
}

func New(
	oauth *oauth.OAuth,
	repoResolver *reporesolver.RepoResolver,
	enforcer *rbac.Enforcer,
	pages *pages.Pages,
	idResolver *idresolver.Resolver,
	mentionsResolver *mentions.Resolver,
	db *db.DB,
	config *config.Config,
	notifier notify.Notifier,
	validator *validator.Validator,
	indexer *issues_indexer.Indexer,
	logger *slog.Logger,
) *Issues {
	return &Issues{
		oauth:            oauth,
		repoResolver:     repoResolver,
		enforcer:         enforcer,
		pages:            pages,
		idResolver:       idResolver,
		mentionsResolver: mentionsResolver,
		db:               db,
		config:           config,
		notifier:         notifier,
		logger:           logger,
		validator:        validator,
		indexer:          indexer,
	}
}

func (rp *Issues) RepoSingleIssue(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "RepoSingleIssue")
	user := rp.oauth.GetMultiAccountUser(r)
	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		return
	}

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Error404(w)
		return
	}

	reactionMap, err := db.GetReactionMap(rp.db, 20, issue.AtUri())
	if err != nil {
		l.Error("failed to get issue reactions", "err", err)
	}

	userReactions := map[models.ReactionKind]bool{}
	if user != nil {
		userReactions = db.GetReactionStatusMap(rp.db, user.Active.Did, issue.AtUri())
	}

	backlinks, err := db.GetBacklinks(rp.db, issue.AtUri())
	if err != nil {
		l.Error("failed to fetch backlinks", "err", err)
		rp.pages.Error503(w)
		return
	}

	labelDefs, err := db.GetLabelDefinitions(
		rp.db,
		orm.FilterIn("at_uri", f.Labels),
		orm.FilterContains("scope", tangled.RepoIssueNSID),
	)
	if err != nil {
		l.Error("failed to fetch labels", "err", err)
		rp.pages.Error503(w)
		return
	}

	defs := make(map[string]*models.LabelDefinition)
	for _, l := range labelDefs {
		defs[l.AtUri().String()] = &l
	}

	rp.pages.RepoSingleIssue(w, pages.RepoSingleIssueParams{
		LoggedInUser: user,
		RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
		Issue:        issue,
		CommentList:  issue.CommentList(),
		Backlinks:    backlinks,
		Reactions:    reactionMap,
		UserReacted:  userReactions,
		LabelDefs:    defs,
	})
}

func (rp *Issues) EditIssue(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "EditIssue")
	user := rp.oauth.GetMultiAccountUser(r)

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Error404(w)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rp.pages.EditIssueFragment(w, pages.EditIssueParams{
			LoggedInUser: user,
			RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
			Issue:        issue,
		})
	case http.MethodPost:
		noticeId := "issues"
		newIssue := issue
		newIssue.Title = r.FormValue("title")
		newIssue.Body = r.FormValue("body")
		newIssue.Mentions, newIssue.References = rp.mentionsResolver.Resolve(r.Context(), newIssue.Body)

		if err := rp.validator.ValidateIssue(newIssue); err != nil {
			l.Error("validation error", "err", err)
			rp.pages.Notice(w, noticeId, fmt.Sprintf("Failed to edit issue: %s", err))
			return
		}

		newRecord := newIssue.AsRecord()

		// edit an atproto record
		client, err := rp.oauth.AuthorizedClient(r)
		if err != nil {
			l.Error("failed to get authorized client", "err", err)
			rp.pages.Notice(w, noticeId, "Failed to edit issue.")
			return
		}

		ex, err := comatproto.RepoGetRecord(r.Context(), client, "", tangled.RepoIssueNSID, user.Active.Did, newIssue.Rkey)
		if err != nil {
			l.Error("failed to get record", "err", err)
			rp.pages.Notice(w, noticeId, "Failed to edit issue, no record found on PDS.")
			return
		}

		_, err = comatproto.RepoPutRecord(r.Context(), client, &comatproto.RepoPutRecord_Input{
			Collection: tangled.RepoIssueNSID,
			Repo:       user.Active.Did,
			Rkey:       newIssue.Rkey,
			SwapRecord: ex.Cid,
			Record: &lexutil.LexiconTypeDecoder{
				Val: &newRecord,
			},
		})
		if err != nil {
			l.Error("failed to edit record on PDS", "err", err)
			rp.pages.Notice(w, noticeId, "Failed to edit issue on PDS.")
			return
		}

		// modify on DB -- TODO: transact this cleverly
		tx, err := rp.db.Begin()
		if err != nil {
			l.Error("failed to edit issue on DB", "err", err)
			rp.pages.Notice(w, noticeId, "Failed to edit issue.")
			return
		}
		defer tx.Rollback()

		err = db.PutIssue(tx, newIssue)
		if err != nil {
			l.Error("failed to edit issue", "err", err)
			rp.pages.Notice(w, "issues", "Failed to edit issue.")
			return
		}

		if err = tx.Commit(); err != nil {
			l.Error("failed to edit issue", "err", err)
			rp.pages.Notice(w, "issues", "Failed to cedit issue.")
			return
		}

		rp.pages.HxRefresh(w)
	}
}

func (rp *Issues) DeleteIssue(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "DeleteIssue")
	noticeId := "issue-actions-error"

	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		return
	}

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Notice(w, noticeId, "Failed to delete issue.")
		return
	}
	l = l.With("did", issue.Did, "rkey", issue.Rkey)

	tx, err := rp.db.Begin()
	if err != nil {
		l.Error("failed to start transaction", "err", err)
		rp.pages.Notice(w, "issue-comment", "Failed to create comment, try again later.")
		return
	}
	defer tx.Rollback()

	// delete from PDS
	client, err := rp.oauth.AuthorizedClient(r)
	if err != nil {
		l.Error("failed to get authorized client", "err", err)
		rp.pages.Notice(w, "issue-comment", "Failed to delete comment.")
		return
	}
	_, err = comatproto.RepoDeleteRecord(r.Context(), client, &comatproto.RepoDeleteRecord_Input{
		Collection: tangled.RepoIssueNSID,
		Repo:       issue.Did,
		Rkey:       issue.Rkey,
	})
	if err != nil {
		// TODO: transact this better
		l.Error("failed to delete issue from PDS", "err", err)
		rp.pages.Notice(w, noticeId, "Failed to delete issue.")
		return
	}

	// delete from db
	if err := db.DeleteIssues(tx, issue.Did, issue.Rkey); err != nil {
		l.Error("failed to delete issue", "err", err)
		rp.pages.Notice(w, noticeId, "Failed to delete issue.")
		return
	}
	tx.Commit()

	rp.notifier.DeleteIssue(r.Context(), issue)

	// return to all issues page
	ownerSlashRepo := reporesolver.GetBaseRepoPath(r, f)
	rp.pages.HxRedirect(w, "/"+ownerSlashRepo+"/issues")
}

func (rp *Issues) CloseIssue(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "CloseIssue")
	user := rp.oauth.GetMultiAccountUser(r)
	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		return
	}

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Error404(w)
		return
	}

	roles := repoinfo.RolesInRepo{Roles: rp.enforcer.GetPermissionsInRepo(user.Active.Did, f.Knot, f.DidSlashRepo())}
	isRepoOwner := roles.IsOwner()
	isCollaborator := roles.IsCollaborator()
	isIssueOwner := user.Active.Did == issue.Did

	// TODO: make this more granular
	if isIssueOwner || isRepoOwner || isCollaborator {
		err = db.CloseIssues(
			rp.db,
			orm.FilterEq("id", issue.Id),
		)
		if err != nil {
			l.Error("failed to close issue", "err", err)
			rp.pages.Notice(w, "issue-action", "Failed to close issue. Try again later.")
			return
		}
		// change the issue state (this will pass down to the notifiers)
		issue.Open = false

		// notify about the issue closure
		rp.notifier.NewIssueState(r.Context(), syntax.DID(user.Active.Did), issue)

		ownerSlashRepo := reporesolver.GetBaseRepoPath(r, f)
		rp.pages.HxLocation(w, fmt.Sprintf("/%s/issues/%d", ownerSlashRepo, issue.IssueId))
		return
	} else {
		l.Error("user is not permitted to close issue")
		http.Error(w, "for biden", http.StatusUnauthorized)
		return
	}
}

func (rp *Issues) ReopenIssue(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "ReopenIssue")
	user := rp.oauth.GetMultiAccountUser(r)
	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		return
	}

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Error404(w)
		return
	}

	roles := repoinfo.RolesInRepo{Roles: rp.enforcer.GetPermissionsInRepo(user.Active.Did, f.Knot, f.DidSlashRepo())}
	isRepoOwner := roles.IsOwner()
	isCollaborator := roles.IsCollaborator()
	isIssueOwner := user.Active.Did == issue.Did

	if isCollaborator || isRepoOwner || isIssueOwner {
		err := db.ReopenIssues(
			rp.db,
			orm.FilterEq("id", issue.Id),
		)
		if err != nil {
			l.Error("failed to reopen issue", "err", err)
			rp.pages.Notice(w, "issue-action", "Failed to reopen issue. Try again later.")
			return
		}
		// change the issue state (this will pass down to the notifiers)
		issue.Open = true

		// notify about the issue reopen
		rp.notifier.NewIssueState(r.Context(), syntax.DID(user.Active.Did), issue)

		ownerSlashRepo := reporesolver.GetBaseRepoPath(r, f)
		rp.pages.HxLocation(w, fmt.Sprintf("/%s/issues/%d", ownerSlashRepo, issue.IssueId))
		return
	} else {
		l.Error("user is not the owner of the repo")
		http.Error(w, "forbidden", http.StatusUnauthorized)
		return
	}
}

func (rp *Issues) NewIssueComment(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "NewIssueComment")
	user := rp.oauth.GetMultiAccountUser(r)
	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		return
	}

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Error404(w)
		return
	}

	body := r.FormValue("body")
	if body == "" {
		rp.pages.Notice(w, "issue", "Body is required")
		return
	}

	replyToUri := r.FormValue("reply-to")
	var replyTo *string
	if replyToUri != "" {
		replyTo = &replyToUri
	}

	mentions, references := rp.mentionsResolver.Resolve(r.Context(), body)

	comment := models.IssueComment{
		Did:        user.Active.Did,
		Rkey:       tid.TID(),
		IssueAt:    issue.AtUri().String(),
		ReplyTo:    replyTo,
		Body:       body,
		Created:    time.Now(),
		Mentions:   mentions,
		References: references,
	}
	if err = rp.validator.ValidateIssueComment(&comment); err != nil {
		l.Error("failed to validate comment", "err", err)
		rp.pages.Notice(w, "issue-comment", "Failed to create comment.")
		return
	}
	record := comment.AsRecord()

	client, err := rp.oauth.AuthorizedClient(r)
	if err != nil {
		l.Error("failed to get authorized client", "err", err)
		rp.pages.Notice(w, "issue-comment", "Failed to create comment.")
		return
	}

	// create a record first
	resp, err := comatproto.RepoPutRecord(r.Context(), client, &comatproto.RepoPutRecord_Input{
		Collection: tangled.RepoIssueCommentNSID,
		Repo:       comment.Did,
		Rkey:       comment.Rkey,
		Record: &lexutil.LexiconTypeDecoder{
			Val: &record,
		},
	})
	if err != nil {
		l.Error("failed to create comment", "err", err)
		rp.pages.Notice(w, "issue-comment", "Failed to create comment.")
		return
	}
	atUri := resp.Uri
	defer func() {
		if err := rollbackRecord(context.Background(), atUri, client); err != nil {
			l.Error("rollback failed", "err", err)
		}
	}()

	tx, err := rp.db.Begin()
	if err != nil {
		l.Error("failed to start transaction", "err", err)
		rp.pages.Notice(w, "issue-comment", "Failed to create comment, try again later.")
		return
	}
	defer tx.Rollback()

	commentId, err := db.AddIssueComment(tx, comment)
	if err != nil {
		l.Error("failed to create comment", "err", err)
		rp.pages.Notice(w, "issue-comment", "Failed to create comment.")
		return
	}
	err = tx.Commit()
	if err != nil {
		l.Error("failed to commit transaction", "err", err)
		rp.pages.Notice(w, "issue-comment", "Failed to create comment, try again later.")
		return
	}

	// reset atUri to make rollback a no-op
	atUri = ""

	// notify about the new comment
	comment.Id = commentId

	rp.notifier.NewIssueComment(r.Context(), &comment, mentions)

	ownerSlashRepo := reporesolver.GetBaseRepoPath(r, f)
	rp.pages.HxLocation(w, fmt.Sprintf("/%s/issues/%d#comment-%d", ownerSlashRepo, issue.IssueId, commentId))
}

func (rp *Issues) IssueComment(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "IssueComment")
	user := rp.oauth.GetMultiAccountUser(r)

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Error404(w)
		return
	}

	commentId := chi.URLParam(r, "commentId")
	comments, err := db.GetIssueComments(
		rp.db,
		orm.FilterEq("id", commentId),
	)
	if err != nil {
		l.Error("failed to fetch comment", "id", commentId)
		http.Error(w, "failed to fetch comment id", http.StatusBadRequest)
		return
	}
	if len(comments) != 1 {
		l.Error("incorrect number of comments returned", "id", commentId, "len(comments)", len(comments))
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}
	comment := comments[0]

	rp.pages.IssueCommentBodyFragment(w, pages.IssueCommentBodyParams{
		LoggedInUser: user,
		RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
		Issue:        issue,
		Comment:      &comment,
	})
}

func (rp *Issues) EditIssueComment(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "EditIssueComment")
	user := rp.oauth.GetMultiAccountUser(r)

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Error404(w)
		return
	}

	commentId := chi.URLParam(r, "commentId")
	comments, err := db.GetIssueComments(
		rp.db,
		orm.FilterEq("id", commentId),
	)
	if err != nil {
		l.Error("failed to fetch comment", "id", commentId)
		http.Error(w, "failed to fetch comment id", http.StatusBadRequest)
		return
	}
	if len(comments) != 1 {
		l.Error("incorrect number of comments returned", "id", commentId, "len(comments)", len(comments))
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}
	comment := comments[0]

	if comment.Did != user.Active.Did {
		l.Error("unauthorized comment edit", "expectedDid", comment.Did, "gotDid", user.Active.Did)
		http.Error(w, "you are not the author of this comment", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rp.pages.EditIssueCommentFragment(w, pages.EditIssueCommentParams{
			LoggedInUser: user,
			RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
			Issue:        issue,
			Comment:      &comment,
		})
	case http.MethodPost:
		// extract form value
		newBody := r.FormValue("body")
		client, err := rp.oauth.AuthorizedClient(r)
		if err != nil {
			l.Error("failed to get authorized client", "err", err)
			rp.pages.Notice(w, "issue-comment", "Failed to create comment.")
			return
		}

		now := time.Now()
		newComment := comment
		newComment.Body = newBody
		newComment.Edited = &now
		newComment.Mentions, newComment.References = rp.mentionsResolver.Resolve(r.Context(), newBody)

		record := newComment.AsRecord()

		tx, err := rp.db.Begin()
		if err != nil {
			l.Error("failed to start transaction", "err", err)
			rp.pages.Notice(w, "repo-notice", "Failed to update description, try again later.")
			return
		}
		defer tx.Rollback()

		_, err = db.AddIssueComment(tx, newComment)
		if err != nil {
			l.Error("failed to perferom update-description query", "err", err)
			rp.pages.Notice(w, "repo-notice", "Failed to update description, try again later.")
			return
		}
		tx.Commit()

		// rkey is optional, it was introduced later
		if newComment.Rkey != "" {
			// update the record on pds
			ex, err := comatproto.RepoGetRecord(r.Context(), client, "", tangled.RepoIssueCommentNSID, user.Active.Did, comment.Rkey)
			if err != nil {
				l.Error("failed to get record", "err", err, "did", newComment.Did, "rkey", newComment.Rkey)
				rp.pages.Notice(w, fmt.Sprintf("comment-%s-status", commentId), "Failed to update description, no record found on PDS.")
				return
			}

			_, err = comatproto.RepoPutRecord(r.Context(), client, &comatproto.RepoPutRecord_Input{
				Collection: tangled.RepoIssueCommentNSID,
				Repo:       user.Active.Did,
				Rkey:       newComment.Rkey,
				SwapRecord: ex.Cid,
				Record: &lexutil.LexiconTypeDecoder{
					Val: &record,
				},
			})
			if err != nil {
				l.Error("failed to update record on PDS", "err", err)
			}
		}

		// return new comment body with htmx
		rp.pages.IssueCommentBodyFragment(w, pages.IssueCommentBodyParams{
			LoggedInUser: user,
			RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
			Issue:        issue,
			Comment:      &newComment,
		})
	}
}

func (rp *Issues) ReplyIssueCommentPlaceholder(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "ReplyIssueCommentPlaceholder")
	user := rp.oauth.GetMultiAccountUser(r)

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Error404(w)
		return
	}

	commentId := chi.URLParam(r, "commentId")
	comments, err := db.GetIssueComments(
		rp.db,
		orm.FilterEq("id", commentId),
	)
	if err != nil {
		l.Error("failed to fetch comment", "id", commentId)
		http.Error(w, "failed to fetch comment id", http.StatusBadRequest)
		return
	}
	if len(comments) != 1 {
		l.Error("incorrect number of comments returned", "id", commentId, "len(comments)", len(comments))
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}
	comment := comments[0]

	rp.pages.ReplyIssueCommentPlaceholderFragment(w, pages.ReplyIssueCommentPlaceholderParams{
		LoggedInUser: user,
		RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
		Issue:        issue,
		Comment:      &comment,
	})
}

func (rp *Issues) ReplyIssueComment(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "ReplyIssueComment")
	user := rp.oauth.GetMultiAccountUser(r)

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Error404(w)
		return
	}

	commentId := chi.URLParam(r, "commentId")
	comments, err := db.GetIssueComments(
		rp.db,
		orm.FilterEq("id", commentId),
	)
	if err != nil {
		l.Error("failed to fetch comment", "id", commentId)
		http.Error(w, "failed to fetch comment id", http.StatusBadRequest)
		return
	}
	if len(comments) != 1 {
		l.Error("incorrect number of comments returned", "id", commentId, "len(comments)", len(comments))
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}
	comment := comments[0]

	rp.pages.ReplyIssueCommentFragment(w, pages.ReplyIssueCommentParams{
		LoggedInUser: user,
		RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
		Issue:        issue,
		Comment:      &comment,
	})
}

func (rp *Issues) DeleteIssueComment(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "DeleteIssueComment")
	user := rp.oauth.GetMultiAccountUser(r)

	issue, ok := r.Context().Value("issue").(*models.Issue)
	if !ok {
		l.Error("failed to get issue")
		rp.pages.Error404(w)
		return
	}

	commentId := chi.URLParam(r, "commentId")
	comments, err := db.GetIssueComments(
		rp.db,
		orm.FilterEq("id", commentId),
	)
	if err != nil {
		l.Error("failed to fetch comment", "id", commentId)
		http.Error(w, "failed to fetch comment id", http.StatusBadRequest)
		return
	}
	if len(comments) != 1 {
		l.Error("incorrect number of comments returned", "id", commentId, "len(comments)", len(comments))
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}
	comment := comments[0]

	if comment.Did != user.Active.Did {
		l.Error("unauthorized action", "expectedDid", comment.Did, "gotDid", user.Active.Did)
		http.Error(w, "you are not the author of this comment", http.StatusUnauthorized)
		return
	}

	if comment.Deleted != nil {
		http.Error(w, "comment already deleted", http.StatusBadRequest)
		return
	}

	// optimistic deletion
	deleted := time.Now()
	err = db.DeleteIssueComments(rp.db, orm.FilterEq("id", comment.Id))
	if err != nil {
		l.Error("failed to delete comment", "err", err)
		rp.pages.Notice(w, fmt.Sprintf("comment-%s-status", commentId), "failed to delete comment")
		return
	}

	// delete from pds
	if comment.Rkey != "" {
		client, err := rp.oauth.AuthorizedClient(r)
		if err != nil {
			l.Error("failed to get authorized client", "err", err)
			rp.pages.Notice(w, "issue-comment", "Failed to delete comment.")
			return
		}
		_, err = comatproto.RepoDeleteRecord(r.Context(), client, &comatproto.RepoDeleteRecord_Input{
			Collection: tangled.RepoIssueCommentNSID,
			Repo:       user.Active.Did,
			Rkey:       comment.Rkey,
		})
		if err != nil {
			l.Error("failed to delete from PDS", "err", err)
		}
	}

	// optimistic update for htmx
	comment.Body = ""
	comment.Deleted = &deleted

	// htmx fragment of comment after deletion
	rp.pages.IssueCommentBodyFragment(w, pages.IssueCommentBodyParams{
		LoggedInUser: user,
		RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
		Issue:        issue,
		Comment:      &comment,
	})
}

func (rp *Issues) RepoIssues(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "RepoIssues")

	params := r.URL.Query()
	page := pagination.FromContext(r.Context())

	user := rp.oauth.GetMultiAccountUser(r)
	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		return
	}

	query := searchquery.Parse(params.Get("q"))

	var isOpen *bool
	if urlState := params.Get("state"); urlState != "" {
		switch urlState {
		case "open":
			isOpen = ptrBool(true)
		case "closed":
			isOpen = ptrBool(false)
		}
		query.Set("state", urlState)
	} else if queryState := query.Get("state"); queryState != nil {
		switch *queryState {
		case "open":
			isOpen = ptrBool(true)
		case "closed":
			isOpen = ptrBool(false)
		}
	} else if _, hasQ := params["q"]; !hasQ {
		// no q param at all -- default to open
		isOpen = ptrBool(true)
		query.Set("state", "open")
	}

	var authorDid string
	if authorHandle := query.Get("author"); authorHandle != nil {
		identity, err := rp.idResolver.ResolveIdent(r.Context(), *authorHandle)
		if err != nil {
			l.Debug("failed to resolve author handle", "handle", *authorHandle, "err", err)
		} else {
			authorDid = identity.DID.String()
		}
	}

	var negatedAuthorDid string
	if negatedAuthors := query.GetAllNegated("author"); len(negatedAuthors) > 0 {
		identity, err := rp.idResolver.ResolveIdent(r.Context(), negatedAuthors[0])
		if err != nil {
			l.Debug("failed to resolve negated author handle", "handle", negatedAuthors[0], "err", err)
		} else {
			negatedAuthorDid = identity.DID.String()
		}
	}

	labels := query.GetAll("label")
	negatedLabels := query.GetAllNegated("label")

	var keywords, negatedKeywords []string
	var phrases, negatedPhrases []string
	for _, item := range query.Items() {
		switch item.Kind {
		case searchquery.KindKeyword:
			if item.Negated {
				negatedKeywords = append(negatedKeywords, item.Value)
			} else {
				keywords = append(keywords, item.Value)
			}
		case searchquery.KindQuoted:
			if item.Negated {
				negatedPhrases = append(negatedPhrases, item.Value)
			} else {
				phrases = append(phrases, item.Value)
			}
		}
	}

	searchOpts := models.IssueSearchOptions{
		Keywords:         keywords,
		Phrases:          phrases,
		RepoAt:           f.RepoAt().String(),
		IsOpen:           isOpen,
		AuthorDid:        authorDid,
		Labels:           labels,
		NegatedKeywords:  negatedKeywords,
		NegatedPhrases:   negatedPhrases,
		NegatedLabels:    negatedLabels,
		NegatedAuthorDid: negatedAuthorDid,
		Page:             page,
	}

	totalIssues := 0
	if isOpen == nil {
		totalIssues = f.RepoStats.IssueCount.Open + f.RepoStats.IssueCount.Closed
	} else if *isOpen {
		totalIssues = f.RepoStats.IssueCount.Open
	} else {
		totalIssues = f.RepoStats.IssueCount.Closed
	}

	var issues []models.Issue

	if searchOpts.HasSearchFilters() {
		res, err := rp.indexer.Search(r.Context(), searchOpts)
		if err != nil {
			l.Error("failed to search for issues", "err", err)
			return
		}
		l.Debug("searched issues with indexer", "count", len(res.Hits))
		totalIssues = int(res.Total)

		if len(res.Hits) > 0 {
			issues, err = db.GetIssues(
				rp.db,
				orm.FilterIn("id", res.Hits),
			)
			if err != nil {
				l.Error("failed to get issues", "err", err)
				rp.pages.Notice(w, "issues", "Failed to load issues. Try again later.")
				return
			}
		}
	} else {
		filters := []orm.Filter{
			orm.FilterEq("repo_at", f.RepoAt()),
		}
		if isOpen != nil {
			openInt := 0
			if *isOpen {
				openInt = 1
			}
			filters = append(filters, orm.FilterEq("open", openInt))
		}
		issues, err = db.GetIssuesPaginated(
			rp.db,
			page,
			filters...,
		)
		if err != nil {
			l.Error("failed to get issues", "err", err)
			rp.pages.Notice(w, "issues", "Failed to load issues. Try again later.")
			return
		}
	}

	labelDefs, err := db.GetLabelDefinitions(
		rp.db,
		orm.FilterIn("at_uri", f.Labels),
		orm.FilterContains("scope", tangled.RepoIssueNSID),
	)
	if err != nil {
		l.Error("failed to fetch labels", "err", err)
		rp.pages.Error503(w)
		return
	}

	defs := make(map[string]*models.LabelDefinition)
	for _, l := range labelDefs {
		defs[l.AtUri().String()] = &l
	}

	filterState := ""
	if isOpen != nil {
		if *isOpen {
			filterState = "open"
		} else {
			filterState = "closed"
		}
	}

	rp.pages.RepoIssues(w, pages.RepoIssuesParams{
		LoggedInUser: rp.oauth.GetMultiAccountUser(r),
		RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
		Issues:       issues,
		IssueCount:   totalIssues,
		LabelDefs:    defs,
		FilterState:  filterState,
		FilterQuery:  query.String(),
		Page:         page,
	})
}

func ptrBool(b bool) *bool { return &b }

func (rp *Issues) NewIssue(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "NewIssue")
	user := rp.oauth.GetMultiAccountUser(r)

	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rp.pages.RepoNewIssue(w, pages.RepoNewIssueParams{
			LoggedInUser: user,
			RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
		})
	case http.MethodPost:
		body := r.FormValue("body")
		mentions, references := rp.mentionsResolver.Resolve(r.Context(), body)

		issue := &models.Issue{
			RepoAt:     f.RepoAt(),
			Rkey:       tid.TID(),
			Title:      r.FormValue("title"),
			Body:       body,
			Open:       true,
			Did:        user.Active.Did,
			Created:    time.Now(),
			Mentions:   mentions,
			References: references,
			Repo:       f,
		}

		if err := rp.validator.ValidateIssue(issue); err != nil {
			l.Error("validation error", "err", err)
			rp.pages.Notice(w, "issues", fmt.Sprintf("Failed to create issue: %s", err))
			return
		}

		record := issue.AsRecord()

		// create an atproto record
		client, err := rp.oauth.AuthorizedClient(r)
		if err != nil {
			l.Error("failed to get authorized client", "err", err)
			rp.pages.Notice(w, "issues", "Failed to create issue.")
			return
		}
		resp, err := comatproto.RepoPutRecord(r.Context(), client, &comatproto.RepoPutRecord_Input{
			Collection: tangled.RepoIssueNSID,
			Repo:       user.Active.Did,
			Rkey:       issue.Rkey,
			Record: &lexutil.LexiconTypeDecoder{
				Val: &record,
			},
		})
		if err != nil {
			l.Error("failed to create issue", "err", err)
			rp.pages.Notice(w, "issues", "Failed to create issue.")
			return
		}
		atUri := resp.Uri

		tx, err := rp.db.BeginTx(r.Context(), nil)
		if err != nil {
			rp.pages.Notice(w, "issues", "Failed to create issue, try again later")
			return
		}
		rollback := func() {
			err1 := tx.Rollback()
			err2 := rollbackRecord(context.Background(), atUri, client)

			if errors.Is(err1, sql.ErrTxDone) {
				err1 = nil
			}

			if err := errors.Join(err1, err2); err != nil {
				l.Error("failed to rollback txn", "err", err)
			}
		}
		defer rollback()

		err = db.PutIssue(tx, issue)
		if err != nil {
			l.Error("failed to create issue", "err", err)
			rp.pages.Notice(w, "issues", "Failed to create issue.")
			return
		}

		if err = tx.Commit(); err != nil {
			l.Error("failed to create issue", "err", err)
			rp.pages.Notice(w, "issues", "Failed to create issue.")
			return
		}

		// everything is successful, do not rollback the atproto record
		atUri = ""

		rp.notifier.NewIssue(r.Context(), issue, mentions)

		ownerSlashRepo := reporesolver.GetBaseRepoPath(r, f)
		rp.pages.HxLocation(w, fmt.Sprintf("/%s/issues/%d", ownerSlashRepo, issue.IssueId))
		return
	}
}

// this is used to rollback changes made to the PDS
//
// it is a no-op if the provided ATURI is empty
func rollbackRecord(ctx context.Context, aturi string, client *atpclient.APIClient) error {
	if aturi == "" {
		return nil
	}

	parsed := syntax.ATURI(aturi)

	collection := parsed.Collection().String()
	repo := parsed.Authority().String()
	rkey := parsed.RecordKey().String()

	_, err := comatproto.RepoDeleteRecord(ctx, client, &comatproto.RepoDeleteRecord_Input{
		Collection: collection,
		Repo:       repo,
		Rkey:       rkey,
	})
	return err
}
