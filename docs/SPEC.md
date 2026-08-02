# Техническое задание: CLI-утилита `yt` для YouTrack

Версия документа: **1.1.1** · Статус: **Черновик** · Дата: 2026-07-31

> Ревизия 1.1.1 (по Атому 3.2, #19): исправлен §2.4 — Go-идентификатор `$Type` невалиден
> (Go не допускает `$` в идентификаторах), поле называется `Type` с json-тегом `$type`;
> jq-совместимость не меняется.

> Ревизия 1.1 закрывает вопросы ревизии ТЗ (issue #5, комментарий «Ревизия ТЗ…»):
> исправлена эвристика `id`/`idReadable` (§4.1), проект в `issue create` резолвится в `id`
> по официальной документации (§3.4), унифицирован формат `--json` (§4.3), добавлены
> `yt version` (§3.11), exit 130 (§4.4), уточнены `--with-token`, `--editor`, batch-ошибки
> `/commands` и Приложение А.

Документ является техническим заданием на реализацию CLI-утилиты `yt` на языке **Go 1.24.0**,
функционально аналогичной `gh` (GitHub CLI), но для локального сервера YouTrack (JetBrains).

ТЗ самодостаточно: на его основе можно вести задачи на реализацию без уточняющих вопросов.
Все привязки к REST API сделаны по спецификации сервера `http://localhost:8080/api/openapi.json`
(OpenAPI 3.0.1, версия API **2025.3**, 136 endpoint-ов, 218 схем). Перед началом реализации
спецификацию необходимо перекачать заново и сверить актуальность (см. §4.8).

---

## 1. Общие сведения

### 1.1. Назначение

`yt` — консольная утилита для работы с задачами (issues) и сопутствующими сущностями
локального сервера YouTrack из терминала: просмотр, поиск, создание, редактирование,
закрытие, удаление задач, комментарии, выполнение команд командного языка YouTrack
(`state: Fixed, Priority: High`), просмотр проектов, тегов и собственного профиля.

Утилита пригодна для интерактивного использования (TTY) и для скриптов/CI
(флаг `--json`, предсказуемый exit-код).

### 1.2. Целевая аудитория

- Разработчики, работающие с задачами YouTrack из терминала.
- DevOps/инженеры, автоматизирующие работу с задачами в CI (создание/закрытие issues, сбор статистики).

### 1.3. Область применения и границы (что НЕ входит в v1)

В **v1** входят команды из §3: `auth`, `issue`, `search`, `command`, `project`, `user`, `tag`, `version`.

В **v1 НЕ входит** (перспективные направления, см. §3.10):

- Администрирование сервера (пользователи, группы, кастомные поля, бандлы, бэкапы, глобальные настройки).
- Агильные доски/спринты (`/agiles`).
- Статьи базы знаний (`/articles`).
- Time tracking (`/timeTracking`, `workItems`).
- Вложения, реакции, вотчеры, голоса, VCS-изменения.
- Saved queries.
- Драфты задач (draft API).

Эти направления зафиксированы как кандидаты на **v1.1+** (§3.10) и при реализации не должны
ломать API и структуру пакетов v1.

### 1.4. Имя и артефакты

- Имя исполняемого файла и команды: `yt`.
- Go-модуль: `github.com/amolofeev/youtrack-cli` (репозиторий youtrack-cli, код — `cmd/yt`
  и пакеты `internal/...`).
- Требование к инструментарию: **Go 1.24.0** и выше (в `go.mod` — `go 1.24.0`).
- Версия утилиты: берётся из файла `VERSION` в корне репозитория (формат `MAJOR.MINOR.PATCH`,
  текущее значение `0.0.1-pre-alpha`). При сборке версия встраивается через
  `-ldflags "-X .../internal/version.Version=$(cat VERSION)"`.

### 1.5. Требования к окружению

- ОС: Linux и macOS (v1); Windows — допустимо, но без гарантий (пути конфигурации через `os.UserConfigDir()`).
- Сеть: доступность базового URL сервера (по умолчанию `http://localhost:8080/api`).
- Наличие permanent token'а YouTrack с правами на целевые операции.

---

## 2. Архитектура и стек

### 2.1. Общая схема

```
┌────────────┐   cobra/pflag    ┌─────────────┐   internal/api    ┌─────────────┐
│  cmd/yt    │ ───────────────► │ internal/   │ ────────────────► │  YouTrack   │
│ (main.go)  │   регистрация    │ commands    │   HTTP+fields     │  REST API   │
└────────────┘                  │   (cobra)   │ ◄──────────────── │  localhost  │
                               └─────────────┘   JSON response    └─────────────┘
                                      │                ▲
                                      ▼                │
                               ┌─────────────┐   ┌─────────────┐
                               │ internal/   │   │ internal/   │
                               │ config      │   │ output      │
                               │ (config.yml)│   │ (tty/json)  │
                               └─────────────┘   └─────────────┘
```

Поток выполнения команды:

1. `main.go` вызывает `commands.NewRootCommand()` и `Execute()`.
2. Cobra разбирает флаги и аргументы, загружает конфигурацию
   (приоритет флаг > env > config > дефолт, см. §3.2).
3. Команда формирует объект запроса (endpoint, параметры, список `fields`), передаёт в `internal/api.Client`.
4. Клиент выполняет HTTP-запрос (таймаут, retry, обработка ошибок §4.4–4.5).
5. Результат передаётся в `internal/output` для рендеринга (TTY-таблица / `--json` / pager).

### 2.2. Структура репозитория

```
.
├── cmd/
│   └── yt/
│       └── main.go                  # точка входа: вызов commands.Execute()
├── internal/
│   ├── api/                         # HTTP-клиент YouTrack
│   │   ├── client.go                # base transport: URL, заголовки, таймаут, retry, ошибки
│   │   ├── auth.go                  # GET /users/me — проверка токена
│   │   ├── issues.go                # GET/POST/DELETE /issues, /issues/{id}
│   │   ├── comments.go              # GET/POST /issues/{id}/comments
│   │   ├── commands.go              # POST /commands, /commands/assist
│   │   ├── search.go                # GET /issues (query), POST /search/assist
│   │   ├── projects.go              # GET /admin/projects
│   │   ├── tags.go                  # GET /tags
│   │   ├── types.go                 # структуры ответов (Issue, IssueComment, ...)
│   │   └── fields.go                # списки полей fields для каждой операции
│   ├── commands/                    # Cobra-команды
│   │   ├── root.go                  # root: глобальные флаги, конфиг, обработка ошибок
│   │   ├── auth_cmd.go
│   │   ├── issue_cmd.go
│   │   ├── search_cmd.go
│   │   ├── command_cmd.go
│   │   ├── project_cmd.go
│   │   ├── user_cmd.go
│   │   ├── tag_cmd.go
│   │   └── version_cmd.go
│   ├── config/                      # чтение/запись ~/.config/yt/config.yml, приоритеты
│   ├── output/                      # рендеринг: таблицы, JSON, pager, работа с TTY
│   │   ├── table.go
│   │   ├── json.go
│   │   ├── pager.go
│   │   └── tty.go
│   └── version/                     # версия, commit, дата сборки (закладываются ldflags)
├── docs/
│   └── SPEC.md                      # данный документ
├── testdata/                        # golden-файлы для тестов вывода
├── .golangci.yml
├── Makefile                         # build / test / lint / integration
├── go.mod
└── go.sum
```

Требование к зависимостям: пакет `cmd` не должен содержать бизнес-логики (только инициализация
и вызов команд). Вся логика — в `internal/...`.

### 2.3. CLI-фреймворк

- **Cobra** (`github.com/spf13/cobra`) + **pflag** (`github.com/spf13/pflag`) — как в `gh`.
- `SilenceUsage: true`, `SilenceErrors: true`: ошибки печатаются в stderr один раз (§4.4),
  без дампа справки при runtime-ошибке.
- Группировка подкоманд по разделам справки (`Root().AddGroup(...)`): `Основное`, `Issues`,
  `Сервер`, `Служебное`.
- Справка команды содержит: описание, флаги с дефолтами, примеры (1–3 штуки).
- `completion` — встроенная команда Cobra (bash/zsh/fish).

### 2.4. Подход к API-клиенту: выбор и обоснование

**Решение: рукописный клиент** (не оapi-codegen и не openapi-generator).

Обоснование:

1. **`$type`-полиморфизм.** Спека YouTrack активно использует `discriminator: propertyName: $type`
   (схемы `Issue`, `IssueComment`, `IssueCustomField`, `CommandList`, `Suggestion` и др.).
   Генераторы порождают для таких схем интерфейсы с type-switch'ами, что увеличивает объём
   кода и усложняет его поддержку — при том что CLI нужны лишь скалярные поля и 1–2 уровня вложенности.
2. **Параметр `fields`.** YouTrack требует осознанного запроса полей в каждом запросе (§4.2).
   В сгенерированном клиенте списки полей превращаются в непрозрачные строки `queryParams`,
   то есть типизация не выигрывается — всё равно приходится писать `fields` вручную.
3. **Объём покрытия.** В v1 нужны ~20 операций из 136 endpoint-ов. Генерация по полной спеке
   тянет сотни неиспользуемых типов; генерация по вырезанному подмножеству требует
   сопровождения вырезки, что дороже рукописных типов.
4. **Гибкость.** Рукописный клиент позволяет: скрывать кастомные поля из списков,
   отображать state-поле, свободно переиспользовать поля в renderer'ах, точно управлять
   ошибками и временными полями (`$type` в ответах).

Правила рукописного клиента:

- Все endpoint-обёртки живут в `internal/api` и принимают на вход **список полей** (см. §4.2).
- Структуры ответов в `types.go` используют **указатели/`omitempty`** для опциональных полей
  (ответы YouTrack опускают незапрошенные поля).
- Изменяемые поля (`summary`, `description`, `text`, `project.id` и т.п.) типизируются явно.
- `$type` в структурах декларируется как `Type string \`json:"$type,omitempty"\`` (поле
  `Type`, тэг `$type`) — чтобы сохранить совместимость с `jq` при `--json`.

**Альтернатива на будущее:** при расширении на v2 (полное покрытие админки) допустимо
перейти на oapi-codegen с генерацией по предварительно вырезанному подмножеству спецификации.
До тех пор интерфейс `internal/api` следует проектировать так, чтобы замена реализации
(рукописной на сгенерированную) не затронула пакеты `commands` и `output`.

### 2.5. Зависимости (v1)

| Модуль | Назначение |
|---|---|
| `github.com/spf13/cobra` (+ `pflag`) | CLI-фреймворк |
| `gopkg.in/yaml.v3` | чтение/запись `config.yml` |
| `golang.org/x/term` | определение TTY, ширина терминала |
| stdlib (`net/http`, `text/tabwriter`, `encoding/json`, `context`, `time`, `errors`) | транспорт, таблицы, JSON |

Иных зависимостей не вводить без явного обоснования в PR/задаче. Все зависимости совместимы
с Go 1.24.0 (проверяется `go vet` и сборкой в CI).

### 2.6. Версионирование и сборка

- `Makefile`:
  - `make build` — сборка `bin/yt` (`-trimpath -ldflags` с версией из `VERSION`).
  - `make test` — `go test ./...`.
  - `make lint` — `golangci-lint run`.
  - `make vet` — `go vet ./...`.
  - `make integration` — интеграционные тесты (см. §5.4).
- CI (GitHub Actions, опционально на этом этапе): на PR — `vet`, `lint`, `test`, `build`.

---

## 3. Функциональные требования

> Обозначения в таблицах команд: `[ ]` — опционально, `(...)` — значения по умолчанию,
> все пути указаны относительно базового URL `http://localhost:8080/api` (по умолчанию).

### 3.1. Глобальные флаги

Присутствуют у всех команд (включая подкоманды):

| Флаг | Тип | Назначение |
|---|---|---|
| `--base-url` | string | Базовый URL API. Дефолт — из config/env, иначе `http://localhost:8080/api` |
| `--token` | string | Permanent token. Дефолт — из config/env |
| `--json` | bool | Машинный вывод (JSON) вместо TTY-рендера |
| `--verbose` | bool | Подробный лог в stderr (уровень `debug`) |
| `--help`, `--version` | — | стандартные (Cobra) |

Приоритет разрешения значений: **флаг > env > config > дефолт** (см. §3.2).

### 3.2. Конфигурация и приоритеты

**Файл конфигурации:** `~/.config/yt/config.yml`
(путь через `os.UserConfigDir()`; переопределяется `YT_CONFIG_HOME` для тестов и скриптов).

```yaml
# ~/.config/yt/config.yml
base_url: http://localhost:8080/api
token: perm:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Права на файл: `0600`. При `auth login` файл создаётся автоматически (включая каталоги `mkdir -p`).

**Переменные окружения:**

| Переменная | Назначение |
|---|---|
| `YT_BASE_URL` | базовый URL API |
| `YT_TOKEN` | permanent token |
| `YT_CONFIG_HOME` | каталог конфигурации (переопределение пути) |
| `YT_LOG_LEVEL` | уровень логирования (`error`, `warn`, `info`, `debug`) |
| `YT_HTTP_TIMEOUT` | таймаут запроса (сек., по умолчанию `30`) |
| `YT_NO_COLOR` | при `1` — отключить ANSI-цвета |

**Приоритет значений** (от высшего к низшему):

```
флаги  >  env (YT_*)  >  config.yml  >  встроенные дефолты
```

> Решение (подтверждено ревизией, issue #5): принят стандартный порядок
> «флаги > env > config > дефолт». В исходной постановке указан порядок «env > config > флаги» —
> он отклонён: иначе флагом нельзя переопределить env-переменную в конкретном вызове,
> что ломает скрипты/CI (флаг — самый явный и самый «короткоживущий» способ управления).
> Пересмотр этого пункта возможен только сознательным решением владельца ТЗ.

**Проверка авторизации:** при первом сетевом обращении команда проверяет наличие токена;
невалидность токена (HTTP 401) обрабатывается по §4.4 (сообщение «не авторизован, выполните `yt auth login`»).

### 3.3. `yt auth` — аутентификация

| Команда | Эндпоинт | Метод | Параметры запроса |
|---|---|---|---|
| `yt auth login` | `/users/me` (проверка токена) | GET | `fields=id,login,fullName,email` |
| `yt auth status` | `/users/me` (проверка токена) | GET | `fields=id,login,fullName,email,guest` |
| `yt auth logout` | — | — | (удаление токена из конфига) |

#### `yt auth login`

Интерактивно запрашивает и сохраняет учётные данные:

```
yt auth login
? Base URL: http://localhost:8080/api
? Token: ••••••••••••••••••••••••••
✓ Authenticated as alex@example.com (Alex)
```

- В неинтерактивном режиме (нет TTY): токен читается из stdin — в этом случае он не
  печатается и не попадает в shell history/`ps`.
- Токен, переданный флагом `--with-token`, **не является скрытым**: значение видно
  в shell history (как аргумент команды) и в списке процессов (`ps`). Это штатное
  ограничение; указано в справке команды.
- base URL — из флага `--base-url` / `YT_BASE_URL` / дефолта.
- Флаги: `--with-token` (string, токен из аргумента/флага), `--base-url` (string).
- Перед сохранением выполняется проверка токена запросом `GET /users/me`.
- При успехе конфиг сохраняется; печатается сообщение `✓ Authenticated as <login> (<fullName>)`.
- При ошибке (401/403/сеть) — сообщение об ошибке, exit 1, конфиг **не** сохраняется.

#### `yt auth logout`

- Удаляет `token` из конфига. Base URL сохраняется.
- Если токена в конфиге нет — сообщение `not logged in`, exit 1.
- При успехе: `✓ Logged out`.

#### `yt auth status`

- Печатает: базовый URL, логин, полное имя, email, признак guest.
- Проверяет токен живым запросом `GET /users/me`; при 401 — `✗ not logged in`, exit 1.
- Пример (TTY):

```
yt (0.0.1-pre-alpha)
Server:   http://localhost:8080/api
Login:    alex
Name:     Alex
Email:    alex@example.com
Guest:    false
✓ Authenticated
```

- `--json`:

```json
{"baseUrl":"http://localhost:8080/api","login":"alex","fullName":"Alex","email":"alex@example.com","guest":false}
```

  `baseUrl` — единственное поле, добавляемое утилитой (его нет в ответе `/users/me`);
  добавление задокументировано в §4.3, именование — camelCase (стиль сервера).

### 3.4. `yt issue` — работа с задачами

| Команда | Эндпоинт | Метод | Параметры запроса |
|---|---|---|---|
| `yt issue list [<query>]` | `/issues` | GET | `query`, `customFields`, `fields`, `$skip`, `$top` |
| `yt issue view <id>` | `/issues/{id}` | GET | `fields` |
| `yt issue create` | `/issues` | POST | `fields`, `draftId`(нет в v1), `muteUpdateNotifications`(нет в v1) |
| `yt issue edit <id>` | `/issues/{id}` | POST | `fields`, `muteUpdateNotifications` |
| `yt issue close <id>...` | `/commands` | POST | `fields`, `muteUpdateNotifications`(нет в v1) |
| `yt issue delete <id>` | `/issues/{id}` | DELETE | — |
| `yt issue comment list <id>` | `/issues/{id}/comments` | GET | `fields`, `$skip`, `$top` |
| `yt issue comment create <id>` | `/issues/{id}/comments` | POST | `fields` |

Идентификатор `<id>` в v1 принимает **и ring-id, и `idReadable`** (`PROJ-42`) — оба значения
передаются в path-параметр `{id}` без преобразований (YouTrack резолвит оба). URL-кодирование
значения обязательное.

#### `yt issue list [<query>]`

Список задач. Аргумент `<query>` (опционально) — строка поиска YouTrack (e.g. `project: PRJ`,
`#Unresolved`). Дополнительно флаги формируют префикс запроса:

| Флаг | Что добавляет к запросу |
|---|---|
| `-q, --query string` | полный поисковый запрос (заменяет позиционный аргумент) |
| `-s, --state string` | `state: <value>` (значения `open`/`resolved`/`all` транслируются в `#Unresolved`/`#Resolved`/``, иначе подставляются как есть) |
| `-P, --project string` | `project: <name>` |
| `-a, --assignee string` | `assignee: <login>` |
| `-l, --limit int` | `$top` (по умолчанию 30, максимум 100) |
| `--skip int` | `$skip` (по умолчанию 0) |
| `--tag string` | `tag: <name>` (повторяемый флаг) |

Флаги комбинируются через `AND` (конкатенация через пробел). Порядок сортировки не
гарантируется сервером без `order by:` в запросе — это задокументированное поведение.

Поля (`fields`):
`id,idReadable,summary,created,updated,resolved,project(id,shortName),reporter(id,login,fullName),customFields(id,name,value($type,name))`

Вывод TTY (таблица, `text/tabwriter`):

```
ID      STATE     CREATED       UPDATED       REPORTER  SUMMARY
PRJ-42  Open      2026-07-01   2026-07-05    alex      Fix login flow
PRJ-43  Fixed     2026-07-02   2026-07-06    alex      Write TZ for yt CLI
```

Колонка `STATE` берётся из поля кастомного поля с именем `State` (если есть и `value.name` не пуст).

`--json` — массив объектов как с сервера:

```json
[
  {
    "id": "2-1",
    "idReadable": "PRJ-42",
    "summary": "Fix login flow",
    "created": 1784160000000,
    "updated": 1784534400000,
    "resolved": null,
    "project": {"id": "0-0", "shortName": "PRJ"},
    "reporter": {"id": "1-1", "login": "alex", "fullName": "Alex"},
    "customFields": [{"id": "1-2", "name": "State", "value": {"$type": "StateBundleElement", "name": "Open"}}]
  }
]
```

При пустом результате в TTY: `No issues found for query "<query>"` (exit 0); при `--json`: `[]`.

#### `yt issue view <id>`

Просмотр задачи. Флаги:

| Флаг | Назначение |
|---|---|
| `-c, --comments` | вывести комментарии после тела задачи |
| `-C, --comments-limit int` | максимальное число комментариев (по умолчанию 20, при `--comments`) |
| `--json` | сырой объект Issue |

Поля (`fields`):
`id,idReadable,summary,description,created,updated,resolved,project(id,shortName,name),reporter(id,login,fullName,email),updater(id,login,fullName),customFields(id,name,value($type,id,name,login,fullName,minutes,minutesPerDay,presentation)),tags(id,name),commentsCount`

При `--comments` дополнительно запрашивается `GET /issues/{id}/comments`
(`fields=$type,id,text,created,author(id,login,fullName)`, `$top=$comments-limit`).

`--json` (без `--comments`) — сырой объект Issue (поля из списка выше; `comments` не включён).
`--json --comments` — тот же объект с добавленным ключом `comments` (массив объектов
`IssueComment` из отдельного запроса, поля `$type,id,text,created,author(id,login,fullName)`).
Добавление поля, которого нет в ответе `/issues/{id}`, — задокументированное исключение §4.3.

TTY-вывод:

```
PRJ-42  Fix login flow
────────────────────────────────────────────────────────────────
STATE: Open      PROJECT: PRJ        REPORTER: alex
CREATED: 2026-07-01 14:00  UPDATED: 2026-07-05 10:00
Tags: backend, auth
────────────────────────────────────────────────────────────────
<description или "No description">

Comments (2):
───────────
alex · 2026-07-01 14:05
First comment text.

ivan · 2026-07-03 09:00
Second comment text.
```

#### `yt issue create`

Создание задачи. Флаги:

| Флаг | Обязательность | Назначение |
|---|---|---|
| `-p, --project string` | да | shortName, имя или ring-id проекта (резолвится в `project.id`, см. ниже) |
| `-t, --title string` | да* | summary; *обязателен, если не задан `--editor` |
| `-b, --body string` | нет | description (взаимоисключающе с `--editor`) |
| `--editor` | нет | открыть `$EDITOR` (или `vi`) для ввода текста (см. формат шаблона) |
| `--json` | нет | вывести созданный объект |

Резолвинг проекта: по официальной документации YouTrack создание issue требует
`project` в виде **`id`** (ring-id). Порядок:

1. Если `--project` соответствует ring-id (`^[0-9]+-[0-9]+$`) — используется как есть.
2. Иначе выполняется `GET /admin/projects?fields=id,shortName,name&$top=200`
   (у эндпоинта нет query-параметра, поэтому фильтрация клиентская): совпадение ищется
   по `shortName` (без учёта регистра), затем по `name`. Не найдено →
   `project <value> not found`, exit 1.

Тело запроса (`POST /issues?fields=id,idReadable,summary,project(id,shortName)`):

```json
{"project": {"id": "0-0"}, "summary": "Fix login flow", "description": "Steps: ..."}
```

Формат редактора (`--editor`): открывается `$EDITOR` (или `vi`) с шаблоном:

```
Summary: Fix login flow

Description:
Steps to reproduce:
1. ...
```

- Строка `Summary: <текст>` — summary (обязателен; пусто → `no summary provided`, exit 1).
- Всё, что после строки `Description:`, — description (может быть пустым).
- Если задан `-t`, строка `Summary:` в шаблоне подставляется с этим значением,
  редактируется только description.
- `-b` и `--editor` взаимоисключающие (одновременное использование — ошибка использования, exit 2).

Поля ответа: `id,idReadable,summary,project(id,shortName)`.

TTY: `✓ Created issue PRJ-42: Fix login flow`. `--json` — объект Issue.

Валидация до запроса: обязателен `--project`; обязателен либо `--title`, либо `--editor`.
При ошибке 400 сервер вернёт описание — выводится как есть (см. §4.4).

#### `yt issue edit <id>`

Изменение задачи. Флаги: `--title string`, `--body string`. Хотя бы один обязателен.
Изменяется только то, что передано (частичное обновление).

Тело запроса (`POST /issues/{id}`):

```json
{"summary": "New summary", "description": "New description"}
```

Поля ответа: `id,idReadable,summary,description`.

TTY: `✓ Updated issue PRJ-42`. `--json` — объект Issue.

#### `yt issue close <id>...`

Закрытие (перевод в resolved-состояние) одной или нескольких задач.

Реализуется через командный язык YouTrack:

| Флаг | Назначение |
|---|---|
| `-s, --state string` | состояние разрешения (по умолчанию `Fixed`) |
| `-m, --message string` | комментарий, добавляемый вместе с командой |
| `-y, --yes` | не запрашивать подтверждение в TTY |
| `--json` | вывести обновлённые задачи |

Тело запроса (`POST /commands`):

```json
{
  "query": "state: Fixed",
  "comment": "Resolved by yt",
  "issues": [{"idReadable": "PRJ-42"}, {"idReadable": "PRJ-43"}]
}
```

Поля ответа: `issues(id,idReadable,summary,resolved,project(id,shortName))`.

TTY (подтверждение при TTY, если нет `-y`):

```
! This will close 2 issue(s) via command "state: Fixed". Continue? [y/N] y
✓ PRJ-42 → Fixed
✓ PRJ-43 → Fixed
```

`--json` — массив `issues` из ответа.

Команда применяется **одним запросом** к списку задач. Если команда не может быть применена
(например, нет прав или задача уже в запрошенном состоянии), сервер возвращает HTTP-ошибку
(400/403) с `error_description` для **всего** запроса — изменения не применяются, CLI выводит
ошибку (§4.4) и завершается с exit 1. Частичное применение к части списка не документируется;
фактическое поведение фиксируется интеграционным тестом (§5.4) против целевого сервера.

#### `yt issue delete <id>`

Удаление задачи. При TTY запрашивает подтверждение (если нет `-y/--yes`):

```
! Warning: this will permanently delete PRJ-42. Continue? [y/N]
```

- Успех: `✓ Deleted issue PRJ-42` (exit 0).
- Отмена: сообщение `Aborted`, exit 1.

#### `yt issue comment list <id>`

Список комментариев. Флаги: `--limit int` (`$top`, по умолчанию 30), `--skip int`.

Поля: `$type,id,text,created,author(id,login,fullName)`.

TTY:

```
alex · 2026-07-01 14:05
First comment text.
────
ivan · 2026-07-03 09:00
Second comment text.
```

`--json` — массив `IssueComment`. Пустой список: `No comments for PRJ-42` / `[]`.

#### `yt issue comment create <id>`

Добавление комментария. Флаг `-m, --message string` (обязателен) либо `--editor`.

Тело запроса (`POST /issues/{id}/comments`): `{"text": "..."}`.

Поля ответа: `$type,id,text,created,author(id,login)`.

TTY: `✓ Added comment to PRJ-42`. `--json` — объект `IssueComment`.

### 3.5. `yt search`

| Команда | Эндпоинт | Метод | Параметры запроса |
|---|---|---|---|
| `yt search <query>` | `/issues` | GET | `query`, `fields`, `$top`, `$skip` |
| `yt search suggest "<частичный запрос>"` | `/search/assist` | POST | `fields` |

#### `yt search <query>`

Поиск задач по произвольному запросу. По сути это `issue list` без «умных» флагов
(`state/project/assignee` не транслируются): аргумент — сырой поисковый запрос.

Флаги: `-l, --limit int` (по умолчанию 30), `--skip int`, `--json`.

Поля — как у `issue list`; TTY — та же таблица.

#### `yt search suggest "<query>"`

Автодополнение поискового запроса (обёртка над `/search/assist`).

Тело запроса:

```json
{"query": "project: PRJ state: O"}
```

Поля ответа: `query,suggestions(option,description,prefix,suffix,group)`.

TTY — список подсказок в формате `option — description` (сгруппирован по `group`):

```
Commands:
state: Open — matches issues in Open state
...
```

`--json` — объект `SearchSuggestions` (без полей, помеченных как неиспользуемые).

Назначение: поддержка интерактивного автодополнения в оболочке и справка по синтаксису.
В v1 — только вывод, без интерактивного UI.

### 3.6. `yt command` — командный язык YouTrack

| Команда | Эндпоинт | Метод | Параметры запроса |
|---|---|---|---|
| `yt command "<команды>" <issue>...` | `/commands` | POST | `fields`, `muteUpdateNotifications`(нет в v1) |
| `yt command assist "<команды>"` | `/commands/assist` | POST | `fields` |

#### `yt command "<commands>" <issue>...`

Применяет одну или несколько команд к задачам. Пример:

```
yt command "state: Fixed Priority: High" PRJ-42 PRJ-43
```

Флаги: `-m, --message string` (комментарий с командой), `--run-as string` (исполнить от имени
другого пользователя, если разрешено), `-y, --yes`, `--json`.

Тело запроса (`POST /commands`):

```json
{
  "query": "state: Fixed Priority: High",
  "comment": "Triaged",
  "issues": [{"idReadable": "PRJ-42"}, {"idReadable": "PRJ-43"}]
}
```

Поля ответа: `issues(id,idReadable,summary,resolved,project(id,shortName))`.

TTY:

```
✓ PRJ-42: state → Fixed, Priority → High
✓ PRJ-43: state → Fixed, Priority → High
```

При ошибке в команде (синтаксис, недопустимое значение) сервер вернёт `error_description` —
выводится в stderr, exit 1.

#### `yt command assist "<commands>"`

Предварительный разбор команды и подсказки (без применения). Полезно для валидации
команд в CI и для интерактивного автодополнения.

Тело запроса: `{"query": "<команды>", "caret": <len>}`.

Поля ответа: `query,suggestions(option,description,prefix,suffix,group)`.

TTY: `OK: <команда> — <описание>` для каждой подсказки, либо сообщение об ошибке парсинга.
`--json` — объект `CommandList`.

### 3.7. `yt project`

| Команда | Эндпоинт | Метод | Параметры запроса |
|---|---|---|---|
| `yt project list` | `/admin/projects` | GET | `fields`, `$skip`, `$top` |

Флаги: `-l, --limit int` (по умолчанию 50), `--skip int`, `--json`.

Поля: `id,name,shortName,archived,leader(id,login,fullName)`.

TTY:

```
SHORTNAME  NAME              ARCHIVED  LEADER
PRJ        Project One       false     alex
```

`--json` — массив объектов `Project`.

### 3.8. `yt user`

| Команда | Эндпоинт | Метод | Параметры запроса |
|---|---|---|---|
| `yt user whoami` | `/users/me` | GET | `fields` |

Поля: `id,login,fullName,email,guest,avatarUrl`.

TTY:

```
Login:    alex
Name:     Alex
Email:    alex@example.com
Guest:    false
ID:       1-1
```

`--json`:

```json
{"id":"1-1","login":"alex","fullName":"Alex","email":"alex@example.com","guest":false,"avatarUrl":"..."}
```

### 3.9. `yt tag`

| Команда | Эндпоинт | Метод | Параметры запроса |
|---|---|---|---|
| `yt tag list` | `/tags` | GET | `query`, `fields`, `$skip`, `$top` |

Флаги: `-q, --query string` (фильтр по имени тега), `-l, --limit int` (по умолчанию 50), `--json`.

Поля: `id,name,untagOnResolve`.

TTY:

```
NAME     UNTAG ON RESOLVE
backend  false
auth     true
```

`--json` — массив объектов `Tag`.

### 3.10. Перспективные команды (v1.1+)

В v1 не реализуются, но учитываются при проектировании пакетов (`internal/api` пополняется
файлами, не меняя контракты):

- `yt agile`/`yt sprint` — `/agiles`, `/agiles/{id}/sprints` (спринты, доски).
- `yt article` — `/articles` (база знаний), вложения, комментарии, теги статей.
- `yt workitem`/`yt time` — `/issues/{id}/timeTracking`, `/issues/{id}/timeTracking/workItems`,
  `/admin/timeTrackingSettings`.
- `yt attachment` — `/issues/{id}/attachments` (загрузка/скачивание).
- Взаимодействия: вотчеры, голоса, реакции (`/issues/{id}/comments/{id}/reactions`).
- VCS-изменения (`/issues/{id}/vcsChanges`).
- Saved queries (`/savedQueries`).

Эти команды должны строиться на тех же примитивах `internal/api` (fields, retry, ошибки)
и `internal/output` (table/json/pager), без новых механизмов.

### 3.11. `yt version`

Служебная команда — вывод версии утилиты. Не требует токена и подключения к серверу.

TTY:

```
yt version 0.0.1-pre-alpha
commit: 2036315
built:  2026-07-31T12:00:00Z
go:     go1.24.0
os:     linux
arch:   amd64
```

- `commit` и `built` встраиваются через `-ldflags` (§1.4); при сборке без них — `unknown`.
- `--json`:

```json
{"version":"0.0.1-pre-alpha","commit":"2036315","built":"2026-07-31T12:00:00Z","go":"go1.24.0","os":"linux","arch":"amd64"}
```

- Встроенный флаг Cobra `--version` на корне печатает только первую строку
  (`yt version 0.0.1-pre-alpha`) и завершается с кодом 0.
- Флаг `--json` не требует авторизации и работает без сети.

---

## 4. Ключевые технические требования

### 4.1. Идентификация задач: `id` и `idReadable`

- Все команды `issue`/`command`, принимающие `<id>`, работают и с ring-id (`2-1`),
  и с человекочитаемым `idReadable` (`PRJ-42`).
- Для path-параметра `{id}` (`issue view/edit/delete`, `issue comment ...`) значение
  передаётся без преобразований — YouTrack резолвит оба формата (подтверждено официальной
  документацией). Обязательно URL-кодирование (`url.PathEscape`).
- Для `yt issue close <id>...` и `yt command ... <issue>...` идентификаторы передаются
  в теле запроса как `{"idReadable": "..."}` или `{"id": "..."}`. Выбор поля — по форме
  значения (детерминированно, форматы не пересекаются):
  - `^[0-9]+-[0-9]+$` (например `2-1`) → ring-id → `{"id": "<значение>"}`;
  - `^[A-Za-z][A-Za-z0-9]*-[0-9]+$` (например `PRJ-42`, `B2B-3`; сравнение без учёта
    регистра) → `idReadable` → `{"idReadable": "<значение>"}`;
  - остальное → ошибка использования (`cannot parse issue id: <value>`, exit 2).
- Неизвестная задача (HTTP 404) → сообщение вида `Issue PRJ-999 not found`, exit 1.

### 4.2. Параметр `fields`

- **Каждый** запрос к API явно передаёт `fields` — только нужные поля (§3).
  Никаких «запросить всё».
- Списки полей задаются константами в `internal/api/fields.go` с комментарием,
  какой команде/выводу они соответствуют.
- Правило поддержки: если рендеру нужна новая колонка/поле — сначала добавляется поле
  в `fields.go` и в структуру ответа в `types.go`, только потом используется в выводе.
- Для `--json` набор `fields` должен покрывать все поля, выводимые в JSON.
- Приоритет полей в `fields`-строке стабильный (единый источник в `fields.go`),
  чтобы вывод был детерминированным (важно для golden-тестов, §5.2).

### 4.3. Формат вывода: TTY / `--json` / pager

**TTY (по умолчанию):**

- Таблицы — `text/tabwriter`; многострочные поля переносятся/обрезаются по ширине терминала.
- Детали задачи (`issue view`) — разделители из символов `─`/`────`.
- ANSI-цвета: только если stdout — TTY и `YT_NO_COLOR != 1`. Минимальная палитра:
  зелёный — успех (`✓`), жёлтый — предупреждения, красный — ошибки в stderr, цвет штатов
  (Open/Fixed) опционален.
- Pager: для «длинного» вывода (`issue view` с описанием > 1 экрана, `comment list` при TTY)
  запускается pager из `$PAGER` (по умолчанию `less -FRX`). Pager не запускается при
  `--json`, `--verbose`, не-TTY stdout, или если `PAGER=cat`.

**`--json`:**

- Вывод — **сырой** JSON-документ с сервера: массив или объект, без обёрток
  («ошибка», «результат») и без ANSI. Совместим с `jq`.
- Все временные метки остаются unix-миллисекундами (`created`, `updated`, `resolved`) —
  как их отдаёт сервер. Человекочитаемые даты — только в TTY.
- Исключения — поля, добавляемые утилитой к ответу сервера — допускаются только если явно
  перечислены в разделе команды (именование camelCase, как у сервера):
  `auth status --json` → `baseUrl` (§3.3); `issue view --comments --json` → `comments` (§3.4).
- `--json` валиден только при exit-коде 0; при ошибке в stderr — сообщение, stdout пуст.

**Требование к stdout/stderr:** данные — только в stdout, служебное — только в stderr.

### 4.4. Обработка ошибок и exit-коды

**Exit-коды:**

| Код | Значение |
|---|---|
| `0` | успех |
| `1` | runtime/API ошибка (HTTP 4xx/5xx, сеть, нет токена) |
| `2` | ошибка использования CLI (неизвестная команда/флаг, невалидные аргументы, `--help` не применим) |
| `130` | отменено пользователем (SIGINT/SIGTERM, см. §4.5) |

**Формат сообщения об ошибке (stderr, один блок):**

```
yt: <краткое сообщение>
      причина из ответа сервера (error / error_description)
```

- HTTP-ошибки: выводится HTTP-код, тело `{"error": "<title>", "error_description": "<detail>"}`
  (формат подтверждён живой проверкой сервера, §6) — например:

  ```
  yt: request failed: 404 Issue PRJ-999 not found
  yt: request failed: 403 You don't have permission to ...
  ```

- Специальные случаи:
  - `401` → `not logged in or token is invalid, run "yt auth login"` (код 1).
  - Сетевые ошибки (connect refused, timeout) → `cannot reach <base-url>: <err>`.
  - Нет токена и не `auth login` → `no token provided: run "yt auth login" or set YT_TOKEN`.
- `$type` из тела ответа выводить не требуется; коды/сообщения из `error_description` — обязательно.
- Никогда не выводить токен в сообщениях и логах.

### 4.5. Таймауты и повторные попытки

- Общий таймаут запроса: 30 с (переопределяется `YT_HTTP_TIMEOUT`).
- Таймаут соединения: 10 с.
- Retry: **только для идемпотентных методов** (GET). До 3 попыток с экспоненциальным
  backoff (500 мс × 2^n) + случайный jitter (±50 мс). Условия повтора: HTTP 429, 5xx,
  сетевые ошибки. На `Retry-After` (при наличии) — приоритет.
- POST/PUT/DELETE не повторяются автоматически (неидемпотентны); единственная попытка.
- Отмена: команда слушает контекст (SIGINT/SIGTERM → отмена запросов, exit 130).

### 4.6. Логирование

- По умолчанию уровень `error`; `--verbose`/`YT_LOG_LEVEL=debug` → в stderr подробный лог
  (URL, метод, статус, длительность; **без** тела ответа и без токена).
- Формат: `2026-07-31T12:00:00Z DBG GET /issues?$top=30 status=200 dur=123ms`.

### 4.7. Безопасность

- Токен хранится в файле с правами `0600`; не логируется; не попадает в `--json`/таблицы.
- В `--json` и help-текстах токен никогда не печатается.
- HTTPS по умолчанию не навязывается (локальный сервер на HTTP), но URL-параметр `--base-url`
  принимается как есть.

### 4.8. Актуализация спецификации API

- Перед началом реализации и при подозрении на расхождения — скачать
  `http://localhost:8080/api/openapi.json` и сверить: пути, имена параметров, схемы из §3.
- В репозитории не хранить слепок спецификации; вместо этого — документально зафиксировать
  в задаче на реализацию дату проверки и расхождения (если были).

---

## 5. Тестирование и качество

### 5.1. Юнит-тесты

- `internal/config`: приоритеты (флаг > env > config > дефолт), чтение/запись файла,
  создание каталогов, права `0600`, `YT_CONFIG_HOME`.
- `internal/api`: сборка URL и query-параметров (в т.ч. `fields`), декодирование ответов,
  маппинг `$type`, парсинг ошибок из тела.
- `internal/commands`: валидация аргументов (обязательные флаги), формирование query
  из флагов `state/project/assignee`, конструирование тел запросов.

### 5.2. Golden-тесты вывода

- Каждая команда рендера: фикстурный ответ → ожидаемый TTY-вывод (файлы в `testdata/`).
- При изменении формата вывода golden-файлы обновляются осознанно (флаг `-update`).
- Отдельно покрываются: пустой список, список с кастомными полями, длинное описание (pager
  в тестах отключён через `PAGER=cat`/заглушку), `--json` для каждой команды.

### 5.3. Тесты API-клиента

- `httptest.Server` (fake YouTrack): проверка метода, пути, query-параметров (`fields`, `$top`),
  заголовка `Authorization: Bearer`, тела запроса.
- Кейсы ошибок: 401/403/404/429/500, сетевой таймаут, retry-логика (счётчик попыток).

### 5.4. Интеграционные тесты

- За флагом/переменной `YT_INTEGRATION=1` — прогон против живого сервера
  (`YT_BASE_URL`, `YT_TOKEN`). Используют реальные данные, но не удаляют их без явного
  разрешения (тесты на delete/create помечены `t.Skip` по умолчанию).
- В CI не запускаются.

### 5.5. Статический анализ и форматирование

- `gofmt` (обязательно, `go fmt ./...`), `go vet ./...` без предупреждений.
- `golangci-lint` (конфиг `.golangci.yml`, линтеры по умолчанию + `errcheck`, `govet`,
  `ineffassign`, `staticcheck`, `unused`, `misspell`).
- `go mod tidy` — чистота `go.mod`/`go.sum`.
- (Опционально) `govulncheck ./...`.

---

## 6. Критерии приемки

1. **Полнота команд.** Реализованы все команды §3 (`auth`, `issue list/view/create/edit/close/delete`,
   `issue comment list/create`, `search`, `search suggest`, `command`, `command assist`,
   `project list`, `user whoami`, `tag list`, `version`), каждая — с указанными флагами и
   поведением (TTY + `--json`).
2. **Привязка к API.** Каждая команда сопоставлена с эндпоинтом и параметрами из
   `openapi.json` (таблицы §3); расхождений со свежей спецификацией нет (сверка по §4.8).
3. **Примеры в документации.** README содержит примеры вызова и примеры JSON-вывода для
   ключевых команд (`issue list`, `issue view`, `issue create`, `command`, `user whoami`),
   совпадающие с фактическим выводом утилиты.
4. **Совместимость с Go 1.24.0.** `go.mod` — `go 1.24.0`; сборка и тесты проходят
   на Go 1.24.0; сторонние зависимости не требуют более нового Go.
5. **Тесты и качество.** `go vet`, `golangci-lint`, юнит- и golden-тесты проходят;
   покрытие `internal/config`, `internal/api` (парсинг ошибок), `internal/output` — не ниже 70%.
6. **Обработка ошибок.** Все ошибочные сценарии §4.4 дают ненулевой exit-код, сообщение
   в stderr с кодом/описанием ошибки API; 401 — с подсказкой `yt auth login`.
7. **Приоритет конфигурации** — флаг > env > config > дефолт (по §3.2).
8. **Никакого мусора.** Токен не логируется; stdout свободен от служебных сообщений;
   `yt auth logout` действительно удаляет токен.
9. **Соответствие §3.10.** Архитектура не препятствует добавлению перспективных команд.

---

## 7. Оценка трудозатрат по этапам

Оценка в человеко-днях (1 инженер, Go 1.24). Диапазон отражает вариативность
(известный/неизвестный сервер, наличие тестовой среды).

| № | Этап | Состав | Оценка |
|---|---|---|---|
| 1 | **Аутентификация и конфигурация** | пакет `config`, `auth login/logout/status`, `user whoami`, проверка токена | 2–3 |
| 2 | **CLI-каркас** | Cobra root, глобальные флаги, пайплайн команд, ошибки/exit-коды, лог, `version`, help | 2–3 |
| 3 | **API-клиент** | HTTP-клиент, retry/backoff, таймауты, типы, `fields`, парсинг ошибок + тесты | 3–4 |
| 4 | **Issue-команды** | list, view, create, edit, close, delete + тесты | 4–5 |
| 5 | **Комментарии, поиск, команды** | comment list/create, search, search suggest, command, command assist | 3–4 |
| 6 | **Проекты и теги** | project list, tag list | 1 |
| 7 | **Формат вывода** | таблицы, `--json`, pager, цвета, golden-тесты | 2–3 |
| 8 | **Полировка** | lint, vet, CI, README с примерами, интеграционные тесты, ревью | 2–3 |
| | **Итого** | | **19–26** |

Рекомендуемый порядок выполнения этапов: **1 → 2 → 3 → 4 → 5 → 6 → 7 → 8** (совпадает с
нумерацией): аутентификация даёт работающий «скелет» уже на этапе 2, а всё issue-ядро —
к концу этапа 4.

Каждый этап — отдельная задача (issue) с критериями приёмки из §5/§6, применимыми к его объёму.

---

## Приложение А. Сводная таблица «команда → эндпоинт → параметры»

Все пути — относительно `{base_url}` (по умолчанию `http://localhost:8080/api`).

| Команда yt | Эндпоинт | Метод | Query-параметры | Поля (fields) |
|---|---|---|---|---|
| `auth login/status`, `user whoami` | `/users/me` | GET | `fields` | `id,login,fullName,email,guest,avatarUrl` (union; наборы по командам — §3.3/§3.8) |
| `issue list` | `/issues` | GET | `query`, `customFields`, `fields`, `$skip`, `$top` | см. §3.4 |
| `issue view` | `/issues/{id}` | GET | `fields` | см. §3.4 |
| `issue create` | `/issues` | POST | `fields`; `draftId`, `muteUpdateNotifications` — есть в спеке, в v1 не используются | `id,idReadable,summary,project(id,shortName)` |
| `issue edit` | `/issues/{id}` | POST | `fields`, `muteUpdateNotifications` | `id,idReadable,summary,description` |
| `issue close` | `/commands` | POST | `fields`; `muteUpdateNotifications` — есть в спеке, в v1 не используется | `issues(id,idReadable,summary,resolved,project(id,shortName))` |
| `issue delete` | `/issues/{id}` | DELETE | — | — |
| `issue comment list` | `/issues/{id}/comments` | GET | `fields`, `$skip`, `$top` | `$type,id,text,created,author(id,login,fullName)` |
| `issue comment create` | `/issues/{id}/comments` | POST | `fields` | `$type,id,text,created,author(id,login)` |
| `search <q>` | `/issues` | GET | `query`, `fields`, `$skip`, `$top` | как `issue list` |
| `search suggest` | `/search/assist` | POST | `fields` | `query,suggestions(option,description,prefix,suffix,group)` |
| `command` | `/commands` | POST | `fields`; `muteUpdateNotifications` — есть в спеке, в v1 не используется | `issues(id,idReadable,summary,resolved,project(id,shortName))` |
| `command assist` | `/commands/assist` | POST | `fields` | `query,suggestions(option,description,prefix,suffix,group)` |
| `project list` | `/admin/projects` | GET | `fields`, `$skip`, `$top` | `id,name,shortName,archived,leader(id,login,fullName)` |
| `tag list` | `/tags` | GET | `query`, `fields`, `$skip`, `$top` | `id,name,untagOnResolve` |

---

## Приложение Б. Глоссарий

| Термин | Значение |
|---|---|
| ring-id / id | внутренний идентификатор сущности YouTrack (строка вида `2-1`) |
| `idReadable` | человекочитаемый идентификатор задачи (`PROJ-42`) |
| permanent token | API-токен YouTrack, формат `perm:<...>`, схема авторизации `permanentToken` (Bearer) |
| `fields` | query-параметр YouTrack для выбора возвращаемых полей (сокращение полезной нагрузки) |
| командный язык YouTrack | синтаксис `state: Fixed, Priority: High` для массовых изменений через `POST /commands` |
| `$type` | дискриминатор полиморфных схем в спецификации YouTrack |
