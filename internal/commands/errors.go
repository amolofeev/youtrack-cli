package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Коды выхода процесса (SPEC §4.4).
const (
	exitOK      = 0
	exitRuntime = 1
	exitUsage   = 2
	exitCancel  = 130
)

// ExitError — ошибка с кодом выхода процесса, отличным от 1. Для usage-ошибок
// (неизвестная команда/флаг, невалидные аргументы) код 2 (§4.4).
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// usageError оборачивает ошибку использования CLI (exit 2, §4.4).
func usageError(err error) error {
	return &ExitError{Code: exitUsage, Err: err}
}

// argsValidator оборачивает валидатор позиционных аргументов cobra так, чтобы
// ошибки валидации давали exit 2 (usage), а не runtime (1). Использовать во
// всех командах: Args: argsValidator(cobra.NoArgs).
func argsValidator(v cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := v(cmd, args); err != nil {
			return usageError(err)
		}
		return nil
	}
}

// unknownCommandArgs — валидатор root: любой позиционный аргумент корневой
// команды — неизвестная подкоманда (§4.4, exit 2). Воспроизводит сообщение
// дефолтного cobra.legacyArgs, включая блок «Did you mean this?».
func unknownCommandArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return usageError(fmt.Errorf("unknown command %q for %q%s", args[0], cmd.CommandPath(), suggestions(cmd, args[0])))
}

// suggestions копирует неэкспортируемый cobra.findSuggestions
// (vendor/github.com/spf13/cobra/command.go, func findSuggestions).
func suggestions(cmd *cobra.Command, arg string) string {
	if cmd.DisableSuggestions {
		return ""
	}
	if cmd.SuggestionsMinimumDistance <= 0 {
		cmd.SuggestionsMinimumDistance = 2
	}
	if s := cmd.SuggestionsFor(arg); len(s) > 0 {
		var b strings.Builder
		b.WriteString("\n\nDid you mean this?\n")
		for _, x := range s {
			fmt.Fprintf(&b, "\t%v\n", x)
		}
		return b.String()
	}
	return ""
}

// exitCodeFor вычисляет код выхода по ошибке (§4.4): ExitError — свой код,
// отмена контекста (SIGINT/SIGTERM, §4.5) — 130, прочее — 1 (runtime/API).
func exitCodeFor(err error) int {
	if err == nil {
		return exitOK
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	if errors.Is(err, context.Canceled) {
		return exitCancel
	}
	return exitRuntime
}

// formatError приводит ошибку к сообщению для stderr (§4.4): у ExitError
// печатается только внутренняя ошибка, остальные — как есть.
func formatError(err error) string {
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Err.Error()
	}
	return err.Error()
}
