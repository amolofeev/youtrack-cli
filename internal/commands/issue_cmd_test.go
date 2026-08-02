package commands

import (
	"encoding/json"
	"testing"

	"github.com/amolofeev/youtrack-cli/internal/api"
)

func TestNewIssueCmd(t *testing.T) {
	cmd := newIssueCmd()
	if cmd.Use != "issue" {
		t.Errorf("Use = %q, want issue", cmd.Use)
	}

	list := cmd.Commands()
	if len(list) == 0 {
		t.Fatal("expected at least one subcommand")
	}
	var found bool
	for _, c := range list {
		if c.Name() == "list" {
			found = true
		}
	}
	if !found {
		t.Error("expected \"list\" subcommand")
	}
}

func TestNewIssueListCmd_Flags(t *testing.T) {
	cmd := newIssueListCmd()

	flags := []string{"query", "state", "project"}
	expectedShort := map[string]string{
		"query":   "q",
		"state":   "s",
		"project": "P",
	}
	for _, f := range flags {
		fs := cmd.Flags().Lookup(f)
		if fs == nil {
			t.Errorf("flag %q not found", f)
			continue
		}
		if exp := expectedShort[f]; exp != "" && fs.Shorthand != exp {
			t.Errorf("flag %q shorthand = %q, want %q", f, fs.Shorthand, exp)
		}
	}

	limit := cmd.Flags().Lookup("limit")
	if limit == nil {
		t.Fatal("flag limit not found")
	}
	if limit.DefValue != "30" {
		t.Errorf("limit default = %q, want 30", limit.DefValue)
	}
}

func TestBuildIssueQuery_WithAll(t *testing.T) {
	q := buildIssueQuery("fix login", "", "open", "PRJ", "alice", nil)
	want := "fix login state: #Unresolved project: PRJ assignee: alice"
	if q != want {
		t.Errorf(" = %q, want %q", q, want)
	}
}

func TestBuildIssueQuery_PosOnly(t *testing.T) {
	q := buildIssueQuery("fix login", "", "", "", "", nil)
	if q != "fix login" {
		t.Errorf(" = %q, want fix login", q)
	}
}

func TestBuildIssueQuery_QueryFlagOverwritesPos(t *testing.T) {
	q := buildIssueQuery("positional", "actual query", "", "", "", nil)
	if q != "actual query" {
		t.Errorf(" = %q, want actual query", q)
	}
}

func TestBuildIssueQuery_WithTags(t *testing.T) {
	q := buildIssueQuery("", "fix", "", "", "", []string{"backend", "security"})
	want := "fix tag: backend tag: security"
	if q != want {
		t.Errorf(" = %q, want %q", q, want)
	}
}

func TestBuildIssueQuery_StateAll(t *testing.T) {
	q := buildIssueQuery("", "", "all", "", "", nil)
	if q != "" {
		t.Errorf(" = %q, want empty for state=all", q)
	}
}

func TestTranslateState(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"open", "#Unresolved"},
		{"resolved", "#Resolved"},
		{"all", ""},
		{"In Progress", "In Progress"},
	}
	for _, tt := range tests {
		if got := translateState(tt.in); got != tt.want {
			t.Errorf("translateState(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{"2026-07-01", 1782864000000, "2026-07-01"},
		{"epoch", 0, ""},
	}
	for _, tt := range tests {
		if got := formatDate(tt.ms); got != tt.want {
			t.Errorf("formatDate(%d) = %q, want %q", tt.ms, got, tt.want)
		}
	}
}

func TestIssueRow(t *testing.T) {
	rawState := json.RawMessage(`{"$type":"EnumBundleElement","id":"5-1","name":"Done"}`)
	i := api.Issue{
		ID:         "2-1",
		IDReadable: "PRJ-1",
		Summary:    "Fix login",
		Created:    1782864000000,
		Updated:    1783296000000,
		Reporter: &api.User{
			Login:    "alice",
			FullName: "Alice",
		},
		CustomFields: []api.IssueCustomField{{
			Type:  "EnumIssueCustomField",
			ID:    "4-1",
			Name:  "State",
			Value: rawState,
		}},
	}

	row := issueRow(i)
	if len(row) != 6 {
		t.Fatalf("row cols = %d, want 6", len(row))
	}
	checks := []struct {
		idx   int
		valid func(string) bool
	}{
		{0, func(s string) bool { return s == "PRJ-1" }},      // ID (idReadable)
		{1, func(s string) bool { return s == "Done" }},       // STATE
		{2, func(s string) bool { return s == "2026-07-01" }}, // CREATED
		{3, func(s string) bool { return s == "2026-07-06" }}, // UPDATED
		{4, func(s string) bool { return s == "alice" }},      // REPORTER
		{5, func(s string) bool { return s == "Fix login" }},  // SUMMARY
	}
	for _, c := range checks {
		if !c.valid(row[c.idx]) {
			t.Errorf("issueRow[%d] = %q, expected non-empty", c.idx, row[c.idx])
		}
	}
}

func TestIssueID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		read string
		want string
	}{
		{"uses readable", "2-1", "PRJ-1", "PRJ-1"},
		{"falls back to id", "2-1", "", "2-1"},
	}
	for _, tt := range tests {
		i := api.Issue{ID: tt.id, IDReadable: tt.read}
		if got := issueID(i); got != tt.want {
			t.Errorf("issueID(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestIssueReporter_Nil(t *testing.T) {
	i := api.Issue{}
	if got := issueReporter(i); got != "" {
		t.Errorf("issueReporter(nil) = %q, want empty", got)
	}
}

func TestFormatDate_EpochZero(t *testing.T) {
	if got := formatDate(0); got != "" {
		t.Errorf("formatDate(0) = %q, want empty", got)
	}
}

func TestBuildIssueQuery_EmptyAll(t *testing.T) {
	q := buildIssueQuery("", "", "", "", "", nil)
	if q != "" {
		t.Errorf("buildIssueQuery(empty) = %q, want empty", q)
	}
}
