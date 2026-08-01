package api

import (
	"context"
	"encoding/json"
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

// issueCreateRequest — тело POST /issues при yt issue create (SPEC §3.4).
// project передаётся как ring-id; description опускается при пустой строке.
type issueCreateRequest struct {
	Project     Project `json:"project"`
	Summary     string  `json:"summary"`
	Description string  `json:"description,omitempty"`
}

// CreateIssue создаёт задачу (POST /issues, SPEC §3.4). projectID — ring-id
// уже резолвинутого проекта (yt issue create), summary/description — текст
// задачи. fields — FieldsIssueCreate.
func (c *Client) CreateIssue(ctx context.Context, projectID, summary, description, fields string) (*Issue, error) {
	body, err := json.Marshal(issueCreateRequest{
		Project:     Project{ID: projectID},
		Summary:     summary,
		Description: description,
	})
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if fields != "" {
		q.Set("fields", fields)
	}
	var out Issue
	if err := c.Post(ctx, "/issues", q, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// issueUpdateRequest — тело POST /issues/{id} при yt issue edit (SPEC §3.4).
// Частичное обновление: nil означает «поле не изменять», пустая строка
// передаётся серверу как есть.
type issueUpdateRequest struct {
	Summary     *string `json:"summary,omitempty"`
	Description *string `json:"description,omitempty"`
}

// UpdateIssue обновляет задачу (POST /issues/{id}, SPEC §3.4). id — ring-id
// или idReadable, передаётся без преобразований с URL-кодированием (§4.1).
// summary/description — частичное обновление: nil — поле не изменяется.
// fields — FieldsIssueEdit.
func (c *Client) UpdateIssue(ctx context.Context, id, fields string, summary, description *string) (*Issue, error) {
	body, err := json.Marshal(issueUpdateRequest{
		Summary:     summary,
		Description: description,
	})
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if fields != "" {
		q.Set("fields", fields)
	}
	var out Issue
	if err := c.Post(ctx, "/issues/"+EscapePath(id), q, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteIssue удаляет задачу (DELETE /issues/{id}, SPEC §3.4). id — ring-id
// или idReadable, передаётся без преобразований с URL-кодированием (§4.1).
// Ответ сервера — 200 без тела.
func (c *Client) DeleteIssue(ctx context.Context, id string) error {
	return c.Delete(ctx, "/issues/"+EscapePath(id))
}
