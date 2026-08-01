package commands

import (
	"fmt"

	"github.com/amolofeev/prompt-and-pray/internal/api"
	"github.com/spf13/cobra"
)

// newSearchCmd создаёт yt search (SPEC §3.5): поиск задач по произвольному
// запросу. Аргумент — сырой поисковый запрос YouTrack (без «умных» флагов
// issue list), рендер — как у issue list.
func newSearchCmd() *cobra.Command {
	var (
		limit int
		skip  int
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Поиск задач",
		Long: "Поиск задач по произвольному запросу YouTrack (GET /issues).\n" +
			"Аргумент — сырой поисковый запрос (как у issue list без «умных» флагов).\n",
		Args: argsValidator(cobra.ExactArgs(1)),
		RunE: runSearchCmd(&limit, &skip),
	}
	flags := cmd.Flags()
	flags.IntVarP(&limit, "limit", "l", issueListDefaultLimit, "максимум задач (1..100)")
	flags.IntVar(&skip, "skip", 0, "пропустить первых N задач")
	return cmd
}

// runSearchCmd возвращает RunE для yt search (SPEC §3.5): GET /issues с сырым
// query, полями FieldsIssueList и $top/$skip; вывод — как у issue list.
func runSearchCmd(limit, skip *int) func(*cobra.Command, []string) error {
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

		query := args[0]
		issues, err := client.Search(cmd.Context(), query, api.FieldsIssueList, *limit, *skip)
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
