package api

import (
	"context"
)

// Search возвращает задачи по произвольному поисковому запросу
// (GET /issues?query=..., SPEC §3.5). Тот же эндпоинт, что и ListIssues,
// но без «умных» флагов issue list: аргумент — сырой запрос YouTrack.
func (c *Client) Search(ctx context.Context, query, fields string, top, skip int) ([]Issue, error) {
	return c.ListIssues(ctx, query, fields, top, skip)
}
