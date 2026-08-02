package commands

import (
	"fmt"

	"github.com/amolofeev/youtrack-cli/internal/api"
	"github.com/amolofeev/youtrack-cli/internal/output"
	"github.com/spf13/cobra"
)

// projectListDefaultLimit — дефолт --limit для yt project list (SPEC §3.7).
const projectListDefaultLimit = 50

// newProjectCmd создаёт группу команд yt project (SPEC §3.7).
func newProjectCmd() *cobra.Command {
	projectCmd := &cobra.Command{
		Use:   "project",
		Short: "Проекты",
		// Runnable: «yt project» — help (exit 0); Args — «yt project foo» — exit 2 (§4.4).
		Args: argsValidator(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	projectCmd.AddCommand(newProjectListCmd())
	return projectCmd
}

// newProjectListCmd создаёт yt project list (SPEC §3.7): список проектов
// (GET /admin/projects) с флагами -l/--limit и --skip.
func newProjectListCmd() *cobra.Command {
	var (
		limit int
		skip  int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Список проектов",
		Long: "Список проектов (GET /admin/projects).\n" +
			"Колонки: SHORTNAME, NAME, ARCHIVED, LEADER.\n",
		Args: argsValidator(cobra.NoArgs),
		RunE: runProjectListCmd(&limit, &skip),
	}
	flags := cmd.Flags()
	flags.IntVarP(&limit, "limit", "l", projectListDefaultLimit, "максимум проектов (1..)")
	flags.IntVar(&skip, "skip", 0, "пропустить первых N проектов")
	return cmd
}

// runProjectListCmd возвращает RunE для yt project list (SPEC §3.7):
// GET /admin/projects с полями FieldsProjectList и $top/$skip.
func runProjectListCmd(limit, skip *int) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if *limit < 1 {
			return usageError(fmt.Errorf("--limit must be at least 1"))
		}
		if *skip < 0 {
			return usageError(fmt.Errorf("--skip must be non-negative"))
		}

		client, err := requireClient(cmd)
		if err != nil {
			return err
		}

		projects, err := client.ListProjects(cmd.Context(), api.FieldsProjectList, *limit, *skip)
		if err != nil {
			return err
		}

		p := printer(cmd)
		if p.JSON() {
			return writeProjectJSON(p, projects)
		}
		return writeProjectTTY(p, projects)
	}
}

// writeProjectJSON сериализует список проектов в JSON (SPEC §3.7).
func writeProjectJSON(p *output.Printer, projects []api.Project) error {
	if projects == nil {
		projects = []api.Project{}
	}
	return p.WriteJSON(projects)
}

// writeProjectTTY печатает таблицу проектов (SPEC §3.7) или сообщение при
// пустом результате.
func writeProjectTTY(p *output.Printer, projects []api.Project) error {
	if len(projects) == 0 {
		return p.Linef("No projects found")
	}

	rows := make([][]string, 0, len(projects))
	for _, pr := range projects {
		rows = append(rows, projectRow(pr))
	}
	headers := []string{"SHORTNAME", "NAME", "ARCHIVED", "LEADER"}
	return p.Table(headers, rows)
}

// projectRow собирает строку таблицы project list (SPEC §3.7): SHORTNAME,
// NAME, ARCHIVED (false/true), LEADER — login лидера.
func projectRow(pr api.Project) []string {
	leader := ""
	if pr.Leader != nil {
		leader = pr.Leader.Login
	}
	return []string{
		pr.ShortName,
		pr.Name,
		fmt.Sprintf("%t", boolValue(pr.Archived)),
		leader,
	}
}
