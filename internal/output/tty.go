package output

import (
	"io"
	"os"

	"golang.org/x/term"
)

// ANSI-коды минимальной палитры (SPEC §4.3): зелёный — успех (✓),
// жёлтый — предупреждения, красный — ошибки в stderr.
const (
	ANSIReset  = "\x1b[0m"
	ANSIGreen  = "\x1b[32m"
	ANSIYellow = "\x1b[33m"
	ANSIRed    = "\x1b[31m"
)

// IsTerminal сообщает, является ли writer терминалом.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// TerminalWidth возвращает ширину терминала в колонках или 0, если определить
// не удалось.
func TerminalWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	wd, _, err := term.GetSize(int(f.Fd()))
	if err != nil || wd <= 0 {
		return 0
	}
	return wd
}

// noColorEnv учитывает YT_NO_COLOR: при значении «1» цвета выключены.
func noColorEnv() bool {
	return os.Getenv("YT_NO_COLOR") == "1"
}

// Paint оборачивает s в ANSI-код code, если цвета включены.
func Paint(enabled bool, code, s string) string {
	if !enabled || s == "" {
		return s
	}
	return code + s + ANSIReset
}

// Green красит s зелёным.
func Green(enabled bool, s string) string { return Paint(enabled, ANSIGreen, s) }

// Yellow красит s жёлтым.
func Yellow(enabled bool, s string) string { return Paint(enabled, ANSIYellow, s) }

// Red красит s красным.
func Red(enabled bool, s string) string { return Paint(enabled, ANSIRed, s) }

// Success добавляет зелёную галочку «✓ » перед сообщением об успехе.
func Success(enabled bool, s string) string { return Green(enabled, "✓ "+s) }
