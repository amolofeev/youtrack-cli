package api

import (
	"context"
)

// ListProjects возвращает список проектов (GET /admin/projects, SPEC §3.7).
// fields — FieldsProjectList (или FieldsProjectResolve для резолвинга проекта).
func (c *Client) ListProjects(ctx context.Context, fields string, top, skip int) ([]Project, error) {
	q := listQuery(fields, top, skip)
	var out []Project
	if err := c.Get(ctx, "/admin/projects", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}
