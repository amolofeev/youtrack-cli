# Структура подпроекта `yt/`

CLI-утилита для YouTrack (Go, cobra). Модуль `github.com/amolofeev/youtrack-cli`.
Подпроект изолирован в `yt/` и готовится к переносу в отдельный git-репозиторий (#53).

## Карта: путь → смысл

| Путь | Смысл |
| --- | --- |
| `cmd/yt/` | Точка входа (`main.go`) |
| `internal/api/` | HTTP-клиент YouTrack (auth, client, comments, fields, issues, projects, search, tags) |
| `internal/commands/` | cobra-команды: `root`, `auth`, `issue`, `search`, `command`, `project`, `tag`, `version` + валидация/конвенции |
| `internal/config/` | Конфигурация (файл конфига, token/auth) |
| `internal/output/` | Рендер результата: TTY-таблицы, `--json`, pager (`pager.go`) |
| `internal/version/` | Версия/commit/build, встраиваются через `-ldflags` |
| `README.md` | Пользовательская документация CLI: сборка, конфигурация, примеры ключевых команд |
| `VERSION` | Источник версии (`yt version`, ldflags); релизный тег `v$(cat VERSION)` |
| `testdata/` | Golden-файлы вывода команд (`*.golden`, SPEC §5.2); обновляются флагом `-update` |
| `.github/workflows/` | CI (`ci.yml`: vet/lint/test/build) и релиз (`release.yml`, тег `v*`) |
| `.opencode/` | Локальная память подпроекта (`yt_memory.md`) и снапшот спеки API (`openapi.json`) — gitignored |
| `docs/SPEC.md` | ТЗ (rev 1.1.1) — актуальная копия (архив — в корне монорепо) |
| `docs/PLAN.md` | Декомпозиция #5 — актуальная копия |
| `docs/MIGRATION.md` | План миграции yt в независимый git-репозиторий (#53) |
| `docs/structure.md` | Этот файл — single source структуры подпроекта |
| `vendor/` | Генерат (`go mod vendor`) — не для поиска/чтения |
| `bin/` | Генерат (`make build` → `bin/yt`) — не для поиска/чтения |

## Поддержка

Файл — single source структуры. Обновляй его вместе с изменением структуры кода
(правило — `yt/AGENTS.md`).
