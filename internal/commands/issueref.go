package commands

import (
	"fmt"
	"regexp"

	"github.com/amolofeev/yt/internal/api"
)

// Паттерны идентификаторов задач (§4.1): ring-id и idReadable. Форматы не
// пересекаются, поэтому выбор поля детерминирован.
var (
	issueRingIDRe   = regexp.MustCompile(`^[0-9]+-[0-9]+$`)
	issueReadableRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*-[0-9]+$`)
)

// parseIssueRef разбирает идентификатор задачи по эвристике §4.1 для команд,
// передающих id в теле запроса (issue close, command): «2-1» → ring-id;
// «PRJ-42»/«B2B-3» (без учёта регистра) → idReadable; прочее — ошибка
// использования (exit 2). Общий хелпер, переиспользуется Атомом 5.4.
func parseIssueRef(value string) (api.IssueRef, error) {
	if issueRingIDRe.MatchString(value) {
		return api.IssueRef{ID: value}, nil
	}
	if issueReadableRe.MatchString(value) {
		return api.IssueRef{IDReadable: value}, nil
	}
	return api.IssueRef{}, usageError(fmt.Errorf("cannot parse issue id: %s", value))
}
