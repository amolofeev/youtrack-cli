package commands

import (
	"fmt"

	"github.com/amolofeev/yt/internal/api"
	"github.com/amolofeev/yt/internal/output"
	"github.com/spf13/cobra"
)

// newSearchCmd создаёт yt search (SPEC §3.5): поиск задач по произвольному
// запросу. Аргумент — сырой поисковый запрос YouTrack (без «умных» флагов
// issue list), рендер — как у issue list. Подкоманда suggest — автодополнение
// поискового запроса.
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
	cmd.AddCommand(newSearchSuggestCmd())
	return cmd
}

// newSearchSuggestCmd создаёт yt search suggest (SPEC §3.5): автодополнение
// поискового запроса (POST /search/assist). Аргумент — частичный поисковый
// запрос; в v1 — только вывод подсказок, без интерактивного UI.
func newSearchSuggestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest <query>",
		Short: "Автодополнение поискового запроса",
		Long: "Автодополнение поискового запроса YouTrack (POST /search/assist).\n" +
			"Аргумент — частичный поисковый запрос; выводятся подсказки «option — description»\n" +
			"сгруппированные по group. В v1 — только вывод, без интерактивного UI.\n",
		Args: argsValidator(cobra.ExactArgs(1)),
		RunE: runSearchSuggestCmd(),
	}
	return cmd
}

// runSearchSuggestCmd возвращает RunE для yt search suggest (SPEC §3.5):
// POST /search/assist с полями FieldsSearchSuggest и сырым частичным запросом.
func runSearchSuggestCmd() func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		query := args[0]
		sugg, err := client.SearchSuggest(cmd.Context(), query, api.FieldsSearchSuggest)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.JSON() {
			return p.WriteJSON(sugg)
		}
		return writeSearchSuggestTTY(p, sugg, query)
	}
}

// writeSearchSuggestTTY печатает подсказки автодополнения (SPEC §3.5):
// подсказки сгруппированы по group (заголовок «<group>:»), каждая —
// «option — description». Пустой ответ — «No suggestions for "<query>"».
func writeSearchSuggestTTY(p *output.Printer, sugg *api.SearchSuggestions, query string) error {
	if len(sugg.Suggestions) == 0 {
		return p.Linef("No suggestions for %s", quoteQuery(query))
	}
	var groupOrder []string
	byGroup := make(map[string][]api.Suggestion)
	for _, s := range sugg.Suggestions {
		g := s.Group
		if g == "" {
			g = "Suggestions"
		}
		if _, ok := byGroup[g]; !ok {
			groupOrder = append(groupOrder, g)
		}
		byGroup[g] = append(byGroup[g], s)
	}
	for _, g := range groupOrder {
		if err := p.Linef("%s:", g); err != nil {
			return err
		}
		for _, s := range byGroup[g] {
			if err := p.Linef("%s — %s", s.Option, s.Description); err != nil {
				return err
			}
		}
	}
	return nil
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
