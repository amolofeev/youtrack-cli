package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newIssueDeleteCmd создаёт yt issue delete (SPEC §3.4): удаление задачи
// (DELETE /issues/{id}). При TTY запрашивает подтверждение, если нет -y.
func newIssueDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Удалить задачу",
		Long: "Удаление задачи (DELETE /issues/{id}).\n" +
			"<id> — ring-id или idReadable.\n" +
			"Без -y/--yes в TTY запрашивается подтверждение.\n",
		Args: argsValidator(cobra.ExactArgs(1)),
		RunE: runIssueDeleteCmd(&yes),
	}
	flags := cmd.Flags()
	flags.BoolVarP(&yes, "yes", "y", false, "не запрашивать подтверждение")
	return cmd
}

// runIssueDeleteCmd возвращает RunE для yt issue delete (SPEC §3.4):
// подтверждение при TTY (если нет -y), DELETE /issues/{id}, вывод результата.
func runIssueDeleteCmd(yes *bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		id := args[0]

		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.TTY() && !*yes {
			ok, err := confirmDelete(cmd, id)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("Aborted")
			}
		}

		if err := client.DeleteIssue(cmd.Context(), id); err != nil {
			return err
		}
		return p.Successf("Deleted issue %s", id)
	}
}

// confirmDelete печатает предупреждение в stderr и запрашивает подтверждение
// (SPEC §3.4): «! Warning: this will permanently delete PRJ-42. Continue?
// [y/N] ». Возвращает true при «y»/«yes» (без учёта регистра).
func confirmDelete(cmd *cobra.Command, id string) (bool, error) {
	prompt := fmt.Sprintf("! Warning: this will permanently delete %s. Continue? [y/N] ", id)
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
