package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amolofeev/yt/internal/api"
	"github.com/amolofeev/yt/internal/config"
	"github.com/amolofeev/yt/internal/output"
	"github.com/amolofeev/yt/internal/version"
	"github.com/spf13/cobra"
)

type printerKey struct{}
type configKey struct{}
type clientKey struct{}

// globalOptions — значения глобальных флагов (§3.1). Здесь хранятся только
// значения, заданные флагами; разрешение приоритета «флаг > env > config >
// дефолт» выполняет pipeline (§3.2, Атом 2.4).
type globalOptions struct {
	baseURL string
	token   string
	json    bool
	verbose bool
}

// NewRootCommand создаёт корневую команду yt (SPEC §2.3, §3.1): глобальные
// флаги --base-url/--token/--json/--verbose, группы справки Основное/Issues/
// Сервер/Служебное, встроенную команду completion и пайплайн (§2.1) в
// PersistentPreRunE.
func NewRootCommand() *cobra.Command {
	opts := &globalOptions{}

	cmd := &cobra.Command{
		Use:           "yt",
		Short:         "Клиент для локального сервера YouTrack",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Version,
		Args:          unknownCommandArgs,
		// Root runnable, чтобы валидация аргументов срабатывала и для
		// неизвестных подкоманд (exit 2, §4.4); «yt» без аргументов печатает help.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			mode := output.ModeTTY
			if opts.json {
				mode = output.ModeJSON
			}
			printer := output.New(cmd.OutOrStdout(), cmd.ErrOrStderr(), mode, output.WithVerbose(opts.verbose))
			ctx := context.WithValue(cmd.Context(), printerKey{}, printer)
			cmd.SetContext(ctx)
			return runPipeline(cmd, opts)
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVar(&opts.baseURL, "base-url", "", "базовый URL API (по умолчанию из config/env, иначе "+config.DefaultBaseURL+")")
	flags.StringVar(&opts.token, "token", "", "permanent token (по умолчанию из config/env)")
	flags.BoolVar(&opts.json, "json", false, "выводить результат в машинном формате JSON")
	flags.BoolVar(&opts.verbose, "verbose", false, "подробный лог в stderr (уровень debug)")

	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError(err)
	})

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

	authCmd := newAuthCmd(opts)
	authCmd.GroupID = "core"
	cmd.AddCommand(authCmd)

	userCmd := newUserCmd()
	userCmd.GroupID = "core"
	cmd.AddCommand(userCmd)

	issueCmd := newIssueCmd()
	issueCmd.GroupID = "issues"
	cmd.AddCommand(issueCmd)

	searchCmd := newSearchCmd()
	searchCmd.GroupID = "issues"
	cmd.AddCommand(searchCmd)

	commandCmd := newCommandCmd()
	commandCmd.GroupID = "issues"
	cmd.AddCommand(commandCmd)

	projectCmd := newProjectCmd()
	projectCmd.GroupID = "server"
	cmd.AddCommand(projectCmd)

	tagCmd := newTagCmd()
	tagCmd.GroupID = "server"
	cmd.AddCommand(tagCmd)

	return cmd
}

// runPipeline выполняет шаг «разбор флагов → загрузка конфигурации → клиент»
// (SPEC §2.1). Значения разрешаются по приоритету флаг > env > config > дефолт
// (§3.2); клиент кладётся в контекст вместе с resolved-конфигурацией.
func runPipeline(cmd *cobra.Command, opts *globalOptions) error {
	cfg, err := config.Resolve(opts.baseURL, opts.token)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cfg.BaseURL, cfg.Token, cmd.ErrOrStderr(), opts.verbose)
	if err != nil {
		return err
	}
	ctx := context.WithValue(cmd.Context(), configKey{}, cfg)
	ctx = context.WithValue(ctx, clientKey{}, client)
	cmd.SetContext(ctx)
	return nil
}

// newAPIClient создаёт API-клиент с таймаутом и логгером из окружения (§4.5–4.6).
// Используется pipeline и командами, которым нужен клиент с другим токеном
// (auth login проверяет введённый токен до сохранения, §3.3).
func newAPIClient(baseURL, token string, w io.Writer, verbose bool) (*api.Client, error) {
	timeout, err := config.HTTPTimeout()
	if err != nil {
		return nil, err
	}
	return api.New(baseURL, token,
		api.WithTimeout(timeout),
		api.WithLogger(newLogger(w, verbose)),
	)
}

// newLogger создаёт функцию лога для API-клиента (§4.6). По умолчанию уровень
// error (клиент ничего не логирует); --verbose или YT_LOG_LEVEL=debug включают
// debug-лог в stderr формата «2026-07-31T12:00:00Z DBG GET /issues?$top=30
// status=200 dur=123ms». Тело ответа и токен не логируются.
func newLogger(w io.Writer, verbose bool) func(string, ...any) {
	if !verbose && config.LogLevel() != "debug" {
		return func(string, ...any) {}
	}
	return func(format string, args ...any) {
		fmt.Fprintf(w, "%s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
	}
}

// Run выполняет CLI с os.Args и возвращает код выхода процесса.
func Run() int {
	return RunArgs(os.Args[1:], os.Stdout, os.Stderr)
}

// RunArgs выполняет CLI с заданными аргументами и потоками, возвращая код
// выхода процесса (SPEC §4.4): 0 — успех, 1 — runtime/API, 2 — usage,
// 130 — отменено пользователем (SIGINT/SIGTERM, §4.5).
func RunArgs(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := NewRootCommand()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	root.SetContext(ctx)
	return run(root)
}

// run выполняет команду и преобразует ошибку в код выхода. Сообщение ошибки
// печатается в stderr один раз в формате «yt: <сообщение>» (§4.4).
func run(root *cobra.Command) int {
	err := root.Execute()
	if err == nil {
		return exitOK
	}
	fmt.Fprintf(root.ErrOrStderr(), "yt: %s\n", formatError(err))
	return exitCodeFor(err)
}

// configFromContext возвращает resolved-конфигурацию из контекста (устанавливает
// pipeline) или nil, если pipeline не выполнялся.
func configFromContext(cmd *cobra.Command) *config.Config {
	cfg, _ := cmd.Context().Value(configKey{}).(*config.Config)
	return cfg
}

// requireClient возвращает API-клиент из контекста, проверяя наличие токена
// (§4.4: «no token provided: run "yt auth login" or set YT_TOKEN»). Команды, не
// требующие API (version, completion, auth login), его не вызывают.
func requireClient(cmd *cobra.Command) (*api.Client, error) {
	client, _ := cmd.Context().Value(clientKey{}).(*api.Client)
	if client == nil {
		return nil, errors.New("API client is not initialized")
	}
	cfg := configFromContext(cmd)
	if cfg == nil || cfg.Token == "" {
		return nil, fmt.Errorf(`no token provided: run "yt auth login" or set YT_TOKEN`)
	}
	return client, nil
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
