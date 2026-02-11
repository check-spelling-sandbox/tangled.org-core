package repo

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"tangled.org/core/appview/db"
	"tangled.org/core/appview/models"
	"tangled.org/core/appview/pages"
)

// Webhooks displays the webhooks settings page
func (rp *Repo) Webhooks(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "Webhooks")

	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	user := rp.oauth.GetMultiAccountUser(r)

	webhooks, err := db.GetWebhooksForRepo(rp.db, f.RepoAt())
	if err != nil {
		l.Error("failed to get webhooks", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to load webhooks")
		return
	}

	rp.pages.RepoWebhooksSettings(w, pages.RepoWebhooksSettingsParams{
		LoggedInUser: user,
		RepoInfo:     rp.repoResolver.GetRepoInfo(r, user),
		Webhooks:     webhooks,
	})
}

// AddWebhook creates a new webhook
func (rp *Repo) AddWebhook(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "AddWebhook")

	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	url := strings.TrimSpace(r.FormValue("url"))
	if url == "" {
		rp.pages.Notice(w, "webhooks-error", "Webhook URL is required")
		return
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		rp.pages.Notice(w, "webhooks-error", "Webhook URL must start with http:// or https://")
		return
	}

	secret := strings.TrimSpace(r.FormValue("secret"))
	// if secret is empty, we don't sign

	active := r.FormValue("active") == "on"

	events := []string{}
	if r.FormValue("event_push") == "on" {
		events = append(events, string(models.WebhookEventPush))
	}

	if len(events) == 0 {
		rp.pages.Notice(w, "webhooks-error", "Push events must be enabled")
		return
	}

	webhook := &models.Webhook{
		RepoAt: f.RepoAt(),
		Url:    url,
		Secret: secret,
		Active: active,
		Events: events,
	}

	tx, err := rp.db.Begin()
	if err != nil {
		l.Error("failed to start transaction", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to create webhook")
		return
	}
	defer tx.Rollback()

	if err := db.AddWebhook(tx, webhook); err != nil {
		l.Error("failed to add webhook", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to create webhook")
		return
	}

	if err := tx.Commit(); err != nil {
		l.Error("failed to commit transaction", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to create webhook")
		return
	}

	rp.pages.HxRefresh(w)
}

// UpdateWebhook updates an existing webhook
func (rp *Repo) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "UpdateWebhook")

	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		l.Error("invalid webhook id", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	webhook, err := db.GetWebhook(rp.db, id)
	if err != nil {
		l.Error("failed to get webhook", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Webhook not found")
		return
	}

	// Verify webhook belongs to this repo
	if webhook.RepoAt != f.RepoAt() {
		l.Error("webhook does not belong to repo", "webhook_repo", webhook.RepoAt, "current_repo", f.RepoAt())
		w.WriteHeader(http.StatusForbidden)
		return
	}

	url := strings.TrimSpace(r.FormValue("url"))
	if url != "" {
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			rp.pages.Notice(w, "webhooks-error", "Webhook URL must start with http:// or https://")
			return
		}
		webhook.Url = url
	}

	secret := strings.TrimSpace(r.FormValue("secret"))
	if secret != "" {
		webhook.Secret = secret
	}

	webhook.Active = r.FormValue("active") == "on"

	// Parse events - only push events are supported for now
	events := []string{}
	if r.FormValue("event_push") == "on" {
		events = append(events, string(models.WebhookEventPush))
	}

	if len(events) > 0 {
		webhook.Events = events
	}

	tx, err := rp.db.Begin()
	if err != nil {
		l.Error("failed to start transaction", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to update webhook")
		return
	}
	defer tx.Rollback()

	if err := db.UpdateWebhook(tx, webhook); err != nil {
		l.Error("failed to update webhook", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to update webhook")
		return
	}

	if err := tx.Commit(); err != nil {
		l.Error("failed to commit transaction", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to update webhook")
		return
	}

	rp.pages.HxRefresh(w)
}

// DeleteWebhook deletes a webhook
func (rp *Repo) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "DeleteWebhook")

	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		l.Error("invalid webhook id", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	webhook, err := db.GetWebhook(rp.db, id)
	if err != nil {
		l.Error("failed to get webhook", "err", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Verify webhook belongs to this repo
	if webhook.RepoAt != f.RepoAt() {
		l.Error("webhook does not belong to repo", "webhook_repo", webhook.RepoAt, "current_repo", f.RepoAt())
		w.WriteHeader(http.StatusForbidden)
		return
	}

	tx, err := rp.db.Begin()
	if err != nil {
		l.Error("failed to start transaction", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to delete webhook")
		return
	}
	defer tx.Rollback()

	if err := db.DeleteWebhook(tx, id); err != nil {
		l.Error("failed to delete webhook", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to delete webhook")
		return
	}

	if err := tx.Commit(); err != nil {
		l.Error("failed to commit transaction", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to delete webhook")
		return
	}

	rp.pages.HxRefresh(w)
}

// ToggleWebhook toggles the active state of a webhook
func (rp *Repo) ToggleWebhook(w http.ResponseWriter, r *http.Request) {
	l := rp.logger.With("handler", "ToggleWebhook")

	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		l.Error("failed to get repo and knot", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		l.Error("invalid webhook id", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	webhook, err := db.GetWebhook(rp.db, id)
	if err != nil {
		l.Error("failed to get webhook", "err", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Verify webhook belongs to this repo
	if webhook.RepoAt != f.RepoAt() {
		l.Error("webhook does not belong to repo", "webhook_repo", webhook.RepoAt, "current_repo", f.RepoAt())
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Toggle the active state
	webhook.Active = !webhook.Active

	tx, err := rp.db.Begin()
	if err != nil {
		l.Error("failed to start transaction", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to toggle webhook")
		return
	}
	defer tx.Rollback()

	if err := db.UpdateWebhook(tx, webhook); err != nil {
		l.Error("failed to update webhook", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to toggle webhook")
		return
	}

	if err := tx.Commit(); err != nil {
		l.Error("failed to commit transaction", "err", err)
		rp.pages.Notice(w, "webhooks-error", "Failed to toggle webhook")
		return
	}

	rp.pages.HxRefresh(w)
}
