# agents.md (yt/)

Проект `yt` — независимый подпроект в поддиректории `yt/` текущего репозитория
(см. корневой `AGENTS.md`, раздел «Подкаталоги как независимые проекты»).
Здесь собственные `go.mod`, `Makefile` и правила; корневой `AGENTS.md` действует
на уровне репозитория в целом.

## Локальные правила

- Все команды Go выполняются из каталога `yt/` (здесь свой `go.mod`).
- Модуль: `github.com/amolofeev/prompt-and-pray`; структура `cmd/` + `internal/` — по SPEC §2.2.
- Инструментарий: Go 1.24.0 (`~/sdk/go1.24.0/bin/go`, не в PATH); golangci-lint v1.64+ (`~/go/bin`).
- Цели Makefile: `make build` → `bin/yt`; `make test`; `make lint`; `make vet`; `make integration`.
- Версия берётся из корневого `VERSION` (`../VERSION`) и встраивается через `-ldflags`
  в переменные `internal/version.{Version,Commit,Built}`; при сборке без ldflags — `unknown`.
- Объём и критерии приёмки — docs/SPEC.md (rev 1.1) и задачи GitHub (метка `atomic`).
