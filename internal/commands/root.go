package commands

import (
	"context"

	"github.com/amolofeev/prompt-and-pray/internal/config"
	"github.com/amolofeev/prompt-and-pray/internal/output"
	"github.com/amolofeev/prompt-and-pray/internal/version"
	"github.com/spf13/cobra"
)

type printerKey struct{}

// globalOptions — значения глобальных флагов (§3.1). Здесь хранятся только
// значения, заданные флагами; разрешение приоритета «флаг > env > config >
// дефолт» выполняет pipeline (Атом 2.4, #25).
type globalOptions struct {
	baseURL string
	token   string
	json    bool
	verbose bool
}

// NewRootCommand создаёт корневую команду yt (SPEC §2.3, §3.1): глобальные
// флаги --base-url/--token/--json/--verbose, группы справки Основное/Issues/
// Сервер/Служебное и встроенную команду completion.
func NewRootCommand() *cobra.Command {
	opts := &globalOptions{}

	cmd := &cobra.Command{
		Use:           "yt",
		Short:         "Клиент для локального сервера YouTrack",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Version,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			mode := output.ModeTTY
			if opts.json {
				mode = output.ModeJSON
			}
			printer := output.New(cmd.OutOrStdout(), cmd.ErrOrStderr(), mode)
			cmd.SetContext(context.WithValue(cmd.Context(), printerKey{}, printer))
			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVar(&opts.baseURL, "base-url", "", "базовый URL API (по умолчанию из config/env, иначе "+config.DefaultBaseURL+")")
	flags.StringVar(&opts.token, "token", "", "permanent token (по умолчанию из config/env)")
	flags.BoolVar(&opts.json, "json", false, "выводить результат в машинном формате JSON")
	flags.BoolVar(&opts.verbose, "verbose", false, "подробный лог в stderr (уровень debug)")

	cmd.AddGroup(
		&cobra.Group{ID: "core", Title: "Основное"},
		&cobra.Group{ID: "issues", Title: "Issues"},
		&cobra.Group{ID: "server", Title: "Сервер"},
		&cobra.Group{ID: "utility", Title: "Служебное"},
	)
	cmd.SetHelpCommandGroupID("utility")
	cmd.SetCompletionCommandGroupID("utility")

	versionCmd := newVersionCmd()
	versionCmd.GroupID = "utility"
	cmd.AddCommand(versionCmd)

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
