// Package output отвечает за рендеринг результатов команд: TTY-таблицы,
// сырой JSON для --json, служебные/цветные сообщения. Дисциплина stdout/stderr:
// данные — только в stdout, служебное — только в stderr (SPEC §4.3).
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Mode определяет режим рендеринга.
type Mode int

const (
	// ModeTTY — человекочитаемый вывод (таблицы, цвета, pager).
	ModeTTY Mode = iota
	// ModeJSON — машинный вывод: сырой JSON без обёрток, совместимый с jq.
	ModeJSON
)

// Option переопределяет поведение Printer, определяемое из окружения.
type Option func(*Printer)

// WithColor принудительно включает/выключает ANSI-цвета (по умолчанию —
// автоматически: только если stdout — TTY и YT_NO_COLOR != 1).
func WithColor(enabled bool) Option {
	return func(p *Printer) { p.color = enabled }
}

// WithWidth задаёт ширину терминала для переноса в таблицах (по умолчанию —
// определяется автоматически; 0 означает «ширина неизвестна»).
func WithWidth(n int) Option {
	return func(p *Printer) { p.width = n }
}

// Printer рендерит вывод команды в заданном режиме.
type Printer struct {
	out   io.Writer
	errw  io.Writer
	mode  Mode
	color bool
	width int
}

// New создаёт Printer. out — stdout (только данные), errw — stderr (только
// служебное). Цвет и ширина определяются из окружения и могут быть
// переопределены опциями.
func New(out, errw io.Writer, mode Mode, opts ...Option) *Printer {
	p := &Printer{
		out:   out,
		errw:  errw,
		mode:  mode,
		color: IsTerminal(out) && !noColorEnv(),
		width: TerminalWidth(out),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Mode возвращает текущий режим рендеринга.
func (p *Printer) Mode() Mode { return p.mode }

// JSON сообщает, что включён машинный вывод (--json).
func (p *Printer) JSON() bool { return p.mode == ModeJSON }

// TTY сообщает, что включён человекочитаемый вывод.
func (p *Printer) TTY() bool { return p.mode == ModeTTY }

// Color сообщает, включены ли ANSI-цвета.
func (p *Printer) Color() bool { return p.color }

// Width возвращает ширину терминала (0 — неизвестна).
func (p *Printer) Width() int { return p.width }

// Write пишет данные в stdout.
func (p *Printer) Write(data []byte) (int, error) { return p.out.Write(data) }

// Linef печатает строку в stdout.
func (p *Printer) Linef(format string, args ...any) error {
	_, err := fmt.Fprintf(p.out, format+"\n", args...)
	return err
}

// Successf печатает сообщение об успехе (зелёное «✓ ...» при включённых цветах).
func (p *Printer) Successf(format string, args ...any) error {
	return p.Linef("%s", Success(p.color, fmt.Sprintf(format, args...)))
}

// Warnf печатает предупреждение (жёлтое при включённых цветах) в stdout.
func (p *Printer) Warnf(format string, args ...any) error {
	return p.Linef("%s", Yellow(p.color, fmt.Sprintf(format, args...)))
}

// Errorf печатает сообщение об ошибке (красное при включённых цветах) в stderr.
func (p *Printer) Errorf(format string, args ...any) error {
	_, err := fmt.Fprintf(p.errw, "%s\n", Red(p.color, fmt.Sprintf(format, args...)))
	return err
}

// WriteJSON сериализует v в stdout как единый JSON-документ (без обёрток).
// HTML-символы не экранируются, чтобы вывод совпадал с ответом сервера.
// Допустим только в режиме ModeJSON.
func (p *Printer) WriteJSON(v any) error {
	if p.mode != ModeJSON {
		return errors.New("JSON output requested in TTY mode")
	}
	enc := json.NewEncoder(p.out)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// Raw пишет уже готовые JSON-байты в stdout, гарантируя завершающий перевод строки.
// Допустим только в режиме ModeJSON.
func (p *Printer) Raw(data []byte) error {
	if p.mode != ModeJSON {
		return errors.New("raw JSON output requested in TTY mode")
	}
	if _, err := p.out.Write(data); err != nil {
		return err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		_, err := p.out.Write([]byte{'\n'})
		return err
	}
	return nil
}
