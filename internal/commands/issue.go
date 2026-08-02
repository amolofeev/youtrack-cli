package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/amolofeev/youtrack-cli/internal/api"
	"github.com/amolofeev/youtrack-cli/internal/output"
	"github.com/spf13/cobra"
)

const issueListDefaultLimit = 30
const issueListMaxLimit = 100

const issueViewDefaultCommentsLimit = 20

var (
	// issueViewSeparator — разделитель секций в yt issue view (SPEC §3.4, §4.3).
	issueViewSeparator = strings.Repeat("─", 64)
	// issueViewCommentRule — разделитель под секцией Comments.
	issueViewCommentRule = strings.Repeat("─", 11)
)

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
	issueCmd.AddCommand(newIssueViewCmd())
	issueCmd.AddCommand(newIssueCreateCmd())
	issueCmd.AddCommand(newIssueEditCmd())
	issueCmd.AddCommand(newIssueCloseCmd())
	issueCmd.AddCommand(newIssueDeleteCmd())
	issueCmd.AddCommand(newIssueCommentCmd())
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

// newIssueViewCmd создаёт yt issue view (SPEC §3.4): просмотр задачи по
// ring-id или idReadable. -c запрашивает комментарии отдельным GET-запросом.
func newIssueViewCmd() *cobra.Command {
	var (
		comments      bool
		commentsLimit int
	)
	cmd := &cobra.Command{
		Use:   "view <id>",
		Short: "Просмотр задачи",
		Long: "Просмотр задачи по id (ring-id или idReadable, GET /issues/{id}).\n" +
			"-c запрашивает комментарии (GET /issues/{id}/comments, $top=--comments-limit).\n",
		Args: argsValidator(cobra.ExactArgs(1)),
		RunE: runIssueViewCmd(&comments, &commentsLimit),
	}
	flags := cmd.Flags()
	flags.BoolVarP(&comments, "comments", "c", false, "вывести комментарии после тела задачи")
	flags.IntVarP(&commentsLimit, "comments-limit", "C", issueViewDefaultCommentsLimit, "максимум комментариев (1..)")
	return cmd
}

// runIssueViewCmd возвращает RunE для yt issue view (SPEC §3.4): GET
// /issues/{id} (+ /issues/{id}/comments при -c, $top=--comments-limit).
func runIssueViewCmd(comments *bool, commentsLimit *int) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *comments && *commentsLimit < 1 {
			return usageError(fmt.Errorf("--comments-limit must be at least 1"))
		}

		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		id := args[0]
		it, err := client.Issue(cmd.Context(), id, api.FieldsIssueView)
		if err != nil {
			return err
		}

		if *comments {
			comments, err := client.IssueComments(cmd.Context(), id, api.FieldsIssueComments, *commentsLimit, 0)
			if err != nil {
				return err
			}
			it.Comments = comments
			if it.Comments == nil {
				it.Comments = []api.IssueComment{}
			}
		}

		p := printer(cmd)
		if p.JSON() {
			return writeIssueViewJSON(p, it)
		}
		return writeIssueViewTTY(p, it)
	}
}

// issueViewJSON — JSON-представление yt issue view с комментариями: сырой
// объект Issue + ключ comments (исключение §4.3). Поле comments выводится
// всегда, даже при пустом списке, в отличие от встроенного в Issue.
type issueViewJSON struct {
	api.Issue
	Comments []api.IssueComment `json:"comments"`
}

// writeIssueViewJSON печатает задачу в JSON (SPEC §3.4): без --comments —
// сырой объект Issue (comments не включён); с --comments — тот же объект
// с добавленным ключом comments.
func writeIssueViewJSON(p *output.Printer, it *api.Issue) error {
	if it.Comments != nil {
		return p.WriteJSON(issueViewJSON{Issue: *it, Comments: it.Comments})
	}
	return p.WriteJSON(it)
}

// formatDateTime преобразует unix timestamp в миллисекундах в "YYYY-MM-DD HH:MM".
func formatDateTime(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format("2006-01-02 15:04")
}

// issueViewMeta1 собирает строку метаданных задачи (SPEC §3.4): STATE из
// кастомного поля, PROJECT — shortName (иначе name), REPORTER — login.
// Пустые блоки опускаются.
func issueViewMeta1(i api.Issue) string {
	var parts []string
	if v := issueState(i); v != "" {
		parts = append(parts, "STATE: "+v)
	}
	if i.Project != nil {
		proj := i.Project.ShortName
		if proj == "" {
			proj = i.Project.Name
		}
		if proj != "" {
			parts = append(parts, "PROJECT: "+proj)
		}
	}
	if i.Reporter != nil && i.Reporter.Login != "" {
		parts = append(parts, "REPORTER: "+i.Reporter.Login)
	}
	return strings.Join(parts, "  ")
}

// issueViewMeta2 собирает строку дат задачи (SPEC §3.4): CREATED и UPDATED
// в формате "YYYY-MM-DD HH:MM". Нулевые даты опускаются.
func issueViewMeta2(i api.Issue) string {
	var parts []string
	if v := formatDateTime(i.Created); v != "" {
		parts = append(parts, "CREATED: "+v)
	}
	if v := formatDateTime(i.Updated); v != "" {
		parts = append(parts, "UPDATED: "+v)
	}
	return strings.Join(parts, "  ")
}

// issueViewTags собирает строку тегов задачи (SPEC §3.4). Пусто, если тегов нет.
func issueViewTags(i api.Issue) string {
	names := make([]string, 0, len(i.Tags))
	for _, t := range i.Tags {
		if t.Name != "" {
			names = append(names, t.Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "Tags: " + strings.Join(names, ", ")
}

// commentAuthor возвращает имя автора комментария: login, иначе fullName,
// иначе "unknown".
func commentAuthor(c api.IssueComment) string {
	if c.Author != nil {
		if c.Author.Login != "" {
			return c.Author.Login
		}
		if c.Author.FullName != "" {
			return c.Author.FullName
		}
	}
	return "unknown"
}

// writeIssueViewTTY печатает задачу в TTY-формате §3.4: шапка, разделители
// ─, метаданные, теги, описание (или «No description») и, при --comments,
// секцию Comments.
func writeIssueViewTTY(p *output.Printer, it *api.Issue) error {
	if p.Page() {
		defer func() { _ = p.EndPage() }()
	}
	header := issueID(*it)
	if it.Summary != "" {
		header += "  " + it.Summary
	}
	lines := []string{header, issueViewSeparator}
	meta := make([]string, 0, 3)
	if v := issueViewMeta1(*it); v != "" {
		meta = append(meta, v)
	}
	if v := issueViewMeta2(*it); v != "" {
		meta = append(meta, v)
	}
	if v := issueViewTags(*it); v != "" {
		meta = append(meta, v)
	}
	lines = append(lines, meta...)
	if len(meta) > 0 {
		lines = append(lines, issueViewSeparator)
	}
	desc := it.Description
	if desc == "" {
		desc = "No description"
	}
	lines = append(lines, desc)
	for _, line := range lines {
		if err := p.Linef("%s", line); err != nil {
			return err
		}
	}
	return writeIssueCommentsTTY(p, it.Comments)
}

// writeIssueCommentsTTY печатает секцию Comments (SPEC §3.4). comments == nil
// означает, что --comments не задан и секция не выводится.
func writeIssueCommentsTTY(p *output.Printer, comments []api.IssueComment) error {
	if comments == nil {
		return nil
	}
	if err := p.Linef(""); err != nil {
		return err
	}
	if err := p.Linef("Comments (%d):", len(comments)); err != nil {
		return err
	}
	if err := p.Linef("%s", issueViewCommentRule); err != nil {
		return err
	}
	for i, c := range comments {
		line := commentAuthor(c)
		if when := formatDateTime(c.Created); when != "" {
			line += " · " + when
		}
		if err := p.Linef("%s", line); err != nil {
			return err
		}
		if c.Text != "" {
			for _, l := range strings.Split(strings.TrimSuffix(c.Text, "\n"), "\n") {
				if err := p.Linef("%s", l); err != nil {
					return err
				}
			}
		}
		if i < len(comments)-1 {
			if err := p.Linef(""); err != nil {
				return err
			}
		}
	}
	return nil
}

// commentListDefaultLimit — дефолт --limit для yt issue comment list (SPEC §3.4).
const commentListDefaultLimit = 30

// commentListRule — разделитель между комментариями в yt issue comment list
// (SPEC §3.4, пример «────»).
var commentListRule = strings.Repeat("─", 4)

// newIssueCommentCmd создаёт группу команд yt issue comment (SPEC §3.4).
func newIssueCommentCmd() *cobra.Command {
	commentCmd := &cobra.Command{
		Use:   "comment",
		Short: "Комментарии к задаче",
		Args:  argsValidator(cobra.NoArgs),
		// Runnable: «yt issue comment» — help (exit 0); Args — неизвестная
		// подкоманда — exit 2 (§4.4).
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	commentCmd.AddCommand(newIssueCommentListCmd())
	commentCmd.AddCommand(newIssueCommentCreateCmd())
	return commentCmd
}

// newIssueCommentListCmd создаёт yt issue comment list (SPEC §3.4): список
// комментариев задачи (GET /issues/{id}/comments).
func newIssueCommentListCmd() *cobra.Command {
	var (
		limit int
		skip  int
	)
	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "Список комментариев",
		Long: "Список комментариев задачи (GET /issues/{id}/comments).\n" +
			"<id> — ring-id или idReadable.\n",
		Args: argsValidator(cobra.ExactArgs(1)),
		RunE: runIssueCommentListCmd(&limit, &skip),
	}
	flags := cmd.Flags()
	flags.IntVar(&limit, "limit", commentListDefaultLimit, "максимум комментариев (1..)")
	flags.IntVar(&skip, "skip", 0, "пропустить первых N комментариев")
	return cmd
}

// runIssueCommentListCmd возвращает RunE для yt issue comment list (SPEC §3.4):
// GET /issues/{id}/comments с полями FieldsIssueComments и $top/$skip.
func runIssueCommentListCmd(limit, skip *int) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *limit < 1 {
			return usageError(fmt.Errorf("--limit must be at least 1"))
		}
		if *skip < 0 {
			return usageError(fmt.Errorf("--skip must be non-negative"))
		}

		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		id := args[0]
		comments, err := client.IssueComments(cmd.Context(), id, api.FieldsIssueComments, *limit, *skip)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.JSON() {
			return writeCommentsJSON(p, comments)
		}
		return writeCommentListTTY(p, comments, id)
	}
}

// writeCommentsJSON сериализует список комментариев в JSON (SPEC §3.4).
func writeCommentsJSON(p *output.Printer, comments []api.IssueComment) error {
	if comments == nil {
		comments = []api.IssueComment{}
	}
	return p.WriteJSON(comments)
}

// writeCommentListTTY печатает список комментариев (SPEC §3.4): для каждого —
// строка «автор · дата», текст, разделитель ──── между комментариями. Пустой
// список — «No comments for <id>».
func writeCommentListTTY(p *output.Printer, comments []api.IssueComment, id string) error {
	if len(comments) == 0 {
		return p.Linef("No comments for %s", id)
	}
	if p.Page() {
		defer func() { _ = p.EndPage() }()
	}
	for i, c := range comments {
		line := commentAuthor(c)
		if when := formatDateTime(c.Created); when != "" {
			line += " · " + when
		}
		if err := p.Linef("%s", line); err != nil {
			return err
		}
		if c.Text != "" {
			for _, l := range strings.Split(strings.TrimSuffix(c.Text, "\n"), "\n") {
				if err := p.Linef("%s", l); err != nil {
					return err
				}
			}
		}
		if i < len(comments)-1 {
			if err := p.Linef("%s", commentListRule); err != nil {
				return err
			}
		}
	}
	return nil
}

// newIssueCommentCreateCmd создаёт yt issue comment create (SPEC §3.4):
// добавление комментария (POST /issues/{id}/comments). Текст — -m/--message
// либо --editor.
func newIssueCommentCreateCmd() *cobra.Command {
	var (
		message string
		editor  bool
	)
	cmd := &cobra.Command{
		Use:   "create <id>",
		Short: "Добавить комментарий",
		Long: "Добавление комментария к задаче (POST /issues/{id}/comments).\n" +
			"<id> — ring-id или idReadable.\n" +
			"Текст: -m/--message либо --editor ($EDITOR или vi).\n",
		Args: argsValidator(cobra.ExactArgs(1)),
		RunE: runIssueCommentCreateCmd(&message, &editor),
	}
	flags := cmd.Flags()
	flags.StringVarP(&message, "message", "m", "", "текст комментария")
	flags.BoolVar(&editor, "editor", false, "ввод текста через $EDITOR (или vi)")
	return cmd
}

// runIssueCommentCreateCmd возвращает RunE для yt issue comment create
// (SPEC §3.4): валидация флагов, ввод текста, POST /issues/{id}/comments,
// вывод результата.
func runIssueCommentCreateCmd(message *string, editor *bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *editor && *message != "" {
			return usageError(errors.New("--message and --editor are mutually exclusive"))
		}
		if *message == "" && !*editor {
			return usageError(errors.New("--message is required (or use --editor)"))
		}

		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		text := *message
		if *editor {
			text, err = editCommentText(cmd)
			if err != nil {
				return err
			}
			if text == "" {
				return errors.New("no comment text provided")
			}
		}

		id := args[0]
		comment, err := client.CreateComment(cmd.Context(), id, text, api.FieldsIssueCommentCreate)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.JSON() {
			return p.WriteJSON(comment)
		}
		return p.Successf("Added comment to %s", id)
	}
}

// editCommentText открывает редактор с пустым буфером над временным файлом и
// возвращает введённый текст комментария (SPEC §3.4). Возвращает пустую
// строку, если редактор оставил буфер пустым.
func editCommentText(cmd *cobra.Command) (string, error) {
	f, err := os.CreateTemp("", "yt-comment-*.txt")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(f.Name())
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}
	if err := runEditor(cmd, f.Name()); err != nil {
		return "", err
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		return "", fmt.Errorf("read edited file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// projectRingID — паттерн ring-id проекта (SPEC §3.4): «0-0» и т.п. При
// совпадении --project используется как есть, без резолвинга.
var projectRingID = regexp.MustCompile(`^[0-9]+-[0-9]+$`)

// newIssueCreateCmd создаёт yt issue create (SPEC §3.4): создание задачи.
// --project резолвится в ring-id проекта, текст берётся из флагов или --editor.
func newIssueCreateCmd() *cobra.Command {
	var (
		project string
		title   string
		body    string
		editor  bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Создать задачу",
		Long: "Создание задачи (POST /issues).\n" +
			"--project: shortName, имя или ring-id проекта (обязателен).\n" +
			"Текст: --title/--body либо --editor ($EDITOR или vi).\n",
		Args: argsValidator(cobra.NoArgs),
		RunE: runIssueCreateCmd(&project, &title, &body, &editor),
	}
	flags := cmd.Flags()
	flags.StringVarP(&project, "project", "p", "", "проект: shortName, имя или ring-id")
	flags.StringVarP(&title, "title", "t", "", "summary задачи")
	flags.StringVarP(&body, "body", "b", "", "description задачи (взаимоисключающе с --editor)")
	flags.BoolVar(&editor, "editor", false, "ввод текста через $EDITOR (или vi)")
	return cmd
}

// runIssueCreateCmd возвращает RunE для yt issue create (SPEC §3.4):
// валидация флагов, резолвинг проекта, POST /issues, вывод результата.
func runIssueCreateCmd(project, title, body *string, editor *bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *editor && *body != "" {
			return usageError(errors.New("--body and --editor are mutually exclusive"))
		}
		if *project == "" {
			return usageError(errors.New("--project is required"))
		}
		if *title == "" && !*editor {
			return usageError(errors.New("--title is required (or use --editor)"))
		}

		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		summary, description := *title, *body
		if *editor {
			summary, description, err = editIssueText(cmd, summary)
			if err != nil {
				return err
			}
			if summary == "" {
				return errors.New("no summary provided")
			}
		}

		projectID, err := resolveProject(cmd.Context(), client, *project)
		if err != nil {
			return err
		}

		it, err := client.CreateIssue(cmd.Context(), projectID, summary, description, api.FieldsIssueCreate)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.JSON() {
			return p.WriteJSON(it)
		}
		return p.Successf("Created issue %s: %s", issueID(*it), it.Summary)
	}
}

// newIssueEditCmd создаёт yt issue edit (SPEC §3.4): частичное обновление
// задачи (POST /issues/{id}). --title/--body — хотя бы один обязателен.
func newIssueEditCmd() *cobra.Command {
	var (
		title string
		body  string
	)
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Изменить задачу",
		Long: "Изменение задачи (POST /issues/{id}).\n" +
			"<id> — ring-id или idReadable.\n" +
			"Частичное обновление: изменяется только то, что передано\n" +
			"через --title и/или --body (хотя бы один обязателен).\n",
		Args: argsValidator(cobra.ExactArgs(1)),
		RunE: runIssueEditCmd(&title, &body),
	}
	flags := cmd.Flags()
	flags.StringVar(&title, "title", "", "новый summary")
	flags.StringVar(&body, "body", "", "новый description")
	return cmd
}

// runIssueEditCmd возвращает RunE для yt issue edit (SPEC §3.4): валидация
// «хотя бы один из --title/--body», частичное обновление (nil — поле не
// меняется), POST /issues/{id}, вывод результата.
func runIssueEditCmd(title, body *string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *title == "" && *body == "" {
			return usageError(errors.New("at least one of --title, --body is required"))
		}

		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		var summary, description *string
		if *title != "" {
			summary = title
		}
		if *body != "" {
			description = body
		}

		id := args[0]
		it, err := client.UpdateIssue(cmd.Context(), id, api.FieldsIssueEdit, summary, description)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.JSON() {
			return p.WriteJSON(it)
		}
		return p.Successf("Updated issue %s", issueID(*it))
	}
}

// resolveProject резолвит значение --project в ring-id (SPEC §3.4): ring-id
// (^[0-9]+-[0-9]+$) используется как есть; иначе GET /admin/projects с
// клиентской фильтрацией — совпадение по shortName без учёта регистра, затем
// по name. Не найдено — ошибка «project <value> not found».
func resolveProject(ctx context.Context, client *api.Client, value string) (string, error) {
	if projectRingID.MatchString(value) {
		return value, nil
	}
	projects, err := client.ListProjects(ctx, api.FieldsProjectResolve, 200, 0)
	if err != nil {
		return "", err
	}
	for _, p := range projects {
		if strings.EqualFold(p.ShortName, value) {
			return p.ID, nil
		}
	}
	for _, p := range projects {
		if strings.EqualFold(p.Name, value) {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("project %s not found", value)
}

// editorTemplate возвращает стартовый шаблон редактора (SPEC §3.4): строка
// Summary: предзаполняется title (если задан), Description: — пустая.
func editorTemplate(title string) string {
	return "Summary: " + title + "\n\nDescription:\n"
}

// parseEditorContent разбирает содержимое файла редактора (SPEC §3.4):
// строка «Summary: <текст>» — summary, всё после строки «Description:» —
// description (может быть пустым).
func parseEditorContent(content string) (summary, description string) {
	lines := strings.Split(content, "\n")
	descIdx := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "Description:") {
			descIdx = i
			break
		}
	}
	if descIdx >= 0 {
		description = strings.TrimSpace(strings.Join(lines[descIdx+1:], "\n"))
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "Summary:") {
			summary = strings.TrimSpace(strings.TrimPrefix(l, "Summary:"))
			break
		}
	}
	return summary, description
}

// editIssueText открывает редактор с шаблоном Summary:/Description: над
// временным файлом и возвращает введённые summary и description (SPEC §3.4).
func editIssueText(cmd *cobra.Command, title string) (string, string, error) {
	f, err := os.CreateTemp("", "yt-issue-*.txt")
	if err != nil {
		return "", "", fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(editorTemplate(title)); err != nil {
		_ = f.Close()
		return "", "", fmt.Errorf("write template: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", "", fmt.Errorf("close temp file: %w", err)
	}
	if err := runEditor(cmd, f.Name()); err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		return "", "", fmt.Errorf("read edited file: %w", err)
	}
	summary, description := parseEditorContent(string(data))
	return summary, description, nil
}

// runEditor запускает $EDITOR (или vi) над файлом path, наследуя потоки
// команды. Ошибка редактора возвращается обёрнутой (exit 1).
func runEditor(cmd *cobra.Command, path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	c := exec.Command(parts[0], append(parts[1:], path)...)
	c.Stdin = cmd.InOrStdin()
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	if err := c.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}
	return nil
}
