package state

import (
	"net/http"

	"tangled.org/core/appview/db"
	"tangled.org/core/appview/pages"
	"tangled.org/core/orm"
)

func (s *State) Home(w http.ResponseWriter, r *http.Request) {
	// TODO: set this flag based on the UI
	filtered := false

	user := s.oauth.GetMultiAccountUser(r)

	timeline, err := db.MakeTimeline(s.db, 50, "", filtered)
	if err != nil {
		s.logger.Error("failed to make timeline", "err", err)
		s.pages.Notice(w, "timeline", "Uh oh! Failed to load timeline.")
		return
	}

	blueskyPosts, err := db.GetBlueskyPosts(s.db, 8)
	if err != nil {
		s.logger.Error("failed to get bluesky posts", "err", err)
	}

	s.pages.Home(w, pages.TimelineParams{
		LoggedInUser: user,
		Timeline:     timeline,
		BlueskyPosts: blueskyPosts,
	})
}
func (s *State) HomeOrTimeline(w http.ResponseWriter, r *http.Request) {
	if s.oauth.GetMultiAccountUser(r) != nil {
		s.Timeline(w, r)
		return
	}
	s.Home(w, r)
}

func (s *State) Timeline(w http.ResponseWriter, r *http.Request) {
	user := s.oauth.GetMultiAccountUser(r)

	// TODO: set this flag based on the UI
	filtered := false

	var userDid string
	if user != nil && user.Active != nil {
		userDid = user.Active.Did
	}
	timeline, err := db.MakeTimeline(s.db, 50, userDid, filtered)
	if err != nil {
		s.logger.Error("failed to make timeline", "err", err)
		s.pages.Notice(w, "timeline", "Uh oh! Failed to load timeline.")
	}

	repos, err := db.GetTopStarredReposLastWeek(s.db)
	if err != nil {
		s.logger.Error("failed to get top starred repos", "err", err)
		s.pages.Notice(w, "topstarredrepos", "Unable to load.")
		return
	}

	gfiLabel, err := db.GetLabelDefinition(s.db, orm.FilterEq("at_uri", s.config.Label.GoodFirstIssue))
	if err != nil {
		// non-fatal
	}

	s.pages.Timeline(w, pages.TimelineParams{
		LoggedInUser: user,
		Timeline:     timeline,
		Repos:        repos,
		GfiLabel:     gfiLabel,
	})
}
