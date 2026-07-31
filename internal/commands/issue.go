package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/amolofeev/prompt-and-pray/internal/api"
	"github.com/amolofeev/prompt-and-pray/internal/output"
	"github.com/spf13/cobra"
)

const issueListDefaultLimit = 30
const issueListMaxLimit = 100

// buildIssueQuery собирает поисковый запрос из позиционного аргумента и флагов
// (SPEC §3.4): -q заменяет позиционный аргумент, остальные флаги добавляют
// префиксы state:/project:/assignee:/tag: через AND (конкатенация пробелом).
func buildIssueQuery(positional, query, state, project, assignee string, tags []string) string {
	var parts []string
	base := query
	if base == "" {
		base = positional
	}
	if base != "" {
		parts = append(parts, base)
	}
	if v := translateState(state); v != "" {
		parts = append(parts, "state: "+v)
	}
	if project != "" {
		parts = append(parts, "project: "+project)
	}
	if assignee != "" {
		parts = append(parts, "assignee: "+assignee)
	}
	for _, tag := range tags {
		if tag != "" {
			parts = append(parts, "tag: "+tag)
		}
	}
	return strings.Join(parts, " ")
}

// translateState транслирует -s в значение префикса state: (SPEC §3.4):
// open → #Unresolved, resolved → #Resolved, all → пусто (флаг не участвует),
// прочее — как есть.
func translateState(s string) string {
	switch s {
	case "open":
		return "#Unresolved"
	case "resolved":
		return "#Resolved"
	case "all":
		return ""
	default:
		return s
	}
}

// formatDate преобразует unix timestamp в миллисекундах в YYYY-MM-DD.
func formatDate(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format("2006-01-02")
}

// issueRow собирает строку таблицы issue list (SPEC §3.4): ID — idReadable,
// STATE — кастомное поле State, REPORTER — login, даты — YYYY-MM-DD.
func issueRow(i api.Issue) []string {
	return []string{
		issueID(i),
		issueState(i),
		formatDate(i.Created),
		formatDate(i.Updated),
		issueReporter(i),
		i.Summary,
	}
}

func issueID(i api.Issue) string {
	if i.IDReadable != "" {
		return i.IDReadable
	}
	return i.ID
}

func issueState(i api.Issue) string {
	for _, f := range i.CustomFields {
		if f.Name == "State" {
			if v, ok := f.ValueObject(); ok && v.Name != "" {
				return v.Name
			}
		}
	}
	return ""
}

func issueReporter(i api.Issue) string {
	if i.Reporter != nil {
		return i.Reporter.Login
	}
	return ""
}

// newIssueCmd создаёт группу команд yt issue (SPEC §3.4).
func newIssueCmd() *cobra.Command {
	issueCmd := &cobra.Command{
		Use:   "issue",
		Short: "Работа с задачами",
		Args:  argsValidator(cobra.NoArgs),
		// Runnable: «yt issue» — help (exit 0); Args — «yt issue <subcommand>» — exit 2 (§4.4).
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	issueCmd.AddCommand(newIssueListCmd())
	return issueCmd
}

// newIssueListCmd создаёт yt issue list (SPEC §3.4): список задач по query YouTrack.
func newIssueListCmd() *cobra.Command {
	var (
		queryFlag string
		state     string
		project   string
		assignee  string
		limit     int
		skip      int
		tags      []string
	)
	cmd := &cobra.Command{
		Use:   "list [<query>]",
		Short: "Список задач",
		Long: "Список задач по поисковому запросу YouTrack (GET /issues).\n" +
			"Позиционный аргумент или -q задают основной запрос.\n" +
			"Флаги -s/-P/-a/--tag добавляют префиксы через AND.\n",
		Args: argsValidator(cobra.MaximumNArgs(1)),
		RunE: runIssueListCmd(&queryFlag, &state, &project, &assignee, &limit, &skip, &tags),
	}
	flags := cmd.Flags()
	flags.StringVarP(&queryFlag, "query", "q", "", "полный поисковый запрос (заменяет позиционный аргумент)")
	flags.StringVarP(&state, "state", "s", "", `состояние: open/resolved/all или значение как есть`)
	flags.StringVarP(&project, "project", "P", "", "проект")
	flags.StringVarP(&assignee, "assignee", "a", "", "исполнитель (login)")
	flags.IntVarP(&limit, "limit", "l", issueListDefaultLimit, "максимум задач (1..100)")
	flags.IntVar(&skip, "skip", 0, "пропустить первых N задач")
	flags.StringArrayVar(&tags, "tag", nil, "тег, можно несколько раз")
	return cmd
}

// runIssueListCmd возвращает RunE для yt issue list (SPEC §3.4).
func runIssueListCmd(queryFlag *string, state, project, assignee *string, limit, skip *int, tags *[]string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *limit < 1 || *limit > issueListMaxLimit {
			return usageError(fmt.Errorf("--limit must be between 1 and %d", issueListMaxLimit))
		}
		if *skip < 0 {
			return usageError(fmt.Errorf("--skip must be non-negative"))
		}

		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		positional := ""
		if len(args) > 0 {
			positional = args[0]
		}
		query := buildIssueQuery(positional, *queryFlag, *state, *project, *assignee, *tags)

		issues, err := client.ListIssues(cmd.Context(), query, api.FieldsIssueList, *limit, *skip)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.JSON() {
			return writeIssueJSON(p, issues)
		}
		return writeIssueTTY(p, issues, query)
	}
}

// writeIssueJSON сериализует список задач в JSON (SPEC §3.4).
func writeIssueJSON(p *output.Printer, issues []api.Issue) error {
	if issues == nil {
		issues = []api.Issue{}
	}
	return p.WriteJSON(issues)
}

// writeIssueTTY печатает таблицу задач (SPEC §3.4) или сообщение при пустом результате.
func writeIssueTTY(p *output.Printer, issues []api.Issue, query string) error {
	if len(issues) == 0 {
		msg := "No issues found"
		if query != "" {
			msg = "No issues found for query " + quoteQuery(query)
		}
		return p.Linef(msg)
	}

	rows := make([][]string, 0, len(issues))
	for _, i := range issues {
		rows = append(rows, issueRow(i))
	}
	headers := []string{"ID", "STATE", "CREATED", "UPDATED", "REPORTER", "SUMMARY"}
	return p.Table(headers, rows)
}

func quoteQuery(q string) string {
	return `"` + q + `"`
}
