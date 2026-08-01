package api

import (
	"context"
	"encoding/json"
	"net/url"
)

// IssueRef — ссылка на задачу в теле POST /commands (SPEC §3.4, §3.6):
// ring-id (id) или idReadable. Ровно одно поле заполнено (§4.1).
type IssueRef struct {
	ID         string `json:"id,omitempty"`
	IDReadable string `json:"idReadable,omitempty"`
}

// commandRequest — тело POST /commands при yt issue close и yt command.
type commandRequest struct {
	Query   string     `json:"query"`
	Comment string     `json:"comment,omitempty"`
	Issues  []IssueRef `json:"issues"`
}

// ApplyCommand применяет командный язык YouTrack к списку задач (POST
// /commands, SPEC §3.4, §3.6). query — команда («state: Fixed»), comment —
// комментарий к применению (может быть пустым), issues — ссылки на задачи.
// fields — FieldsCommandIssues.
func (c *Client) ApplyCommand(ctx context.Context, query, comment string, issues []IssueRef, fields string) (*CommandList, error) {
	body, err := json.Marshal(commandRequest{
		Query:   query,
		Comment: comment,
		Issues:  issues,
	})
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if fields != "" {
		q.Set("fields", fields)
	}
	var out CommandList
	if err := c.Post(ctx, "/commands", q, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
