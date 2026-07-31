package api

import (
	"bytes"
	"encoding/json"
)

// Issue представляет задачу YouTrack (схема Issue, дискриминатор $type).
type Issue struct {
	Type          string             `json:"$type,omitempty"`
	ID            string             `json:"id,omitempty"`
	IDReadable    string             `json:"idReadable,omitempty"`
	Summary       string             `json:"summary,omitempty"`
	Description   string             `json:"description,omitempty"`
	Created       int64              `json:"created,omitempty"`
	Updated       int64              `json:"updated,omitempty"`
	Resolved      *int64             `json:"resolved,omitempty"`
	Project       *Project           `json:"project,omitempty"`
	Reporter      *User              `json:"reporter,omitempty"`
	Updater       *User              `json:"updater,omitempty"`
	CustomFields  []IssueCustomField `json:"customFields,omitempty"`
	Tags          []Tag              `json:"tags,omitempty"`
	CommentsCount *int64             `json:"commentsCount,omitempty"`
	Comments      []IssueComment     `json:"comments,omitempty"`
}

// IssueComment представляет комментарий к задаче (схема IssueComment).
type IssueComment struct {
	Type    string `json:"$type,omitempty"`
	ID      string `json:"id,omitempty"`
	Text    string `json:"text,omitempty"`
	Created int64  `json:"created,omitempty"`
	Author  *User  `json:"author,omitempty"`
}

// Project представляет проект (схема Project).
type Project struct {
	Type      string `json:"$type,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	ShortName string `json:"shortName,omitempty"`
	Archived  *bool  `json:"archived,omitempty"`
	Leader    *User  `json:"leader,omitempty"`
}

// Tag представляет тег (схема Tag).
type Tag struct {
	Type           string `json:"$type,omitempty"`
	ID             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	UntagOnResolve *bool  `json:"untagOnResolve,omitempty"`
}

// User представляет пользователя (схемы User/Me).
type User struct {
	Type      string `json:"$type,omitempty"`
	ID        string `json:"id,omitempty"`
	Login     string `json:"login,omitempty"`
	FullName  string `json:"fullName,omitempty"`
	Email     string `json:"email,omitempty"`
	Guest     *bool  `json:"guest,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

// IssueCustomField представляет кастомное поле задачи (схема IssueCustomField
// и наследники). Value хранится как json.RawMessage: для мульти-значений сервер
// возвращает массив, и строгое декодирование в структуру сломало бы весь ответ.
type IssueCustomField struct {
	Type  string          `json:"$type,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// FieldValue — типизированное представление одиночного значения кастомного поля
// (union полей, запрашиваемых через value($type,...) в issue view).
type FieldValue struct {
	Type          string `json:"$type,omitempty"`
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	Login         string `json:"login,omitempty"`
	FullName      string `json:"fullName,omitempty"`
	Minutes       int64  `json:"minutes,omitempty"`
	MinutesPerDay int64  `json:"minutesPerDay,omitempty"`
	Presentation  string `json:"presentation,omitempty"`
}

// ValueObject декодирует value кастомного поля в FieldValue. ok=false, если
// value отсутствует, равно null или не является объектом (например, массив).
func (f *IssueCustomField) ValueObject() (FieldValue, bool) {
	var v FieldValue
	if len(f.Value) == 0 || bytes.Equal(bytes.TrimSpace(f.Value), []byte("null")) {
		return v, false
	}
	if err := json.Unmarshal(f.Value, &v); err != nil {
		return v, false
	}
	return v, true
}

// Suggestion — подсказка автодополнения (схема Suggestion).
type Suggestion struct {
	Type        string `json:"$type,omitempty"`
	ID          string `json:"id,omitempty"`
	Option      string `json:"option,omitempty"`
	Description string `json:"description,omitempty"`
	Prefix      string `json:"prefix,omitempty"`
	Suffix      string `json:"suffix,omitempty"`
	Group       string `json:"group,omitempty"`
}

// SearchSuggestions — ответ /search/assist (схема SearchSuggestions).
type SearchSuggestions struct {
	Type        string       `json:"$type,omitempty"`
	ID          string       `json:"id,omitempty"`
	Query       string       `json:"query,omitempty"`
	Caret       int          `json:"caret,omitempty"`
	Suggestions []Suggestion `json:"suggestions,omitempty"`
}

// CommandList — ответ /commands и /commands/assist (схема CommandList).
type CommandList struct {
	Type        string       `json:"$type,omitempty"`
	ID          string       `json:"id,omitempty"`
	Query       string       `json:"query,omitempty"`
	Comment     string       `json:"comment,omitempty"`
	RunAs       string       `json:"runAs,omitempty"`
	Issues      []Issue      `json:"issues,omitempty"`
	Suggestions []Suggestion `json:"suggestions,omitempty"`
}
