package xrpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	securejoin "github.com/cyphar/filepath-securejoin"
	"tangled.org/core/api/tangled"
	"tangled.org/core/rbac"
	"tangled.org/core/spindle/models"
	xrpcerr "tangled.org/core/xrpc/errors"
)

func (x *Xrpc) CancelPipeline(w http.ResponseWriter, r *http.Request) {
	l := x.Logger
	fail := func(e xrpcerr.XrpcError) {
		l.Error("failed", "kind", e.Tag, "error", e.Message)
		writeError(w, e, http.StatusBadRequest)
	}
	l.Debug("cancel pipeline")

	actorDid, ok := r.Context().Value(ActorDid).(syntax.DID)
	if !ok {
		fail(xrpcerr.MissingActorDidError)
		return
	}

	var input tangled.PipelineCancelPipeline_Input
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		fail(xrpcerr.GenericError(err))
		return
	}

	aturi := syntax.ATURI(input.Pipeline)
	wid := models.WorkflowId{
		PipelineId: models.PipelineId{
			Knot: strings.TrimPrefix(aturi.Authority().String(), "did:web:"),
			Rkey: aturi.RecordKey().String(),
		},
		Name: input.Workflow,
	}
	l.Debug("cancel pipeline", "wid", wid)

	// unfortunately we have to resolve repo-at here
	repoAt, err := syntax.ParseATURI(input.Repo)
	if err != nil {
		fail(xrpcerr.InvalidRepoError(input.Repo))
		return
	}

	ident, err := x.Resolver.ResolveIdent(r.Context(), repoAt.Authority().String())
	if err != nil || ident.Handle.IsInvalidHandle() {
		fail(xrpcerr.GenericError(fmt.Errorf("failed to resolve handle: %w", err)))
		return
	}

	xrpcc := xrpc.Client{Host: ident.PDSEndpoint()}
	resp, err := atproto.RepoGetRecord(r.Context(), &xrpcc, "", tangled.RepoNSID, repoAt.Authority().String(), repoAt.RecordKey().String())
	if err != nil {
		fail(xrpcerr.GenericError(err))
		return
	}

	repo := resp.Value.Val.(*tangled.Repo)
	didSlashRepo, err := securejoin.SecureJoin(ident.DID.String(), repo.Name)
	if err != nil {
		fail(xrpcerr.GenericError(err))
		return
	}

	// TODO: fine-grained role based control
	isRepoOwner, err := x.Enforcer.IsRepoOwner(actorDid.String(), rbac.ThisServer, didSlashRepo)
	if err != nil || !isRepoOwner {
		fail(xrpcerr.AccessControlError(actorDid.String()))
		return
	}
	for _, engine := range x.Engines {
		l.Debug("destroying workflow", "wid", wid)
		err = engine.DestroyWorkflow(r.Context(), wid)
		if err != nil {
			fail(xrpcerr.GenericError(fmt.Errorf("failed to destroy workflow: %w", err)))
			return
		}
		err = x.Db.StatusCancelled(wid, "User canceled the workflow", -1, x.Notifier)
		if err != nil {
			fail(xrpcerr.GenericError(fmt.Errorf("failed to emit status failed: %w", err)))
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
