package api

import (
	"context"
	"net/url"
	"strconv"
)

// listQuery собирает общие query-параметры списковых эндпоинтов: fields,
// $top, $skip. Параметры с нулевыми значениями не передаются.
func listQuery(fields string, top, skip int) url.Values {
	q := url.Values{}
	if fields != "" {
		q.Set("fields", fields)
	}
	if top > 0 {
		q.Set("$top", strconv.Itoa(top))
	}
	if skip > 0 {
		q.Set("$skip", strconv.Itoa(skip))
	}
	return q
}

// ListIssues возвращает список задач (GET /issues, SPEC §3.4). query — полный
// поисковый запрос YouTrack (может быть пустым), fields — FieldsIssueList.
func (c *Client) ListIssues(ctx context.Context, query, fields string, top, skip int) ([]Issue, error) {
	q := listQuery(fields, top, skip)
	if query != "" {
		q.Set("query", query)
	}
	var out []Issue
	if err := c.Get(ctx, "/issues", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Issue возвращает задачу по идентификатору (GET /issues/{id}, SPEC §3.4).
// id — ring-id или idReadable, передаётся без преобразований с
// URL-кодированием (§4.1). fields — FieldsIssueView.
func (c *Client) Issue(ctx context.Context, id, fields string) (*Issue, error) {
	q := url.Values{}
	if fields != "" {
		q.Set("fields", fields)
	}
	var out Issue
	if err := c.Get(ctx, "/issues/"+EscapePath(id), q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
