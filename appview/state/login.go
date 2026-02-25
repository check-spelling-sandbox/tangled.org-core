package state

import (
	"fmt"
	"net/http"
	"strings"

	"tangled.org/core/appview/oauth"
	"tangled.org/core/appview/pages"
)

func (s *State) Login(w http.ResponseWriter, r *http.Request) {
	l := s.logger.With("handler", "Login")

	switch r.Method {
	case http.MethodGet:
		returnURL := r.URL.Query().Get("return_url")
		errorCode := r.URL.Query().Get("error")
		addAccount := r.URL.Query().Get("mode") == "add_account"

		user := s.oauth.GetMultiAccountUser(r)
		if user == nil {
			registry := s.oauth.GetAccounts(r)
			if len(registry.Accounts) > 0 {
				user = &oauth.MultiAccountUser{
					Active:   nil,
					Accounts: registry.Accounts,
				}
			}
		}
		s.pages.Login(w, pages.LoginParams{
			ReturnUrl:    returnURL,
			ErrorCode:    errorCode,
			AddAccount:   addAccount,
			LoggedInUser: user,
		})
	case http.MethodPost:
		handle := r.FormValue("handle")
		returnURL := r.FormValue("return_url")
		addAccount := r.FormValue("add_account") == "true"

		// remove spaces around the handle, handles can't have spaces around them
		handle = strings.TrimSpace(handle)

		// when users copy their handle from bsky.app, it tends to have these characters around it:
		//
		// @nelind.dk:
		//   \u202a ensures that the handle is always rendered left to right and
		//   \u202c reverts that so the rest of the page renders however it should
		handle = strings.TrimPrefix(handle, "\u202a")
		handle = strings.TrimSuffix(handle, "\u202c")

		// `@` is harmless
		handle = strings.TrimPrefix(handle, "@")

		// basic handle validation
		if !strings.Contains(handle, ".") {
			l.Error("invalid handle format", "raw", handle)
			s.pages.Notice(
				w,
				"login-msg",
				fmt.Sprintf("\"%s\" is an invalid handle. Did you mean %s.bsky.social or %s.tngl.sh?", handle, handle, handle),
			)
			return
		}

		if err := s.oauth.SetAuthReturn(w, r, returnURL, addAccount); err != nil {
			l.Error("failed to set auth return", "err", err)
		}

		redirectURL, err := s.oauth.ClientApp.StartAuthFlow(r.Context(), handle)
		if err != nil {
			l.Error("failed to start auth", "err", err)
			s.pages.Notice(
				w,
				"login-msg",
				fmt.Sprintf("Failed to start auth flow: %v", err),
			)
			return
		}

		s.pages.HxRedirect(w, redirectURL)
	}
}

func (s *State) Logout(w http.ResponseWriter, r *http.Request) {
	l := s.logger.With("handler", "Logout")

	currentUser := s.oauth.GetMultiAccountUser(r)
	if currentUser == nil || currentUser.Active == nil {
		s.pages.HxRedirect(w, "/login")
		return
	}

	currentDid := currentUser.Active.Did

	var remainingAccounts []string
	for _, acc := range currentUser.Accounts {
		if acc.Did != currentDid {
			remainingAccounts = append(remainingAccounts, acc.Did)
		}
	}

	if err := s.oauth.RemoveAccount(w, r, currentDid); err != nil {
		l.Error("failed to remove account from registry", "err", err)
	}

	if err := s.oauth.DeleteSession(w, r); err != nil {
		l.Error("failed to delete session", "err", err)
	}

	if len(remainingAccounts) > 0 {
		nextDid := remainingAccounts[0]
		if err := s.oauth.SwitchAccount(w, r, nextDid); err != nil {
			l.Error("failed to switch to next account", "err", err)
			s.pages.HxRedirect(w, "/login")
			return
		}
		l.Info("switched to next account after logout", "did", nextDid)
		s.pages.HxRefresh(w)
		return
	}

	l.Info("logged out last account")
	s.pages.HxRedirect(w, "/login")
}
