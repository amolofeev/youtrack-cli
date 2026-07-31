package api

import (
	"context"
)

// ListTags возвращает список тегов (GET /tags, SPEC §3.9). query — фильтр по
// имени тега, fields — FieldsTagList.
func (c *Client) ListTags(ctx context.Context, query, fields string, top, skip int) ([]Tag, error) {
	q := listQuery(fields, top, skip)
	if query != "" {
		q.Set("query", query)
	}
	var out []Tag
	if err := c.Get(ctx, "/tags", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}
