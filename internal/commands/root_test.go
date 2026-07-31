package commands

import (
	"strings"
	"testing"
)

func TestGroupsRegistered(t *testing.T) {
	root := NewRootCommand()
	got := make(map[string]string)
	for _, g := range root.Groups() {
		got[g.ID] = g.Title
	}
	want := map[string]string{
		"core":    "Основное",
		"issues":  "Issues",
		"server":  "Сервер",
		"utility": "Служебное",
	}
	for id, title := range want {
		if got[id] != title {
			t.Errorf("group %q title = %q, want %q", id, got[id], title)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d groups, want %d: %v", len(got), len(want), got)
	}
}

func TestGlobalFlagsOnRoot(t *testing.T) {
	root := NewRootCommand()
	for _, name := range []string{"base-url", "token", "json", "verbose"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("root persistent flag --%s is missing", name)
		}
	}
}

func TestGlobalFlagsOnSubcommand(t *testing.T) {
	root := NewRootCommand()
	sub, _, err := root.Find([]string{"version"})
	if err != nil {
		t.Fatalf("find version: %v", err)
	}
	for _, name := range []string{"base-url", "token", "json", "verbose"} {
		if sub.InheritedFlags().Lookup(name) == nil {
			t.Errorf("subcommand %q: global flag --%s is missing", sub.Name(), name)
		}
	}
}

func TestHelpShowsGroups(t *testing.T) {
	stdout, stderr, err := runCmd("--help")
	if err != nil {
		t.Fatalf("--help: unexpected error: %v", err)
	}
	if stderr != "" {
		t.Errorf("--help: stderr = %q, want empty", stderr)
	}
	for _, want := range []string{"Основное", "Issues", "Сервер", "Служебное"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help output should contain group %q:\n%s", want, stdout)
		}
	}
	for _, want := range []string{"version", "completion"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help output should contain command %q:\n%s", want, stdout)
		}
	}
}

func TestSubcommandHelpShowsGlobalFlags(t *testing.T) {
	stdout, _, err := runCmd("version", "--help")
	if err != nil {
		t.Fatalf("version --help: unexpected error: %v", err)
	}
	for _, want := range []string{"--base-url", "--token", "--json", "--verbose"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("version help should list global flag %q:\n%s", want, stdout)
		}
	}
}

func TestCompletionInUtilityGroup(t *testing.T) {
	root := NewRootCommand()
	root.InitDefaultCompletionCmd()
	comp, _, err := root.Find([]string{"completion"})
	if err != nil {
		t.Fatalf("find completion: %v", err)
	}
	if comp.GroupID != "utility" {
		t.Errorf("completion.GroupID = %q, want %q", comp.GroupID, "utility")
	}
}

func TestCompletionScripts(t *testing.T) {
	cases := map[string]string{
		"bash": "# bash completion V2 for yt",
		"zsh":  "#compdef yt",
		"fish": "# fish completion for yt",
	}
	for shell, marker := range cases {
		stdout, stderr, err := runCmd("completion", shell)
		if err != nil {
			t.Fatalf("completion %s: unexpected error: %v", shell, err)
		}
		if stderr != "" {
			t.Errorf("completion %s: stderr = %q, want empty", shell, stderr)
		}
		if !strings.Contains(stdout, marker) {
			t.Errorf("completion %s: expected marker %q, got:\n%s", shell, marker, stdout)
		}
	}
}
