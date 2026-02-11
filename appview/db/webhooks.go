package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"tangled.org/core/appview/models"
	"tangled.org/core/orm"
)

// GetWebhooks returns all webhooks for a repository
func GetWebhooks(e Execer, filters ...orm.Filter) ([]models.Webhook, error) {
	var conditions []string
	var args []any
	for _, filter := range filters {
		conditions = append(conditions, filter.Condition())
		args = append(args, filter.Arg()...)
	}

	whereClause := ""
	if conditions != nil {
		whereClause = " where " + strings.Join(conditions, " and ")
	}

	query := fmt.Sprintf(`
		select
			id,
			repo_at,
			url,
			secret,
			active,
			events,
			created_at,
			updated_at
		from webhooks
		%s
		order by created_at desc
	`, whereClause)

	rows, err := e.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []models.Webhook
	for rows.Next() {
		var wh models.Webhook
		var createdAt, updatedAt, eventsStr string
		var secret sql.NullString
		var active int

		err := rows.Scan(
			&wh.Id,
			&wh.RepoAt,
			&wh.Url,
			&secret,
			&active,
			&eventsStr,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook: %w", err)
		}

		if secret.Valid {
			wh.Secret = secret.String
		}
		wh.Active = active == 1
		if eventsStr != "" {
			wh.Events = strings.Split(eventsStr, ",")
		}

		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			wh.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			wh.UpdatedAt = t
		}

		webhooks = append(webhooks, wh)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate webhooks: %w", err)
	}

	return webhooks, nil
}

// GetWebhook returns a single webhook by ID
func GetWebhook(e Execer, id int64) (*models.Webhook, error) {
	webhooks, err := GetWebhooks(e, orm.FilterEq("id", id))
	if err != nil {
		return nil, err
	}

	if len(webhooks) == 0 {
		return nil, sql.ErrNoRows
	}

	if len(webhooks) != 1 {
		return nil, fmt.Errorf("expected 1 webhook, got %d", len(webhooks))
	}

	return &webhooks[0], nil
}

// AddWebhook creates a new webhook
func AddWebhook(e Execer, webhook *models.Webhook) error {
	eventsStr := strings.Join(webhook.Events, ",")
	active := 0
	if webhook.Active {
		active = 1
	}

	result, err := e.Exec(`
		insert into webhooks (repo_at, url, secret, active, events)
		values (?, ?, ?, ?, ?)
	`, webhook.RepoAt.String(), webhook.Url, webhook.Secret, active, eventsStr)

	if err != nil {
		return fmt.Errorf("failed to insert webhook: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get webhook id: %w", err)
	}

	webhook.Id = id
	return nil
}

// UpdateWebhook updates an existing webhook
func UpdateWebhook(e Execer, webhook *models.Webhook) error {
	eventsStr := strings.Join(webhook.Events, ",")
	active := 0
	if webhook.Active {
		active = 1
	}

	_, err := e.Exec(`
		update webhooks
		set url = ?, secret = ?, active = ?, events = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		where id = ?
	`, webhook.Url, webhook.Secret, active, eventsStr, webhook.Id)

	if err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

// DeleteWebhook deletes a webhook
func DeleteWebhook(e Execer, id int64) error {
	_, err := e.Exec(`delete from webhooks where id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	return nil
}

// AddWebhookDelivery records a webhook delivery attempt
func AddWebhookDelivery(e Execer, delivery *models.WebhookDelivery) error {
	success := 0
	if delivery.Success {
		success = 1
	}

	result, err := e.Exec(`
		insert into webhook_deliveries (
			webhook_id,
			event,
			delivery_id,
			url,
			request_body,
			response_code,
			response_body,
			success
		) values (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		delivery.WebhookId,
		delivery.Event,
		delivery.DeliveryId,
		delivery.Url,
		delivery.RequestBody,
		delivery.ResponseCode,
		delivery.ResponseBody,
		success,
	)

	if err != nil {
		return fmt.Errorf("failed to insert webhook delivery: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get delivery id: %w", err)
	}

	delivery.Id = id
	return nil
}

// GetWebhookDeliveries returns recent deliveries for a webhook
func GetWebhookDeliveries(e Execer, webhookId int64, limit int) ([]models.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		select
			id,
			webhook_id,
			event,
			delivery_id,
			url,
			request_body,
			response_code,
			response_body,
			success,
			created_at
		from webhook_deliveries
		where webhook_id = ?
		order by created_at desc
		limit ?
	`

	rows, err := e.Query(query, webhookId, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query webhook deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []models.WebhookDelivery
	for rows.Next() {
		var d models.WebhookDelivery
		var createdAt string
		var success int
		var responseCode sql.NullInt64
		var responseBody sql.NullString

		err := rows.Scan(
			&d.Id,
			&d.WebhookId,
			&d.Event,
			&d.DeliveryId,
			&d.Url,
			&d.RequestBody,
			&responseCode,
			&responseBody,
			&success,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan delivery: %w", err)
		}

		d.Success = success == 1
		if responseCode.Valid {
			d.ResponseCode = int(responseCode.Int64)
		}
		if responseBody.Valid {
			d.ResponseBody = responseBody.String
		}

		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			d.CreatedAt = t
		}

		deliveries = append(deliveries, d)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate deliveries: %w", err)
	}

	return deliveries, nil
}

// GetWebhooksForRepo is a convenience function to get all webhooks for a repository
func GetWebhooksForRepo(e Execer, repoAt syntax.ATURI) ([]models.Webhook, error) {
	return GetWebhooks(e, orm.FilterEq("repo_at", repoAt.String()))
}

// GetActiveWebhooksForRepo returns only active webhooks for a repository
func GetActiveWebhooksForRepo(e Execer, repoAt syntax.ATURI) ([]models.Webhook, error) {
	return GetWebhooks(e,
		orm.FilterEq("repo_at", repoAt.String()),
		orm.FilterEq("active", 1),
	)
}
