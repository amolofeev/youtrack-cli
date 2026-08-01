package api

import (
	"context"
	"encoding/json"
	"net/url"
	"unicode/utf8"
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
	RunAs   string     `json:"runAs,omitempty"`
	Issues  []IssueRef `json:"issues"`
}

// ApplyCommand применяет командный язык YouTrack к списку задач (POST
// /commands, SPEC §3.4, §3.6). query — команда («state: Fixed»), comment —
// комментарий к применению (может быть пустым), runAs — пользователь, от
// имени которого исполняется команда (пусто — текущий пользователь),
// issues — ссылки на задачи. fields — FieldsCommandIssues.
func (c *Client) ApplyCommand(ctx context.Context, query, comment, runAs string, issues []IssueRef, fields string) (*CommandList, error) {
	body, err := json.Marshal(commandRequest{
		Query:   query,
		Comment: comment,
		RunAs:   runAs,
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

// commandAssistRequest — тело POST /commands/assist при yt command assist (SPEC §3.6).
type commandAssistRequest struct {
	Query string `json:"query"`
	Caret int    `json:"caret"`
}

// CommandAssist возвращает подсказки автодополнения для предварительного разбора
// команды без применения (POST /commands/assist, SPEC §3.6). query — команда
// или её часть; caret — позиция каретки в конце query (число символов, а не
// байт — команды могут содержать не-ASCII). fields — FieldsCommandAssist.
func (c *Client) CommandAssist(ctx context.Context, query string, fields string) (*CommandList, error) {
	body, err := json.Marshal(commandAssistRequest{Query: query, Caret: utf8.RuneCountInString(query)})
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if fields != "" {
		q.Set("fields", fields)
	}
	var out CommandList
	if err := c.Post(ctx, "/commands/assist", q, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
