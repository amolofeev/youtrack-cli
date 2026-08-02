package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/amolofeev/yt/internal/api"
	"github.com/amolofeev/yt/internal/config"
	"github.com/amolofeev/yt/internal/version"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// authStatus — выводимое поле baseUrl добавляется утилитой (исключение §4.3,
// его нет в ответе /users/me). Порядок полей определяет порядок ключей в --json.
type authStatus struct {
	BaseURL  string `json:"baseUrl"`
	Login    string `json:"login"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Guest    bool   `json:"guest"`
}

// newAuthCmd создаёт группу команд yt auth (SPEC §3.3).
func newAuthCmd(opts *globalOptions) *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Управление аутентификацией",
		// Runnable, чтобы «yt auth» печатал help с exit 0 (как root); Args —
		// чтобы «yt auth foo» давал exit 2 (unknown command, §4.4).
		Args: argsValidator(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	authCmd.AddCommand(newLoginCmd(opts), newLogoutCmd(), newStatusCmd())
	return authCmd
}

// newLoginCmd создаёт yt auth login (SPEC §3.3): запрос Base URL и токена,
// проверка токена GET /users/me и сохранение конфигурации.
func newLoginCmd(opts *globalOptions) *cobra.Command {
	var withToken string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Сохранить адрес сервера и токен",
		Long: "Интерактивно запрашивает Base URL и токен, проверяет токен запросом\n" +
			"GET /users/me и сохраняет конфигурацию.\n" +
			"В неинтерактивном режиме (нет TTY) токен читается из stdin — он не\n" +
			"попадает в shell history и не виден в ps.\n" +
			"ВНИМАНИЕ: токен через --with-token НЕ скрыт — он виден в shell history\n" +
			"и в списке процессов. Используйте интерактивный ввод или stdin.",
		Args: argsValidator(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogin(cmd, opts, withToken)
		},
	}
	cmd.Flags().StringVar(&withToken, "with-token", "", "токен из аргумента (виден в ps/history)")
	return cmd
}

func runLogin(cmd *cobra.Command, opts *globalOptions, withToken string) error {
	cfg := configFromContext(cmd)
	if cfg == nil {
		return errors.New("config is not initialized")
	}
	baseURL := cfg.BaseURL
	token := withToken
	if token == "" {
		switch {
		case terminalStdin(cmd):
			// Интерактивный режим (§3.3): Base URL с дефолтом, токен без эха.
			baseURL = promptBaseURL(cmd, baseURL)
			b, err := promptSecret(cmd)
			if err != nil {
				return err
			}
			token = string(b)
		default:
			// Неинтерактивный режим: токен из stdin (не печатается, не в history).
			line, err := readLineStdin(cmd)
			if err != nil {
				return err
			}
			token = strings.TrimSpace(line)
		}
	}
	if token == "" {
		return errors.New("token is required")
	}
	client, err := newAPIClient(baseURL, token, cmd.ErrOrStderr(), opts.verbose)
	if err != nil {
		return err
	}
	u, err := client.Me(cmd.Context(), api.FieldsAuthLogin)
	if err != nil {
		return err
	}
	cfg.Token = token
	cfg.BaseURL = baseURL
	if err := config.Save(cfg); err != nil {
		return err
	}
	return printer(cmd).Successf("Authenticated as %s (%s)", u.Login, u.FullName)
}

// newLogoutCmd создаёт yt auth logout (SPEC §3.3): удаляет токен из файла
// конфигурации, base URL сохраняется. Работает с файлом конфигурации
// напрямую, а не с resolved-конфигурацией: токен из YT_TOKEN/env не считается
// «залогиненным».
func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Удалить сохранённый токен",
		Args:  argsValidator(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if cfg.Token == "" {
				return errors.New("not logged in")
			}
			cfg.Token = ""
			if err := config.Save(cfg); err != nil {
				return err
			}
			return printer(cmd).Successf("Logged out")
		},
	}
}

// newStatusCmd создаёт yt auth status (SPEC §3.3): живая проверка токена
// GET /users/me; при 401 — «✗ not logged in», exit 1.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Проверить статус аутентификации",
		Args:  argsValidator(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := requireClient(cmd)
			if err != nil {
				return err
			}
			cfg := configFromContext(cmd)
			if cfg == nil {
				return errors.New("config is not initialized")
			}
			u, err := client.Me(cmd.Context(), api.FieldsAuthStatus)
			if err != nil {
				var ae *api.Error
				if errors.As(err, &ae) && ae.Type == api.ErrorAuth {
					return errors.New("✗ not logged in")
				}
				return err
			}
			p := printer(cmd)
			if p.JSON() {
				return p.WriteJSON(authStatus{
					BaseURL:  cfg.BaseURL,
					Login:    u.Login,
					FullName: u.FullName,
					Email:    u.Email,
					Guest:    boolValue(u.Guest),
				})
			}
			if err := p.Linef("yt (%s)", version.Version); err != nil {
				return err
			}
			for _, row := range []struct{ label, value string }{
				{"Server:", cfg.BaseURL},
				{"Login:", u.Login},
				{"Name:", u.FullName},
				{"Email:", u.Email},
				{"Guest:", fmt.Sprintf("%t", boolValue(u.Guest))},
			} {
				if err := p.Linef("%-9s %s", row.label, row.value); err != nil {
					return err
				}
			}
			return p.Successf("Authenticated")
		},
	}
}

// newUserCmd создаёт группу команд yt user (SPEC §3.8).
func newUserCmd() *cobra.Command {
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "Пользователь",
		// Runnable: «yt user» — help (exit 0); Args — «yt user foo» — exit 2 (§4.4).
		Args: argsValidator(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	userCmd.AddCommand(newWhoamiCmd())
	return userCmd
}

// newWhoamiCmd создаёт yt user whoami (SPEC §3.8): профиль текущего
// пользователя; --json — сырой объект /users/me.
func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Показать текущего пользователя",
		Args:  argsValidator(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := requireClient(cmd)
			if err != nil {
				return err
			}
			u, err := client.Me(cmd.Context(), api.FieldsUserWhoami)
			if err != nil {
				return err
			}
			p := printer(cmd)
			if p.JSON() {
				return p.WriteJSON(u)
			}
			for _, row := range []struct{ label, value string }{
				{"Login:", u.Login},
				{"Name:", u.FullName},
				{"Email:", u.Email},
				{"Guest:", fmt.Sprintf("%t", boolValue(u.Guest))},
				{"ID:", u.ID},
			} {
				if err := p.Linef("%-9s %s", row.label, row.value); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func boolValue(b *bool) bool { return b != nil && *b }

// terminalStdin сообщает, является ли stdin терминалом (интерактивный режим
// yt auth login, §3.3).
func terminalStdin(cmd *cobra.Command) bool {
	f, ok := cmd.InOrStdin().(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// promptBaseURL печатает приглашение «? Base URL [<дефолт>]: » в stderr и
// читает строку; пустой ввод — дефолт из конфигурации.
func promptBaseURL(cmd *cobra.Command, def string) string {
	fmt.Fprintf(cmd.ErrOrStderr(), "? Base URL [%s]: ", def)
	line, err := readLineStdin(cmd)
	if err != nil || strings.TrimSpace(line) == "" {
		return def
	}
	return strings.TrimSpace(line)
}

// promptSecret печатает приглашение «? Token: » в stderr и читает токен без
// эха (term.ReadPassword). Приглашения выводятся в stderr — stdout только для
// данных (§4.3).
func promptSecret(cmd *cobra.Command) ([]byte, error) {
	fmt.Fprint(cmd.ErrOrStderr(), "? Token: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return nil, fmt.Errorf("read token: %w", err)
	}
	return b, nil
}

// readLineStdin читает одну строку из stdin, отрезая перевод строки.
func readLineStdin(cmd *cobra.Command) (string, error) {
	br := bufio.NewReader(cmd.InOrStdin())
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
