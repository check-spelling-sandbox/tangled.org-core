package repo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"tangled.org/core/api/tangled"
	"tangled.org/core/appview/db"
	"tangled.org/core/appview/models"
	"tangled.org/core/appview/pages"
	"tangled.org/core/appview/reporesolver"
	xrpcclient "tangled.org/core/appview/xrpcclient"
	"tangled.org/core/orm"
	"tangled.org/core/types"

	indigoxrpc "github.com/bluesky-social/indigo/xrpc"
	"github.com/go-chi/chi/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func (rp *Repo) Tags(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "RepoTags")
	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		return
	}
	scheme := "http"
	if !rp.config.Core.Dev {
		scheme = "https"
	}
	host := fmt.Sprintf("%s://%s", scheme, f.Knot)
	xrpcc := &indigoxrpc.Client{
		Host: host,
	}
	repo := fmt.Sprintf("%s/%s", f.Did, f.Name)
	xrpcBytes, err := tangled.RepoTags(r.Context(), xrpcc, "", 0, repo)
	if xrpcerr := xrpcclient.HandleXrpcErr(err); xrpcerr != nil {
		l.Error("failed to call XRPC repo.tags", "err", xrpcerr)
		rp.pages.Error503(w)
		return
	}
	var result types.RepoTagsResponse
	if err := json.Unmarshal(xrpcBytes, &result); err != nil {
		l.Error("failed to decode XRPC response", "err", err)
		rp.pages.Error503(w)
		return
	}
	artifacts, err := db.GetArtifact(rp.db, orm.FilterEq("repo_at", f.RepoAt()))
	if err != nil {
		l.Error("failed grab artifacts", "err", err)
		return
	}
	// convert artifacts to map for easy UI building
	artifactMap := make(map[plumbing.Hash][]models.Artifact)
	for _, a := range artifacts {
		artifactMap[a.Tag] = append(artifactMap[a.Tag], a)
	}
	var danglingArtifacts []models.Artifact
	for _, a := range artifacts {
		found := false
		for _, t := range result.Tags {
			if t.Tag != nil {
				if t.Tag.Hash == a.Tag {
					found = true
				}
			}
		}
		if !found {
			danglingArtifacts = append(danglingArtifacts, a)
		}
	}
	user := rp.oauth.GetMultiAccountUser(r)

	rp.pages.RepoTags(w, pages.RepoTagsParams{
		LoggedInUser:      user,
		RepoInfo:          rp.repoResolver.GetRepoInfo(r, user),
		RepoTagsResponse:  result,
		ArtifactMap:       artifactMap,
		DanglingArtifacts: danglingArtifacts,
	})
}

func (rp *Repo) Tag(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "RepoTag")
	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		return
	}
	scheme := "http"
	if !rp.config.Core.Dev {
		scheme = "https"
	}
	host := fmt.Sprintf("%s://%s", scheme, f.Knot)
	xrpcc := &indigoxrpc.Client{
		Host: host,
	}
	repo := fmt.Sprintf("%s/%s", f.Did, f.Name)
	tag := chi.URLParam(r, "tag")

	xrpcBytes, err := tangled.RepoTag(r.Context(), xrpcc, repo, tag)
	if xrpcerr := xrpcclient.HandleXrpcErr(err); xrpcerr != nil {
		// if we don't match an existing tag, and the tag we're trying
		// to match is "latest", resolve to the most recent tag
		if tag == "latest" {
			tagsBytes, err := tangled.RepoTags(r.Context(), xrpcc, "", 1, repo)
			if xrpcerr := xrpcclient.HandleXrpcErr(err); xrpcerr != nil {
				l.Error("failed to call XRPC repo.tags for latest", "err", xrpcerr)
				rp.pages.Error503(w)
				return
			}
			var tagsResult types.RepoTagsResponse
			if err := json.Unmarshal(tagsBytes, &tagsResult); err != nil {
				l.Error("failed to decode XRPC response", "err", err)
				rp.pages.Error503(w)
				return
			}
			if len(tagsResult.Tags) == 0 {
				rp.pages.Error503(w)
				return
			}
			latestTag := tagsResult.Tags[0].Name
			ownerSlashRepo := reporesolver.GetBaseRepoPath(r, f)
			http.Redirect(w, r, fmt.Sprintf("/%s/tags/%s", ownerSlashRepo, url.PathEscape(latestTag)), http.StatusTemporaryRedirect)
			return
		}
		l.Error("failed to call XRPC repo.tag", "err", xrpcerr)
		rp.pages.Error503(w)
		return
	}
	var result types.RepoTagResponse
	if err := json.Unmarshal(xrpcBytes, &result); err != nil {
		l.Error("failed to decode XRPC response", "err", err)
		rp.pages.Error503(w)
		return
	}

	filters := []orm.Filter{orm.FilterEq("repo_at", f.RepoAt())}
	if result.Tag.Tag != nil {
		filters = append(filters, orm.FilterEq("tag", result.Tag.Tag.Hash[:]))
	}

	artifacts, err := db.GetArtifact(rp.db, filters...)
	if err != nil {
		l.Error("failed grab artifacts", "err", err)
		return
	}
	// convert artifacts to map for easy UI building
	artifactMap := make(map[plumbing.Hash][]models.Artifact)
	for _, a := range artifacts {
		artifactMap[a.Tag] = append(artifactMap[a.Tag], a)
	}

	user := rp.oauth.GetMultiAccountUser(r)
	rp.pages.RepoTag(w, pages.RepoTagParams{
		LoggedInUser:    user,
		RepoInfo:        rp.repoResolver.GetRepoInfo(r, user),
		RepoTagResponse: result,
		ArtifactMap:     artifactMap,
	})
}
