package output

import (
	"bytes"
	"os"
	"testing"
)

func TestIsTerminalBuffer(t *testing.T) {
	if IsTerminal(bytes.NewBuffer(nil)) {
		t.Error("IsTerminal(buffer) = true, want false")
	}
}

func TestIsTerminalDevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Error("IsTerminal(/dev/null) = true, want false")
	}
}

func TestTerminalWidthBuffer(t *testing.T) {
	if w := TerminalWidth(bytes.NewBuffer(nil)); w != 0 {
		t.Errorf("TerminalWidth(buffer) = %d, want 0", w)
	}
}

func TestTerminalWidthDevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer f.Close()
	if w := TerminalWidth(f); w != 0 {
		t.Errorf("TerminalWidth(/dev/null) = %d, want 0", w)
	}
}

func TestNoColorEnv(t *testing.T) {
	t.Setenv("YT_NO_COLOR", "")
	if noColorEnv() {
		t.Error("noColorEnv() = true with empty env, want false")
	}
	t.Setenv("YT_NO_COLOR", "1")
	if !noColorEnv() {
		t.Error("noColorEnv() = false with 1, want true")
	}
	t.Setenv("YT_NO_COLOR", "true")
	if noColorEnv() {
		t.Error("noColorEnv() = true with \"true\", want false")
	}
}

func TestNewDisablesColorByEnv(t *testing.T) {
	t.Setenv("YT_NO_COLOR", "1")
	p, _, _ := newTestPrinter(ModeTTY, WithColor(true))
	if !p.Color() {
		t.Error("WithColor(true) must override YT_NO_COLOR")
	}
	p2, _, _ := newTestPrinter(ModeTTY)
	if p2.Color() {
		t.Error("Color() = true without explicit option and with YT_NO_COLOR=1")
	}
}

func TestPaint(t *testing.T) {
	if got, want := Paint(false, ANSIGreen, "ok"), "ok"; got != want {
		t.Errorf("Paint(disabled) = %q, want %q", got, want)
	}
	if got, want := Paint(true, ANSIGreen, "ok"), ANSIGreen+"ok"+ANSIReset; got != want {
		t.Errorf("Paint(enabled) = %q, want %q", got, want)
	}
	if got := Paint(true, ANSIGreen, ""); got != "" {
		t.Errorf("Paint(enabled, empty) = %q, want empty", got)
	}
}

func TestColors(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Green", Green(true, "x"), ANSIGreen + "x" + ANSIReset},
		{"Yellow", Yellow(true, "x"), ANSIYellow + "x" + ANSIReset},
		{"Red", Red(true, "x"), ANSIRed + "x" + ANSIReset},
		{"GreenPlain", Green(false, "x"), "x"},
		{"YellowPlain", Yellow(false, "x"), "x"},
		{"RedPlain", Red(false, "x"), "x"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestSuccess(t *testing.T) {
	if got, want := Success(true, "done"), ANSIGreen+"✓ done"+ANSIReset; got != want {
		t.Errorf("Success() = %q, want %q", got, want)
	}
	if got, want := Success(false, "done"), "✓ done"; got != want {
		t.Errorf("Success() plain = %q, want %q", got, want)
	}
}
