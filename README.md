# yt — CLI для локального YouTrack

Консольная утилита для локального сервера YouTrack (Go 1.24, cobra).
Привязана к REST API YouTrack 2025.3 (`http://localhost:8080/api`).
Вывод ориентирован на терминал (TTY) и машину (`--json`).

```
Основное
  auth        Управление аутентификацией
  user        Пользователь

Issues
  command     Применить команды YouTrack к задачам
  issue       Работа с задачами
  search      Поиск задач

Сервер
  project     Проекты
  tag         Теги

Служебное
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  version     Вывести версию утилиты
```

## Сборка

```sh
make build    # → bin/yt
make test     # юнит- и golden-тесты
make vet      # go vet ./...
make lint     # golangci-lint
```

## Аутентификация и конфигурация

```sh
yt auth login    # сохранить адрес сервера и permanent token
yt auth status   # проверить статус
yt auth logout   # удалить сохранённый токен
```

Конфиг хранится в `~/.config/yt/config.yml` (токен — права `0600`).
Приоритет значений: флаг > переменная окружения > конфиг > дефолт.

| Параметр | Флаг | Env | Дефолт |
| --- | --- | --- | --- |
| URL API | `--base-url` | `YT_BASE_URL` | `http://localhost:8080/api` |
| Токен | `--token` | `YT_TOKEN` | из конфига |
| Каталог конфига | — | `YT_CONFIG_HOME` | `~/.config/yt` |
| Уровень лога | `--verbose` | `YT_LOG_LEVEL` | `error` |
| HTTP-таймаут | — | `YT_HTTP_TIMEOUT` | `30s` |
| Отключить цвета | — | `YT_NO_COLOR` | — |

Глобальные флаги: `--base-url`, `--token`, `--json`, `--verbose`, `-h/--help`.

## Примеры

### `yt issue list` — список задач

Список задач по поисковому запросу (GET `/issues`).

```sh
$ yt issue list --project DEMO -l 3
ID       STATE        CREATED     UPDATED     REPORTER  SUMMARY
DEMO-20  To do        2026-08-01  2026-08-01  admin     Smoke test issue edited (yt issue edit)
DEMO-5   In Progress  2026-08-01  2026-08-01  admin     First steps for project administrators
DEMO-3   To do        2026-08-01  2026-08-01  admin     First steps for system administrators
```

Флаги: `-q/--query` или позиционный аргумент, `-s/--state`, `-P/--project`,
`-a/--assignee`, `--tag`, `-l/--limit` (1..100), `--skip`.

JSON:

```sh
$ yt issue list --project DEMO -l 1 --json
[{"$type":"Issue","id":"3-19","idReadable":"DEMO-20","summary":"Smoke test issue edited (yt issue edit)","created":1785591755335,"updated":1785600781844,"project":{"$type":"Project","id":"0-0","shortName":"DEMO"},"reporter":{"$type":"User","id":"2-1","login":"admin","fullName":"admin"},"customFields":[{"$type":"SingleEnumIssueCustomField","id":"177-0","name":"Priority","value":{"name":"Normal","$type":"EnumBundleElement"}},{"$type":"SingleEnumIssueCustomField","id":"177-1","name":"Type","value":{"name":"Bug","$type":"EnumBundleElement"}},{"$type":"StateIssueCustomField","id":"177-2","name":"State","value":{"name":"To do","$type":"StateBundleElement"}},{"$type":"SingleUserIssueCustomField","id":"178-0","name":"Assignee","value":null},{"$type":"SingleOwnedIssueCustomField","id":"177-3","name":"Subsystem","value":null},{"$type":"MultiVersionIssueCustomField","id":"177-4","name":"Fix versions","value":[]},{"$type":"MultiVersionIssueCustomField","id":"177-5","name":"Affected versions","value":[]},{"$type":"SingleBuildIssueCustomField","id":"177-6","name":"Fixed in build","value":null},{"$type":"PeriodIssueCustomField","id":"184-0","name":"Estimation","value":null},{"$type":"PeriodIssueCustomField","id":"184-1","name":"Spent time","value":null}]}]
```

### `yt issue view` — карточка задачи

Карточка одной задачи (GET `/issues/{id}`).

```sh
$ yt issue view DEMO-20
DEMO-20  Smoke test issue edited (yt issue edit)
────────────────────────────────────────────────────────────────
STATE: To do  PROJECT: DEMO  REPORTER: admin
CREATED: 2026-08-01 13:42  UPDATED: 2026-08-01 16:13
────────────────────────────────────────────────────────────────
edited by yt issue edit smoke test
```

JSON:

```sh
$ yt issue view DEMO-20 --json
{"$type":"Issue","id":"3-19","idReadable":"DEMO-20","summary":"Smoke test issue edited (yt issue edit)","description":"edited by yt issue edit smoke test","created":1785591755335,"updated":1785600781844,"project":{"$type":"Project","id":"0-0","name":"Demo project","shortName":"DEMO"},"reporter":{"$type":"User","id":"2-1","login":"admin","fullName":"admin"},"updater":{"$type":"User","id":"2-1","login":"admin","fullName":"admin"},"customFields":[{"$type":"SingleEnumIssueCustomField","id":"177-0","name":"Priority","value":{"name":"Normal","id":"153-3","$type":"EnumBundleElement"}},{"$type":"SingleEnumIssueCustomField","id":"177-1","name":"Type","value":{"name":"Bug","id":"153-5","$type":"EnumBundleElement"}},{"$type":"StateIssueCustomField","id":"177-2","name":"State","value":{"name":"To do","id":"155-12","$type":"StateBundleElement"}},{"$type":"SingleUserIssueCustomField","id":"178-0","name":"Assignee","value":null},{"$type":"SingleOwnedIssueCustomField","id":"177-3","name":"Subsystem","value":null},{"$type":"MultiVersionIssueCustomField","id":"177-4","name":"Fix versions","value":[]},{"$type":"MultiVersionIssueCustomField","id":"177-5","name":"Affected versions","value":[]},{"$type":"SingleBuildIssueCustomField","id":"177-6","name":"Fixed in build","value":null},{"$type":"PeriodIssueCustomField","id":"184-0","name":"Estimation","value":null},{"$type":"PeriodIssueCustomField","id":"184-1","name":"Spent time","value":null}],"commentsCount":2}
```

### `yt issue create` — создание задачи

Создание задачи (POST `/issues`). `-p` — shortName, имя или ring-id проекта;
текст — `-t/--body` либо `--editor`.

```sh
$ yt issue create -p DEMO -t "Example issue for README (created by yt issue create)" -b "Created during Atom 8.4 to capture real output for the README."
✓ Created issue DEMO-47: Example issue for README (created by yt issue create)
```

JSON:

```sh
$ yt issue create -p DEMO -t "Example issue for README (created by yt issue create)" --json
{"$type":"Issue","id":"3-49","idReadable":"DEMO-48","summary":"Example issue for README (created by yt issue create)","project":{"$type":"Project","id":"0-0","shortName":"DEMO"}}
```

### `yt command` — команды YouTrack

Применение командного языка YouTrack к одной или нескольким задачам одним
запросом (POST `/commands`).

```sh
$ yt command "state: Done priority: Critical" DEMO-47 -y
✓ DEMO-47: state → Done, priority → Critical
```

Без `-y` в терминале запрашивается подтверждение. Флаги: `-m/--message`
(добавить комментарий), `--run-as`, `-y/--yes`.

JSON:

```sh
$ yt command "state: Done priority: Critical" DEMO-48 --json
[{"$type":"Issue","id":"3-49","idReadable":"DEMO-48","summary":"Example issue for README (created by yt issue create)","resolved":1785632721880,"project":{"$type":"Project","id":"0-0","shortName":"DEMO"}}]
```

### `yt user whoami` — текущий пользователь

```sh
$ yt user whoami
Login:    admin
Name:     admin
Email:
Guest:    false
ID:       2-1
```

```sh
$ yt user whoami --json
{"$type":"Me","id":"2-1","login":"admin","fullName":"admin","guest":false,"avatarUrl":"/hub/api/rest/avatar/fa387064-ecd2-4272-8723-9a953c9d3b5c?s=48"}
```

## Справочник команд

| Команда | Эндпоинт | Назначение |
| --- | --- | --- |
| `yt auth login/logout/status` | локально (конфиг); статус — `GET /users/me` | Аутентификация |
| `yt issue list [<query>]` | `GET /issues` | Список задач |
| `yt issue view <id>` | `GET /issues/{id}` | Карточка задачи |
| `yt issue create` | `POST /issues` | Создание задачи |
| `yt issue edit <id>` | `POST /issues/{id}` | Правка title/description |
| `yt issue close <id>...` | `POST /commands` | Закрытие через команду |
| `yt issue delete <id>` | `DELETE /issues/{id}` | Удаление задачи |
| `yt issue comment list/create <id>` | `GET/POST /issues/{id}/comments` | Комментарии |
| `yt search <query>` | `GET /issues` | Поиск |
| `yt search suggest <query>` | `POST /search/assist` | Подсказки поиска |
| `yt command <cmd> <id>...` | `POST /commands` | Команды YouTrack |
| `yt command assist <cmd>` | `POST /commands/assist` | Подсказки команд |
| `yt project list` | `GET /admin/projects` | Проекты |
| `yt tag list` | `GET /tags` | Теги |
| `yt user whoami` | `GET /users/me` | Текущий пользователь |
| `yt version` | — | Версия утилиты |

Авторизация — по permanent token; токен клиент не выводит и не логирует.

## Формат вывода и exit-коды

- stdout — только данные команды; stderr — служебное (ошибки, подтверждения, лог `--verbose`).
- Ошибки: `yt: <message>`, exit-коды — 1 (ошибка выполнения/API), 2 (ошибка
  использования), 130 (прерывание по сигналу), 0 (успех).
- Цвета/таблицы — только в терминале; `--json` — компактный JSON, пригодный для `jq`.
- Длинный вывод (`issue view`, `issue comment list`) в терминале страничится
  через `$PAGER` (по умолчанию `less -FRX`); `PAGER=cat` отключает.

## Интеграционные тесты

Read-only тесты против живого сервера (`localhost:8080`): `YT_INTEGRATION=1 make integration`.
Мутирующие тесты (create/edit/close/command/comment/delete) — только с дополнительным
`YT_INTEGRATION_MUTATE=1`, для них нужен `YT_TOKEN`. В CI интеграционные тесты
не запускаются.
