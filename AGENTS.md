# agents.md (yt/)

Проект `yt` — независимый подпроект в поддиректории `yt/` текущего репозитория
(см. корневой `AGENTS.md`, раздел «Подкаталоги как независимые проекты»).
Здесь собственные `go.mod`, `Makefile` и правила; корневой `AGENTS.md` действует
на уровне репозитория в целом.

## Локальные правила

- Все команды Go выполняются из каталога `yt/` (здесь свой `go.mod`).
- Модуль: `github.com/amolofeev/prompt-and-pray`; структура `cmd/` + `internal/` — по SPEC §2.2.
- Инструментарий: путь до Go — `~/sdk/go1.24.0/bin/go` (не в PATH), пользуйся им
  напрямую; golangci-lint — `~/go/bin/golangci-lint` (v1.64+).
- Так как Go нет в PATH, инструменты, зовущие `go` из PATH (golangci-lint, а также
  `make build/lint` с `GO ?= go`), падают с «go command required, not found». Запускай
  с явным PATH: `PATH="$HOME/sdk/go1.24.0/bin:$PATH" make lint` (или ту же команду
  golangci-lint напрямую). `make test`/`make vet` работают и без PATH (Go задан явно).
- Проверка форматирования — только свой код: `gofmt -l internal cmd`.
- Поведение зависимостей (cobra и т.п.) проверяй юнит-тестом, а не чтением исходников —
  тест быстрее, а факт остаётся в коде. Сверку API-сигнатур (например, какой метод есть
  в cobra v1.10.2) делай по исходникам из module cache (`go env GOMODCACHE`), одним
  заходом: поведение (рендер help, порядок групп) доказывается тестом.
- Конвенции команд `internal/commands` (Атом 2.4, #25): валидация аргументов — через
  `argsValidator(...)`, иначе голые cobra-валидаторы дают exit 1 вместо 2 (SPEC §4.4).
  API-клиент создаёт pipeline в `PersistentPreRunE` и кладёт в контекст — бери через
  `requireClient(cmd)` / `configFromContext(cmd)`, свой `api.New` не создавай.
- `yt/vendor/` и `yt/bin/` в репозитории не хранятся (gitignored, генерируются).
  Не создавай их без нужды: для чтения исходников зависимостей пользуйся module cache
  (`go env GOMODCACHE`), а не `go mod vendor`.
- Цели Makefile: `make build` → `bin/yt`; `make test`; `make lint`; `make vet`; `make integration`.
- Версия берётся из корневого `VERSION` (`../VERSION`) и встраивается через `-ldflags`
  в переменные `internal/version.{Version,Commit,Built}`; при сборке без ldflags — `unknown`.
- Объём и критерии приёмки — docs/SPEC.md (rev 1.1.1) и задачи GitHub (метка `atomic`).
- Сверка со спекой (§4.8/DoR): если живой сервер недоступен — использовать снапшот
  `/tmp/opencode/openapi.json` (сверить версию API = 2025.3), зафиксировать отклонение
  в комментарии задачи; финальная сверка — Атом 8.4.
- Если для задачи нужно поднять YouTrack (`localhost:8080`) или установить ПО —
  спросить человека, не делать автономно.
