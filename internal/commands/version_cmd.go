package commands

import (
	"runtime"

	"github.com/amolofeev/yt/internal/version"
	"github.com/spf13/cobra"
)

// versionInfo — сведения об утилите для yt version (SPEC §3.11). Порядок полей
// определяет порядок ключей в --json.
type versionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Built   string `json:"built"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// currentVersionInfo собирает сведения о сборке (commit/built подставляются
// через -ldflags; без них — "unknown").
func currentVersionInfo() versionInfo {
	return versionInfo{
		Version: version.Version,
		Commit:  version.Commit,
		Built:   version.Built,
		Go:      runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
}

// newVersionCmd создаёт команду yt version (SPEC §3.11). Работает без токена и
// подключения к серверу; --json — машинный вывод.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Вывести версию утилиты",
		Long: "Вывести версию утилиты (version, commit, дату сборки, платформу).\n" +
			"Не требует токена и подключения к серверу.",
		Args: argsValidator(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := printer(cmd)
			info := currentVersionInfo()
			if p.JSON() {
				return p.WriteJSON(info)
			}
			if err := p.Linef("yt version %s", info.Version); err != nil {
				return err
			}
			for _, row := range []struct{ label, value string }{
				{"commit:", info.Commit},
				{"built:", info.Built},
				{"go:", info.Go},
				{"os:", info.OS},
				{"arch:", info.Arch},
			} {
				if err := p.Linef("%-7s %s", row.label, row.value); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
