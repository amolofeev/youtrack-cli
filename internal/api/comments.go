package api

import (
	"context"
	"encoding/json"
	"net/url"
)

// IssueComments возвращает комментарии задачи
// (GET /issues/{id}/comments, SPEC §3.4). id — ring-id или idReadable;
// fields — FieldsIssueComments.
func (c *Client) IssueComments(ctx context.Context, id, fields string, top, skip int) ([]IssueComment, error) {
	q := listQuery(fields, top, skip)
	var out []IssueComment
	if err := c.Get(ctx, "/issues/"+EscapePath(id)+"/comments", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// commentCreateRequest — тело POST /issues/{id}/comments при yt issue comment
// create (SPEC §3.4).
type commentCreateRequest struct {
	Text string `json:"text"`
}

// CreateComment добавляет комментарий к задаче (POST /issues/{id}/comments,
// SPEC §3.4). id — ring-id или idReadable; text — текст комментария;
// fields — FieldsIssueCommentCreate.
func (c *Client) CreateComment(ctx context.Context, id, text, fields string) (*IssueComment, error) {
	body, err := json.Marshal(commentCreateRequest{Text: text})
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if fields != "" {
		q.Set("fields", fields)
	}
	var out IssueComment
	if err := c.Post(ctx, "/issues/"+EscapePath(id)+"/comments", q, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
