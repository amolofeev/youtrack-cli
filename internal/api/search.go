package api

import (
	"context"
	"encoding/json"
	"net/url"
)

// Search возвращает задачи по произвольному поисковому запросу
// (GET /issues?query=..., SPEC §3.5). Тот же эндпоинт, что и ListIssues,
// но без «умных» флагов issue list: аргумент — сырой запрос YouTrack.
func (c *Client) Search(ctx context.Context, query, fields string, top, skip int) ([]Issue, error) {
	return c.ListIssues(ctx, query, fields, top, skip)
}

// searchSuggestRequest — тело POST /search/assist при yt search suggest (SPEC §3.5).
type searchSuggestRequest struct {
	Query string `json:"query"`
}

// SearchSuggest возвращает подсказки автодополнения для частичного поискового
// запроса (POST /search/assist, SPEC §3.5). query — частичный запрос YouTrack;
// fields — FieldsSearchSuggest.
func (c *Client) SearchSuggest(ctx context.Context, query, fields string) (*SearchSuggestions, error) {
	body, err := json.Marshal(searchSuggestRequest{Query: query})
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if fields != "" {
		q.Set("fields", fields)
	}
	var out SearchSuggestions
	if err := c.Post(ctx, "/search/assist", q, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
