# Структура подпроекта `yt/`

CLI-утилита для YouTrack (Go, cobra). Модуль `github.com/amolofeev/prompt-and-pray`.
Спека и планы — в корневом `docs/` (reference `spec`); здесь только устройство кода.

## Карта: путь → смысл

| Путь | Смысл |
| --- | --- |
| `cmd/yt/` | Точка входа (`main.go`) |
| `internal/api/` | HTTP-клиент YouTrack (auth, client, comments, fields, issues, projects, search, tags) |
| `internal/commands/` | cobra-команды: `root`, `auth`, `issue`, `search`, `command`, `project`, `tag`, `version` + валидация/конвенции |
| `internal/config/` | Конфигурация (файл конфига, token/auth) |
| `internal/output/` | Рендер результата: TTY-таблицы, `--json`, pager (`pager.go`) |
| `internal/version/` | Версия/commit/build, встраиваются через `-ldflags` |
| `testdata/` | Golden-файлы вывода команд (`*.golden`, SPEC §5.2); обновляются флагом `-update` |
| `docs/structure.md` | Этот файл — single source структуры подпроекта |
| `vendor/` | Генерат (`go mod vendor`) — не для поиска/чтения |
| `bin/` | Генерат (`make build` → `bin/yt`) — не для поиска/чтения |

## Поддержка

Файл — single source структуры. Обновляй его вместе с изменением структуры кода
(правило — `yt/AGENTS.md`).
