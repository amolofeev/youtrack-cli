package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pagerStub создаёт исполняемый скрипт, который пишет stdin в файл, — заглушка
// pager-а для тестов. Возвращает значение $PAGER (путь к скрипту с выходным
// файлом) и путь выходного файла.
func pagerStub(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "paged.out")
	script := filepath.Join(dir, "pager.sh")
	content := "#!/bin/sh\ncat > \"$1\"\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write pager stub: %v", err)
	}
	return script + " " + out, out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestPagerCommand(t *testing.T) {
	cases := []struct {
		name  string
		pager string
		want  string
	}{
		{"unset", "", "less -FRX"},
		{"custom", "more", "more"},
		{"custom with args", "less -F -R", "less -F -R"},
		{"disabled cat", "cat", ""},
		{"disabled cat trimmed", "  cat  ", ""},
		{"cat with args not disabled", "cat -n", "cat -n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PAGER", tc.pager)
			if got := pagerCommand(); got != tc.want {
				t.Errorf("pagerCommand() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPage_LaunchesPager(t *testing.T) {
	pager, outFile := pagerStub(t)
	t.Setenv("PAGER", pager)
	p, out, _ := newTestPrinter(ModeTTY, WithTerminal(true))
	if !p.Page() {
		t.Fatal("Page() = false, want true (PAGER stub, terminal forced)")
	}
	if err := p.Linef("line one"); err != nil {
		t.Fatalf("Linef() error: %v", err)
	}
	if err := p.Linef("line two"); err != nil {
		t.Fatalf("Linef() error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout not empty while paging: %q", out.String())
	}
	if err := p.EndPage(); err != nil {
		t.Fatalf("EndPage() error: %v", err)
	}
	if got, want := readFile(t, outFile), "line one\nline two\n"; got != want {
		t.Errorf("pager received %q, want %q", got, want)
	}
	// после EndPage вывод снова идёт в stdout.
	if err := p.Linef("after"); err != nil {
		t.Fatalf("Linef() after EndPage error: %v", err)
	}
	if !strings.Contains(out.String(), "after") {
		t.Errorf("stdout after EndPage missing data: %q", out.String())
	}
}

func TestPage_DisabledByPAGERCat(t *testing.T) {
	t.Setenv("PAGER", "cat")
	p, out, _ := newTestPrinter(ModeTTY, WithTerminal(true))
	if p.Page() {
		t.Fatal("Page() = true with PAGER=cat, want false")
	}
	if err := p.Linef("straight"); err != nil {
		t.Fatalf("Linef() error: %v", err)
	}
	if got, want := out.String(), "straight\n"; got != want {
		t.Errorf("stdout = %q, want %q (output must go to stdout when paging disabled)", got, want)
	}
}

func TestPage_DisabledNonTerminal(t *testing.T) {
	t.Setenv("PAGER", "less")
	p, out, _ := newTestPrinter(ModeTTY)
	if p.Page() {
		t.Fatal("Page() = true with non-terminal stdout, want false")
	}
	if err := p.Linef("x"); err != nil {
		t.Fatalf("Linef() error: %v", err)
	}
	if got, want := out.String(), "x\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestPage_DisabledVerbose(t *testing.T) {
	t.Setenv("PAGER", "less")
	p, _, _ := newTestPrinter(ModeTTY, WithTerminal(true), WithVerbose(true))
	if p.Page() {
		t.Fatal("Page() = true with --verbose, want false")
	}
}

func TestPage_DisabledJSONMode(t *testing.T) {
	t.Setenv("PAGER", "less")
	p, _, _ := newTestPrinter(ModeJSON, WithTerminal(true))
	if p.Page() {
		t.Fatal("Page() = true in JSON mode, want false")
	}
}

func TestPage_UnavailablePagerCommand(t *testing.T) {
	t.Setenv("PAGER", "definitely-not-a-pager-binary-xyz")
	p, out, _ := newTestPrinter(ModeTTY, WithTerminal(true))
	if p.Page() {
		t.Fatal("Page() = true for non-existent pager, want false")
	}
	if err := p.Linef("fallback"); err != nil {
		t.Fatalf("Linef() error: %v", err)
	}
	if got, want := out.String(), "fallback\n"; got != want {
		t.Errorf("stdout = %q, want %q (fallback to direct output)", got, want)
	}
}

func TestEndPage_WithoutPage(t *testing.T) {
	p, _, _ := newTestPrinter(ModeTTY)
	if err := p.EndPage(); err != nil {
		t.Errorf("EndPage() without Page() error: %v", err)
	}
}
