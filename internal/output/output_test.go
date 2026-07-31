package output

import (
	"bytes"
	"strings"
	"testing"
)

func newTestPrinter(mode Mode, opts ...Option) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	var out, errw bytes.Buffer
	p := New(&out, &errw, mode, opts...)
	return p, &out, &errw
}

func TestNewDefaults(t *testing.T) {
	p, _, _ := newTestPrinter(ModeTTY)
	if p.Mode() != ModeTTY {
		t.Errorf("Mode() = %v, want ModeTTY", p.Mode())
	}
	if !p.TTY() {
		t.Error("TTY() = false, want true")
	}
	if p.JSON() {
		t.Error("JSON() = true, want false")
	}
	if p.Color() {
		t.Error("Color() = true, want false (buffer is not a TTY)")
	}
	if p.Width() != 0 {
		t.Errorf("Width() = %d, want 0 (buffer has no size)", p.Width())
	}
}

func TestNewJSONMode(t *testing.T) {
	p, _, _ := newTestPrinter(ModeJSON)
	if p.Mode() != ModeJSON {
		t.Errorf("Mode() = %v, want ModeJSON", p.Mode())
	}
	if !p.JSON() {
		t.Error("JSON() = false, want true")
	}
	if p.TTY() {
		t.Error("TTY() = true, want false")
	}
}

func TestOptionsOverride(t *testing.T) {
	p, _, _ := newTestPrinter(ModeTTY, WithColor(true), WithWidth(120))
	if !p.Color() {
		t.Error("WithColor(true): Color() = false")
	}
	if p.Width() != 120 {
		t.Errorf("WithWidth(120): Width() = %d", p.Width())
	}
}

func TestLinef(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY)
	if err := p.Linef("issue %s", "PRJ-1"); err != nil {
		t.Fatalf("Linef() error: %v", err)
	}
	if got, want := out.String(), "issue PRJ-1\n"; got != want {
		t.Errorf("Linef() = %q, want %q", got, want)
	}
}

func TestSuccessfPlain(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY)
	if err := p.Successf("Authenticated as %s", "alex"); err != nil {
		t.Fatalf("Successf() error: %v", err)
	}
	if got, want := out.String(), "✓ Authenticated as alex\n"; got != want {
		t.Errorf("Successf() = %q, want %q", got, want)
	}
}

func TestSuccessfColored(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY, WithColor(true))
	if err := p.Successf("ok"); err != nil {
		t.Fatalf("Successf() error: %v", err)
	}
	want := ANSIGreen + "✓ ok" + ANSIReset + "\n"
	if got := out.String(); got != want {
		t.Errorf("Successf() colored = %q, want %q", got, want)
	}
}

func TestWarnf(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY, WithColor(true))
	if err := p.Warnf("deprecated"); err != nil {
		t.Fatalf("Warnf() error: %v", err)
	}
	want := ANSIYellow + "deprecated" + ANSIReset + "\n"
	if got := out.String(); got != want {
		t.Errorf("Warnf() = %q, want %q", got, want)
	}
}

func TestErrorfGoesToStderr(t *testing.T) {
	p, out, errw := newTestPrinter(ModeTTY, WithColor(true))
	if err := p.Errorf("boom: %s", "bad"); err != nil {
		t.Fatalf("Errorf() error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout not empty: %q", out.String())
	}
	want := ANSIRed + "boom: bad" + ANSIReset + "\n"
	if got := errw.String(); got != want {
		t.Errorf("Errorf() = %q, want %q", got, want)
	}
}

func TestErrorfPlain(t *testing.T) {
	p, _, errw := newTestPrinter(ModeTTY)
	if err := p.Errorf("boom"); err != nil {
		t.Fatalf("Errorf() error: %v", err)
	}
	if got, want := errw.String(), "boom\n"; got != want {
		t.Errorf("Errorf() = %q, want %q", got, want)
	}
}

func TestJSON(t *testing.T) {
	p, out, _ := newTestPrinter(ModeJSON)
	v := []map[string]any{
		{"id": "2-1", "summary": "<tag> & \"quotes\""},
	}
	if err := p.WriteJSON(v); err != nil {
		t.Fatalf("JSON() error: %v", err)
	}
	want := `[{"id":"2-1","summary":"<tag> & \"quotes\""}]` + "\n"
	if got := out.String(); got != want {
		t.Errorf("JSON() = %q, want %q", got, want)
	}
}

func TestJSONRejectsTTYMode(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY)
	if err := p.WriteJSON(struct{}{}); err == nil {
		t.Error("JSON() in TTY mode: want error, got nil")
	}
	if out.Len() != 0 {
		t.Errorf("stdout not empty: %q", out.String())
	}
}

func TestRaw(t *testing.T) {
	p, out, _ := newTestPrinter(ModeJSON)
	if err := p.Raw([]byte(`{"a":1}`)); err != nil {
		t.Fatalf("Raw() error: %v", err)
	}
	if got, want := out.String(), `{"a":1}`+"\n"; got != want {
		t.Errorf("Raw() = %q, want %q", got, want)
	}

	out.Reset()
	if err := p.Raw([]byte("[]\n")); err != nil {
		t.Fatalf("Raw() with trailing newline error: %v", err)
	}
	if got, want := out.String(), "[]\n"; got != want {
		t.Errorf("Raw() = %q, want %q", got, want)
	}

	out.Reset()
	if err := p.Raw(nil); err != nil {
		t.Fatalf("Raw(nil) error: %v", err)
	}
	if got, want := out.String(), "\n"; got != want {
		t.Errorf("Raw(nil) = %q, want %q", got, want)
	}
}

func TestRawRejectsTTYMode(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY)
	if err := p.Raw([]byte(`{}`)); err == nil {
		t.Error("Raw() in TTY mode: want error, got nil")
	}
	if out.Len() != 0 {
		t.Errorf("stdout not empty: %q", out.String())
	}
}

func TestWrite(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY)
	if _, err := p.Write([]byte("data")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if got, want := out.String(), "data"; got != want {
		t.Errorf("Write() = %q, want %q", got, want)
	}
}

func TestTableRejectsJSONMode(t *testing.T) {
	p, out, _ := newTestPrinter(ModeJSON)
	err := p.Table([]string{"A"}, [][]string{{"1"}})
	if err == nil {
		t.Error("Table() in JSON mode: want error, got nil")
	}
	if out.Len() != 0 {
		t.Errorf("stdout not empty: %q", out.String())
	}
}

func TestTableEmptyRowsWritesNothing(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY)
	if err := p.Table([]string{"ID", "SUMMARY"}, nil); err != nil {
		t.Fatalf("Table() error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout not empty: %q", out.String())
	}
	if err := p.Table([]string{"ID", "SUMMARY"}, [][]string{}); err != nil {
		t.Fatalf("Table() error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout not empty: %q", out.String())
	}
}

func TestTableBasic(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY)
	rows := [][]string{
		{"PRJ-42", "Open", "Fix login flow"},
		{"PRJ-43", "Fixed", "Write TZ for yt CLI"},
	}
	if err := p.Table([]string{"ID", "STATE", "SUMMARY"}, rows); err != nil {
		t.Fatalf("Table() error: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"ID",
		"STATE",
		"SUMMARY",
		"PRJ-42",
		"PRJ-43",
		"Fixed",
		"Fix login flow",
		"Write TZ for yt CLI",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Table() output missing %q:\n%s", want, got)
		}
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("Table() produced %d lines, want 3:\n%s", len(lines), got)
	}
	for i, line := range lines {
		if len(line) == 0 {
			t.Errorf("line %d is empty", i)
		}
	}
}

func TestTableMultilineCell(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY, WithWidth(40))
	rows := [][]string{
		{"PRJ-42", "line one\nline two\nline three"},
	}
	if err := p.Table([]string{"ID", "BODY"}, rows); err != nil {
		t.Fatalf("Table() error: %v", err)
	}
	got := out.String()
	for _, want := range []string{"line one", "line two", "line three"} {
		if !strings.Contains(got, want) {
			t.Errorf("Table() missing wrapped line %q:\n%s", want, got)
		}
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 4 {
		t.Errorf("Table() produced %d lines, want 4 (header + 3 rows):\n%s", len(lines), got)
	}
}

func TestTableWrapsByTerminalWidth(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY, WithWidth(20))
	rows := [][]string{
		{"PRJ-42", "a very long summary that must be wrapped into multiple lines"},
	}
	if err := p.Table([]string{"ID", "SUMMARY"}, rows); err != nil {
		t.Fatalf("Table() error: %v", err)
	}
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		// перенос работает посимвольно с учётом ANSI-нет; проверяем, что
		// длинных строк больше нет после переноса.
		if len(line) > 24 {
			t.Errorf("line too long (%d > 24): %q", len(line), line)
		}
	}
}

func TestTableHardSplitsLongWord(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY, WithWidth(10))
	rows := [][]string{
		{"ID", "thisisasingleverylongwordoverthelimit"},
	}
	if err := p.Table([]string{"ID", "TEXT"}, rows); err != nil {
		t.Fatalf("Table() error: %v", err)
	}
	if !strings.Contains(out.String(), "thisisa") {
		t.Errorf("long word not hard-split:\n%s", out.String())
	}
	// слово разбито на несколько строк, а не оставлено целиком
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n")[1:] {
		if strings.Contains(line, "thisisasingleverylongwordoverthelimit") {
			t.Errorf("word not split:\n%s", out.String())
		}
	}
}

func TestTableUnicodeWidth(t *testing.T) {
	p, out, _ := newTestPrinter(ModeTTY, WithWidth(12))
	rows := [][]string{
		{"ID", "привет мир привет"},
	}
	if err := p.Table([]string{"ID", "TEXT"}, rows); err != nil {
		t.Fatalf("Table() error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "привет") || !strings.Contains(got, "мир") {
		t.Errorf("unicode wrap failed:\n%s", got)
	}
	if !strings.Contains(got, "\n") {
		t.Errorf("unicode text not wrapped onto new line:\n%s", got)
	}
}
