package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/amolofeev/prompt-and-pray/internal/api"
	"github.com/amolofeev/prompt-and-pray/internal/output"
	"github.com/spf13/cobra"
)

// closeDefaultState — состояние разрешения по умолчанию (SPEC §3.4).
const closeDefaultState = "Fixed"

// newIssueCloseCmd создаёт yt issue close (SPEC §3.4): закрытие одной или
// нескольких задач через командный язык YouTrack (POST /commands). Все id
// применяются одним запросом.
func newIssueCloseCmd() *cobra.Command {
	var (
		state   string
		message string
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "close <id>...",
		Short: "Закрыть задачу",
		Long: "Закрытие (перевод в resolved-состояние) одной или нескольких задач\n" +
			"через командный язык YouTrack (POST /commands).\n" +
			"<id> — ring-id или idReadable; команда применяется одним запросом.\n",
		Args: argsValidator(cobra.MinimumNArgs(1)),
		RunE: runIssueCloseCmd(&state, &message, &yes),
	}
	flags := cmd.Flags()
	flags.StringVarP(&state, "state", "s", closeDefaultState, "состояние разрешения (по умолчанию Fixed)")
	flags.StringVarP(&message, "message", "m", "", "комментарий, добавляемый вместе с командой")
	flags.BoolVarP(&yes, "yes", "y", false, "не запрашивать подтверждение")
	return cmd
}

// runIssueCloseCmd возвращает RunE для yt issue close (SPEC §3.4): разбор id
// по эвристике §4.1, подтверждение при TTY (если нет -y), POST /commands
// одним запросом, вывод обновлённых задач.
func runIssueCloseCmd(state, message *string, yes *bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		refs := make([]api.IssueRef, 0, len(args))
		for _, id := range args {
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
			ok, err := confirmClose(cmd, *state, len(refs))
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("Aborted")
			}
		}

		query := "state: " + *state
		res, err := client.ApplyCommand(cmd.Context(), query, *message, "", refs, api.FieldsCommandIssues)
		if err != nil {
			return err
		}

		if p.JSON() {
			return writeCloseJSON(p, res)
		}
		return writeCloseTTY(p, res, *state)
	}
}

// confirmClose печатает предупреждение в stderr и запрашивает подтверждение
// (SPEC §3.4): «! This will close N issue(s) via command "<query>". Continue?
// [y/N] ». Возвращает true при «y»/«yes» (без учёта регистра).
func confirmClose(cmd *cobra.Command, state string, count int) (bool, error) {
	prompt := fmt.Sprintf("! This will close %d issue(s) via command %q. Continue? [y/N] ", count, "state: "+state)
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

// writeCloseJSON печатает результат yt issue close в JSON (SPEC §3.4):
// массив issues из ответа POST /commands.
func writeCloseJSON(p *output.Printer, res *api.CommandList) error {
	issues := res.Issues
	if issues == nil {
		issues = []api.Issue{}
	}
	return p.WriteJSON(issues)
}

// writeCloseTTY печатает результат yt issue close в TTY (SPEC §3.4): для
// каждой обновлённой задачи строка «✓ <idReadable> → <state>».
func writeCloseTTY(p *output.Printer, res *api.CommandList, state string) error {
	for _, it := range res.Issues {
		if err := p.Successf("%s → %s", issueID(it), state); err != nil {
			return err
		}
	}
	return nil
}
