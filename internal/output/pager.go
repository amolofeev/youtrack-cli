package output

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// DefaultPager — pager по умолчанию (SPEC §4.3): less без инициализации
// терминала (-X), с интерпретацией ANSI-цветов (-R) и выходом, если контент
// помещается на один экран (-F).
const DefaultPager = "less -FRX"

// pagerCommand возвращает команду pager из $PAGER или DefaultPager. PAGER=cat
// отключает pager (SPEC §4.3) — возвращается пустая строка.
func pagerCommand() string {
	pager := os.Getenv("PAGER")
	if pager == "" {
		return DefaultPager
	}
	if strings.TrimSpace(pager) == "cat" {
		return ""
	}
	return pager
}

// pagerState хранит запущенный pager-процесс и оригинальный stdout Printer-а.
type pagerState struct {
	cmd  *exec.Cmd
	pipe io.WriteCloser
	prev io.Writer
}

// pageable сообщает, применим ли pager для текущего Printer-а (SPEC §4.3):
// только в TTY-режиме, без --verbose, при терминальном stdout и PAGER != cat.
func (p *Printer) pageable() bool {
	if p.mode != ModeTTY || p.verbose {
		return false
	}
	if p.terminal != nil {
		if !*p.terminal {
			return false
		}
	} else if !IsTerminal(p.out) {
		return false
	}
	return pagerCommand() != ""
}

// Page запускает pager и направляет дальнейший вывод Printer-а в него
// (SPEC §4.3). Возвращает false, если paging неприменим или pager не удалось
// запустить — в этом случае вывод продолжает идти в stdout, а EndPage вызывать
// не требуется. При true вызывающий обязан завершить paging через EndPage.
func (p *Printer) Page() bool {
	if !p.pageable() {
		return false
	}
	parts := strings.Fields(pagerCommand())
	if len(parts) == 0 {
		return false
	}
	c := exec.Command(parts[0], parts[1:]...)
	c.Stdout = p.out
	c.Stderr = p.errw
	sin, err := c.StdinPipe()
	if err != nil {
		return false
	}
	if err := c.Start(); err != nil {
		return false
	}
	p.paging = &pagerState{cmd: c, pipe: &epipeWriter{w: sin}, prev: p.out}
	p.out = p.paging.pipe
	return true
}

// EndPage завершает paging: закрывает stdin pager-процесса и ждёт его
// завершения, после чего восстанавливает вывод в stdout. Безопасно вызывать,
// даже если paging не был запущен.
func (p *Printer) EndPage() error {
	if p.paging == nil {
		return nil
	}
	state := p.paging
	p.paging = nil
	p.out = state.prev
	if err := state.pipe.Close(); err != nil {
		return fmt.Errorf("close pager stdin: %w", err)
	}
	if err := state.cmd.Wait(); err != nil {
		return fmt.Errorf("pager failed: %w", err)
	}
	return nil
}

// epipeWriter глушит ошибки «broken pipe» при записи в pager: если пользователь
// рано вышел из pager-а (q в less), запись в его stdin возвращает EPIPE — это
// не ошибка команды, остаток вывода просто отбрасывается.
type epipeWriter struct {
	w io.WriteCloser
}

func (e *epipeWriter) Write(b []byte) (int, error) {
	n, err := e.w.Write(b)
	if errors.Is(err, syscall.EPIPE) {
		return n, nil
	}
	return n, err
}

func (e *epipeWriter) Close() error { return e.w.Close() }
