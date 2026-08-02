package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/amolofeev/youtrack-cli/internal/api"
	"github.com/amolofeev/youtrack-cli/internal/output"
	"github.com/spf13/cobra"
)

// newCommandCmd создаёт yt command (SPEC §3.6): применение командного языка
// YouTrack к одной или нескольким задачам (POST /commands). Первый аргумент —
// команда («state: Fixed Priority: High»), остальные — id задач. Все id
// применяются одним запросом. Подкоманда assist — подсказки без применения.
func newCommandCmd() *cobra.Command {
	var (
		message string
		runAs   string
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "command <commands> <id>...",
		Short: "Применить команды YouTrack к задачам",
		Long: "Применение командного языка YouTrack (POST /commands) к одной или\n" +
			"нескольким задачам одним запросом. Первый аргумент — команда\n" +
			"(например \"state: Fixed Priority: High\"), далее — id задач.\n" +
			"<id> — ring-id или idReadable.\n",
		Args: argsValidator(cobra.MinimumNArgs(2)),
		RunE: runCommandCmd(&message, &runAs, &yes),
	}
	flags := cmd.Flags()
	flags.StringVarP(&message, "message", "m", "", "комментарий, добавляемый вместе с командой")
	flags.StringVar(&runAs, "run-as", "", "исполнить команду от имени другого пользователя (если разрешено)")
	flags.BoolVarP(&yes, "yes", "y", false, "не запрашивать подтверждение")
	cmd.AddCommand(newCommandAssistCmd())
	return cmd
}

// newCommandAssistCmd создаёт yt command assist (SPEC §3.6): предварительный
// разбор команды YouTrack и подсказки без применения (POST /commands/assist).
// Аргумент — команда или её часть. В v1 — только вывод подсказок, без
// интерактивного UI.
func newCommandAssistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assist <commands>",
		Short: "Подсказки по командному языку YouTrack",
		Long: "Предварительный разбор команды YouTrack и подсказки без применения\n" +
			"(POST /commands/assist). Аргумент — команда или её часть\n" +
			"(например \"state: \" или \"tag: \"); выводятся подсказки\n" +
			"«OK: <команда> — <описание>». В v1 — только вывод, без интерактивного UI.\n",
		Args: argsValidator(cobra.ExactArgs(1)),
		RunE: runCommandAssistCmd(),
	}
	return cmd
}

// runCommandAssistCmd возвращает RunE для yt command assist (SPEC §3.6):
// POST /commands/assist с полями FieldsCommandAssist и командой как есть.
func runCommandAssistCmd() func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		query := args[0]
		res, err := client.CommandAssist(cmd.Context(), query, api.FieldsCommandAssist)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.JSON() {
			return p.WriteJSON(res)
		}
		return writeCommandAssistTTY(p, res, query)
	}
}

// writeCommandAssistTTY печатает подсказки командного языка (SPEC §3.6):
// для каждой подсказки строка «OK: <команда> — <описание>». Пустой ответ —
// «No suggestions for "<query>"».
func writeCommandAssistTTY(p *output.Printer, res *api.CommandList, query string) error {
	if len(res.Suggestions) == 0 {
		return p.Linef("No suggestions for %s", quoteQuery(query))
	}
	for _, s := range res.Suggestions {
		if err := p.Linef("OK: %s — %s", s.Option, s.Description); err != nil {
			return err
		}
	}
	return nil
}

// runCommandCmd возвращает RunE для yt command (SPEC §3.6): разбор id по
// эвристике §4.1, подтверждение при TTY (если нет -y), POST /commands одним
// запросом, вывод обновлённых задач.
func runCommandCmd(message, runAs *string, yes *bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		query := args[0]
		refs := make([]api.IssueRef, 0, len(args)-1)
		for _, id := range args[1:] {
			ref, err := parseIssueRef(id)
			if err != nil {
				return err
			}
			refs = append(refs, ref)
		}

		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.TTY() && !*yes {
			ok, err := confirmCommand(cmd, query, len(refs))
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("Aborted")
			}
		}

		res, err := client.ApplyCommand(cmd.Context(), query, *message, *runAs, refs, api.FieldsCommandIssues)
		if err != nil {
			return err
		}

		if p.JSON() {
			return writeCommandJSON(p, res)
		}
		return writeCommandTTY(p, res, query)
	}
}

// confirmCommand печатает предупреждение в stderr и запрашивает подтверждение
// (SPEC §3.6): «! This will apply command "<query>" to N issue(s). Continue?
// [y/N] ». Возвращает true при «y»/«yes» (без учёта регистра).
func confirmCommand(cmd *cobra.Command, query string, count int) (bool, error) {
	prompt := fmt.Sprintf("! This will apply command %q to %d issue(s). Continue? [y/N] ", query, count)
	fmt.Fprint(cmd.ErrOrStderr(), prompt)
	line, err := readLineStdin(cmd)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

// writeCommandJSON печатает результат yt command в JSON (SPEC §3.6): массив
// issues из ответа POST /commands.
func writeCommandJSON(p *output.Printer, res *api.CommandList) error {
	issues := res.Issues
	if issues == nil {
		issues = []api.Issue{}
	}
	return p.WriteJSON(issues)
}

// writeCommandTTY печатает результат yt command в TTY (SPEC §3.6): для каждой
// обновлённой задачи строка «✓ <idReadable>: <применённые команды>».
func writeCommandTTY(p *output.Printer, res *api.CommandList, query string) error {
	rendered := formatCommandQuery(query)
	for _, it := range res.Issues {
		if err := p.Successf("%s: %s", issueID(it), rendered); err != nil {
			return err
		}
	}
	return nil
}

// formatCommandQuery превращает командную строку в отображаемый вид для yt
// command (SPEC §3.6): «state: Fixed Priority: High» → «state → Fixed,
// Priority → High». Каждая пара «name: value...» показывается как
// «name → value», пары разделяются «, ». Если команд не распознано —
// возвращается исходная строка.
func formatCommandQuery(query string) string {
	tokens := strings.Fields(query)
	var parts []string
	var name string
	var values []string
	for _, t := range tokens {
		if strings.HasSuffix(t, ":") {
			if name != "" {
				parts = append(parts, formatCommandPart(name, values))
			}
			name = strings.TrimSuffix(t, ":")
			values = nil
			continue
		}
		if name != "" {
			values = append(values, t)
		}
	}
	if name != "" {
		parts = append(parts, formatCommandPart(name, values))
	}
	if len(parts) == 0 {
		return query
	}
	return strings.Join(parts, ", ")
}

// formatCommandPart собирает пару «name → value»: при пустом значении — только
// имя команды.
func formatCommandPart(name string, values []string) string {
	if len(values) == 0 {
		return name
	}
	return name + " → " + strings.Join(values, " ")
}
