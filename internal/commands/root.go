package commands

import (
	"context"

	"github.com/amolofeev/prompt-and-pray/internal/output"
	"github.com/amolofeev/prompt-and-pray/internal/version"
	"github.com/spf13/cobra"
)

type printerKey struct{}

// NewRootCommand создаёт корневую команду yt. Здесь — минимальный каркас,
// достаточный для работы yt version (SPEC §3.11): флаг --json и автогенерация
// --version. Глобальные флаги, группы справки и completion — Атом 2.3 (#22).
func NewRootCommand() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:           "yt",
		Short:         "Клиент для локального сервера YouTrack",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Version,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			mode := output.ModeTTY
			if jsonOut {
				mode = output.ModeJSON
			}
			printer := output.New(cmd.OutOrStdout(), cmd.ErrOrStderr(), mode)
			cmd.SetContext(context.WithValue(cmd.Context(), printerKey{}, printer))
			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "выводить результат в машинном формате JSON")

	cmd.AddCommand(newVersionCmd())

	return cmd
}

// Execute запускает CLI.
func Execute() error {
	return NewRootCommand().Execute()
}

// printer возвращает Printer команды. Если pipeline ещё не инициализировал
// контекст (например, при прямом вызове команды в тесте), создаётся Printer
// в режиме TTY с выводом команды.
func printer(cmd *cobra.Command) *output.Printer {
	if p, ok := cmd.Context().Value(printerKey{}).(*output.Printer); ok {
		return p
	}
	return output.New(cmd.OutOrStdout(), cmd.ErrOrStderr(), output.ModeTTY)
}
