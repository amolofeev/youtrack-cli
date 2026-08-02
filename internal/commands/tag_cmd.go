package commands

import (
	"fmt"

	"github.com/amolofeev/yt/internal/api"
	"github.com/amolofeev/yt/internal/output"
	"github.com/spf13/cobra"
)

// tagListDefaultLimit — дефолт --limit для yt tag list (SPEC §3.9).
const tagListDefaultLimit = 50

// newTagCmd создаёт группу команд yt tag (SPEC §3.9).
func newTagCmd() *cobra.Command {
	tagCmd := &cobra.Command{
		Use:   "tag",
		Short: "Теги",
		// Runnable: «yt tag» — help (exit 0); Args — «yt tag foo» — exit 2 (§4.4).
		Args: argsValidator(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	tagCmd.AddCommand(newTagListCmd())
	return tagCmd
}

// newTagListCmd создаёт yt tag list (SPEC §3.9): список тегов (GET /tags)
// с флагами -q/--query и -l/--limit.
func newTagListCmd() *cobra.Command {
	var (
		query string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Список тегов",
		Long: "Список тегов (GET /tags).\n" +
			"-q задаёт фильтр по имени тега. Колонки: NAME, UNTAG ON RESOLVE.\n",
		Args: argsValidator(cobra.NoArgs),
		RunE: runTagListCmd(&query, &limit),
	}
	flags := cmd.Flags()
	flags.StringVarP(&query, "query", "q", "", "фильтр по имени тега")
	flags.IntVarP(&limit, "limit", "l", tagListDefaultLimit, "максимум тегов (1..)")
	return cmd
}

// runTagListCmd возвращает RunE для yt tag list (SPEC §3.9):
// GET /tags с полями FieldsTagList, query и $top.
func runTagListCmd(query *string, limit *int) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if *limit < 1 {
			return usageError(fmt.Errorf("--limit must be at least 1"))
		}

		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		tags, err := client.ListTags(cmd.Context(), *query, api.FieldsTagList, *limit, 0)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.JSON() {
			return writeTagJSON(p, tags)
		}
		return writeTagTTY(p, tags)
	}
}

// writeTagJSON сериализует список тегов в JSON (SPEC §3.9).
func writeTagJSON(p *output.Printer, tags []api.Tag) error {
	if tags == nil {
		tags = []api.Tag{}
	}
	return p.WriteJSON(tags)
}

// writeTagTTY печатает таблицу тегов (SPEC §3.9) или сообщение при пустом
// результате.
func writeTagTTY(p *output.Printer, tags []api.Tag) error {
	if len(tags) == 0 {
		return p.Linef("No tags found")
	}

	rows := make([][]string, 0, len(tags))
	for _, t := range tags {
		rows = append(rows, tagRow(t))
	}
	headers := []string{"NAME", "UNTAG ON RESOLVE"}
	return p.Table(headers, rows)
}

// tagRow собирает строку таблицы tag list (SPEC §3.9): NAME и
// UNTAG ON RESOLVE (false/true).
func tagRow(t api.Tag) []string {
	return []string{
		t.Name,
		fmt.Sprintf("%t", boolValue(t.UntagOnResolve)),
	}
}
