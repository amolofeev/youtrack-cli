package api

import (
	"context"
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
