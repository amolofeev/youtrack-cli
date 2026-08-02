# agents.md (yt/)

Проект `yt` — независимый подпроект в поддиректории `yt/` текущего репозитория
(см. корневой `AGENTS.md`, раздел «Подкаталоги как независимые проекты»).
Здесь собственные `go.mod`, `Makefile` и правила; корневой `AGENTS.md` действует
на уровне репозитория в целом.

## Структура проекта

Устройство `yt/` (путь → смысл) описано в `docs/structure.md`. Перед первым поиском по
коду открой его — это быстрее и точнее разведки `glob **`.

Правило поддержки: `docs/structure.md` — single source структуры подпроекта. Обновляй
его ВМЕСТЕ с изменением структуры (новый/переименованный пакет в `internal/`, перенос
файлов, новый генерируемый каталог) — в том же коммите, что и изменение кода.

Память подпроекта — `.opencode/yt_memory.md` (в корне репозитория, gitignored):
читай его в начале работы над yt, обновляй по итогам рефлексии. Дорогие факты
и gotcha — только туда; в репо-память (`memory.md`) контент подпроекта не переноси.

## Локальные правила

- Все команды Go выполняются из каталога `yt/` (здесь свой `go.mod`).
- Модуль: `github.com/amolofeev/prompt-and-pray`; структура `cmd/` + `internal/` — по
  SPEC §2.2, раскладка по пакетам — `docs/structure.md`.
- Инструментарий: путь до Go — `~/sdk/go1.24.0/bin/go` (не в PATH), пользуйся им
  напрямую; gofmt — там же (`~/sdk/go1.24.0/bin/gofmt`); golangci-lint —
  `~/go/bin/golangci-lint` (v1.64+).
- Так как Go нет в PATH, ВСЕ цели Makefile (`build/test/vet/integration`) падают:
  Makefile зовёт `$(GO)`, где `GO ?= go` — резолв из PATH («make: go: No such file
  or directory»). Проверено на #45: `make test`/`make vet` без PATH тоже не работают
  (вопреки старой заметке). Запуск любых make-целей — только с явным PATH:
  `PATH="$HOME/sdk/go1.24.0/bin:$PATH" make <target>`.
  С #42 `make lint` сам находит golangci-lint (переменная `GOLANGCI_LINT`, фолбэк
  `$(HOME)/go/bin/golangci-lint`) — бинарь golangci-lint в PATH не обязателен,
  достаточно PATH с Go.
- Проверка форматирования — только свой код: `~/sdk/go1.24.0/bin/gofmt -l internal cmd`.
- Golden-тесты вывода (Атом 7.3, #41): golden-файлы — `yt/testdata/*.golden`
  (commitятся, корневой .gitignore их не игнорирует), регенерация —
  `go test ./internal/commands -run TestGolden -update` из `yt/`. При изменении
  формата вывода команды (рендеры в `internal/commands`) golden-файлы обновлять
  в ТОМ ЖЕ коммите и просматривать их diff — это часть осознанного изменения
  (SPEC §5.2). Перед первым `-update` в свежем клоне создать каталог `testdata/`
  (`os.WriteFile` не создаёт родителей).
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
  `make test` по умолчанию гоняет `go test -cover -bench=. -count=1 -v ./...` — флаги через
  переопределяемую переменную `TESTFLAGS ?= ...` (`make test TESTFLAGS="-run X"`), #52.
- Версия берётся из корневого `VERSION` (`../VERSION`) и встраивается через `-ldflags`
  в переменные `internal/version.{Version,Commit,Built}`; при сборке без ldflags — `unknown`.
- Объём и критерии приёмки — docs/SPEC.md (rev 1.1.1) и задачи GitHub (метка `atomic`).
- Сверка со спекой (§4.8/DoR): если живой сервер недоступен — использовать снапшот
  `.opencode/openapi.json` (сверить версию API = 2025.3; файл gitignored, статичный —
  перекачивать заново только при подозрении на смену версии API), зафиксировать
  отклонение в комментарии задачи; финальная сверка — Атом 8.4.
- Если для задачи нужно поднять YouTrack (`localhost:8080`) или установить ПО —
  спросить человека, не делать автономно.
- Верификация поведения (смоук/интеграция) — ТОЛЬКО против локального реального
  сервера YouTrack (`localhost:8080`); тестовые/мок-серверы НЕ создавать.
  Аутентификацию для смоука проверяй САМОЙ утилитой: `bin/yt auth status`
  (живой GET /users/me, читает сохранённый конфиг `~/.config/yt/config.yml`).
  Файл конфига напрямую не открывать — в нём токен; о конфиге судить только
  по выводу утилиты.
  Токен у человека / зондирование окружения — только если утилита сообщает об
  отсутствии валидной аутентификации («✗ not logged in», «no token provided»);
  не выдумывать токен, не хардкодить.
  При расхождении («decode response» и т.п.) сначала проверяй сам запрос и
  ответ сервера (путь без query, отдаваемое тело), а не дебагь CLI.
  Перед смоуком после правок кода пересобирай `bin/yt`
  (`PATH="$HOME/sdk/go1.24.0/bin:$PATH" make build`): stale-бинарь выдаёт
  вводящие в заблуждение ошибки («unknown flag», «unknown command»).
  TTY-зависимое поведение (pager, ANSI-цвета) проверяй под настоящим pty:
  stdout bash-инструмента — не терминал, поэтому pager не запустится, а цвета
  выключатся. Обёртка: `script -qec '<cmd>' /dev/null` (выделяет pty).
  Для pager-а задавай `PAGER='<stub> <out>'` и сверяй, что контент ушёл
  в файл stub-а, а не в pty.

## Интеграционные тесты (Атом 8.3, #44)

- Интеграционные тесты — `yt/internal/commands/integration_test.go` (SPEC §5.4).
  Запуск: `YT_INTEGRATION=1 make integration` из `yt/` (с PATH на Go); в CI
  не запускаются. Read-only тесты (auth status, whoami, list/view, search,
  suggest, assist, project/tag list) идут только с `YT_INTEGRATION=1`.
- Мутирующие тесты (create/edit/close/command/comment/delete) требуют явного
  разрешения `YT_INTEGRATION_MUTATE=1` — иначе `t.Skip` (SPEC §5.4: create/delete
  по умолчанию skip). Каждый мутирующий тест создаёт смоук-ишью с уникальным
  summary и удаляет её в `t.Cleanup` — не оставлять задачи на сервере.
  Для прогона нужен `YT_TOKEN` (брать у человека/окружения, config.yml напрямую
  не открывать без разрешения владельца).
- Мишень — локальный реальный сервер (localhost:8080); проект и разрешающее
  состояние для create/command берутся с сервера динамически (project list,
  command assist), а не хардкодятся (в DEMO разрешающее состояние — Done,
  не Fixed — дефолт `state: Fixed` даёт 400).
- Атомарность `/commands` (SPEC §3.4): команда с несуществующей задачей в
  batch → HTTP 400, изменения к валидным задачам НЕ применяются (тест
  TestIntegrationCommandAtomicity; сервер 2025.3 отклоняет весь запрос).
