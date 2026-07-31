package api

import (
	"context"
	"net/url"
)

// Me возвращает профиль текущего пользователя по токену
// (GET /users/me, SPEC §3.3, §3.8). fields — FieldsAuthLogin /
// FieldsAuthStatus / FieldsUserWhoami.
func (c *Client) Me(ctx context.Context, fields string) (*User, error) {
	q := url.Values{}
	if fields != "" {
		q.Set("fields", fields)
	}
	var out User
	if err := c.Get(ctx, "/users/me", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
