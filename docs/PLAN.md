# План реализации CLI `yt` — декомпозиция issue #5

Версия: **1.0** · Дата: 2026-07-31 · Статус: **утверждён**

Документ является источником истины по декомпозиции 8 этапов (#7–#14) на атомарные
задачи (метка `atomic`). Обязательные требования — в [docs/SPEC.md](SPEC.md) (rev 1.1),
процессные правила (DoR/DoD) — в [AGENTS.md](../AGENTS.md).

> **Отклонение от SPEC §7 (согласовано с владельцем 2026-07-31).** SPEC рекомендует
> порядок этапов 1→2→…→8, но у этапов 1–3 есть круговая зависимость (auth требует
> каркас и API-клиент). Ниже зафиксирован **топологический** порядок выполнения по фазам.
> Нумерация этапов и состав атомов при этом не меняются — меняется только очерёдность
> их исполнения. Сама SPEC не правится до отдельного решения владельца.

---

## DoR / DoD

Каждая атомарная задача считается **готовой к работе (DoR)**, когда:

1. Её `blocked-by`-зависимости закрыты (связи выставлены в GitHub).
2. Объём однозначен: разделы SPEC, эндпоинты, флаги, поля `fields`, TTY/JSON-поведение.
3. Оценка присутствует (в теле задачи).
4. Актуальность эндпоинтов по живому `openapi.json` не вызывает сомнений
   (финальная сверка — Атом 8.4, промежуточные отклонения фиксируются в комментарии).

Задача считается **выполненной (DoD)**, когда выполнены все пункты её DoD-чеклиста
(шаблон в теле атома): реализация по SPEC, тесты (юнит/httptest/golden, покрытие ≥ 70%
для config/api/output), `go fmt`/`go vet`/`golangci-lint`/`go build`, чистота
stdout/stderr, отсутствие токена в логах/выводе, коммит `[AI]` + push, закрытие
с подтверждающим комментарием.

---

## Фазы и порядок выполнения

| Фаза | Состав (атомы) | Смысл |
|---|---|---|
| **A. Фундамент** | 2.1, 1.1, 3.1, 3.2, 7.1 | скелет репозитория, config, транспорт API, типы/fields, вывод. Всё независимо, можно параллелить |
| **B. Каркас и API** | 2.2, 2.3, 3.3, 3.4 | version, Cobra-root, обёртки чтения API, httptest-база |
| **C. Пайплайн и auth** | 2.4, 1.2 | обработка ошибок/exit-коды/лог, команды auth + whoami |
| **D. Команды** | 4.1–4.6, 5.1–5.5, 6.1–6.2 | issue-ядро, комментарии, поиск, командный язык, проекты, теги |
| **E. Вывод и полировка** | 7.2, 7.3, 8.1–8.4 | pager, golden-тесты, статанализ, CI, интеграция, README |

Порядок внутри фазы: сверху вниз по таблицам этапов. Связи `blocked-by` выставлены
в GitHub — они являются источником истины о готовности.

---

## Этап 1 — Аутентификация и конфигурация (#7)

| Атом | Задача | Спец | Зависимости | Оценка |
|---|---|---|---|---|
| 1.1 | [#16](https://github.com/amolofeev/prompt-and-pray/issues/16) Пакет `internal/config` | §2.2, §3.2 | — | 1 |
| 1.2 | [#26](https://github.com/amolofeev/prompt-and-pray/issues/26) `auth login/logout/status`, `user whoami` | §3.3, §3.8 | 1.1, 2.4, 3.3 | 1.5 |
| | **Итого этап 1** | | | **2.5** (SPEC: 2–3) |

## Этап 2 — CLI-каркас (#9)

| Атом | Задача | Спец | Зависимости | Оценка |
|---|---|---|---|---|
| 2.1 | [#17](https://github.com/amolofeev/prompt-and-pray/issues/17) Bootstrap: go.mod, main.go, version, Makefile | §1.4, §2.2, §2.6 | — | 0.5–1 |
| 2.2 | [#21](https://github.com/amolofeev/prompt-and-pray/issues/21) `yt version` + `--version` | §3.11 | 2.1 | 0.5 |
| 2.3 | [#22](https://github.com/amolofeev/prompt-and-pray/issues/22) Cobra root, глобальные флаги, help-группы, completion | §2.3, §3.1 | 2.1 | 1 |
| 2.4 | [#25](https://github.com/amolofeev/prompt-and-pray/issues/25) Пайплайн, ошибки/exit-коды, логирование | §2.1, §4.4–4.6 | 2.3, 1.1, 3.1 | 1 |
| | **Итого этап 2** | | | **3–3.5** (SPEC: 2–3) |

## Этап 3 — API-клиент (#8)

| Атом | Задача | Спец | Зависимости | Оценка |
|---|---|---|---|---|
| 3.1 | [#18](https://github.com/amolofeev/prompt-and-pray/issues/18) Клиент core: transport, таймауты, retry, ошибки | §2.4, §4.1, §4.5 | — | 1–1.5 |
| 3.2 | [#19](https://github.com/amolofeev/prompt-and-pray/issues/19) `types.go` + `fields.go` | §2.4, §4.2 | — | 1 |
| 3.3 | [#23](https://github.com/amolofeev/prompt-and-pray/issues/23) Обёртки чтения API | §3, Прил. А | 3.1, 3.2 | 1 |
| 3.4 | [#24](https://github.com/amolofeev/prompt-and-pray/issues/24) Тесты на `httptest.Server` | §5.3 | 3.1, 3.2, 3.3 | 1 |
| | **Итого этап 3** | | | **4–4.5** (SPEC: 3–4) |

## Этап 4 — Issue-команды (#10)

| Атом | Задача | Спец | Зависимости | Оценка |
|---|---|---|---|---|
| 4.1 | [#27](https://github.com/amolofeev/prompt-and-pray/issues/27) `yt issue list` | §3.4 | 2.4, 3.3, 7.1 | 1 |
| 4.2 | [#28](https://github.com/amolofeev/prompt-and-pray/issues/28) `yt issue view` | §3.4 | 2.4, 3.3, 7.1 | 1 |
| 4.3 | [#29](https://github.com/amolofeev/prompt-and-pray/issues/29) `yt issue create` | §3.4 | 2.4, 3.3, 7.1 | 1–1.5 |
| 4.4 | [#30](https://github.com/amolofeev/prompt-and-pray/issues/30) `yt issue edit` | §3.4 | 2.4, 3.1, 7.1 | 0.5–1 |
| 4.5 | [#31](https://github.com/amolofeev/prompt-and-pray/issues/31) `yt issue close` | §3.4, §4.1 | 2.4, 3.1, 7.1 | 1 |
| 4.6 | [#32](https://github.com/amolofeev/prompt-and-pray/issues/32) `yt issue delete` | §3.4 | 2.4, 3.1, 7.1 | 0.5 |
| | **Итого этап 4** | | | **5–5.5** (SPEC: 4–5) |

## Этап 5 — Комментарии, поиск, командный язык (#11)

| Атом | Задача | Спец | Зависимости | Оценка |
|---|---|---|---|---|
| 5.1 | [#33](https://github.com/amolofeev/prompt-and-pray/issues/33) `yt issue comment list/create` | §3.4 | 2.4, 3.3, 7.1 | 1 |
| 5.2 | [#37](https://github.com/amolofeev/prompt-and-pray/issues/37) `yt search` | §3.5 | 4.1 | 0.5 |
| 5.3 | [#34](https://github.com/amolofeev/prompt-and-pray/issues/34) `yt search suggest` | §3.5 | 2.4, 3.1, 7.1 | 0.5–1 |
| 5.4 | [#35](https://github.com/amolofeev/prompt-and-pray/issues/35) `yt command` | §3.6, §4.1 | 4.5, 2.4, 7.1 | 1 |
| 5.5 | [#36](https://github.com/amolofeev/prompt-and-pray/issues/36) `yt command assist` | §3.6 | 2.4, 3.1, 7.1 | 0.5 |
| | **Итого этап 5** | | | **3.5–4** (SPEC: 3–4) |

## Этап 6 — Проекты и теги (#12)

| Атом | Задача | Спец | Зависимости | Оценка |
|---|---|---|---|---|
| 6.1 | [#38](https://github.com/amolofeev/prompt-and-pray/issues/38) `yt project list` | §3.7 | 2.4, 3.3, 7.1 | 0.5 |
| 6.2 | [#39](https://github.com/amolofeev/prompt-and-pray/issues/39) `yt tag list` | §3.9 | 2.4, 3.3, 7.1 | 0.5 |
| | **Итого этап 6** | | | **1** (SPEC: 1) |

## Этап 7 — Формат вывода (#13)

| Атом | Задача | Спец | Зависимости | Оценка |
|---|---|---|---|---|
| 7.1 | [#20](https://github.com/amolofeev/prompt-and-pray/issues/20) Пакет `internal/output` | §2.2, §4.3 | — (Фаза A) | 1–1.5 |
| 7.2 | [#40](https://github.com/amolofeev/prompt-and-pray/issues/40) Pager | §4.3 | 7.1, 4.2 | 0.5–1 |
| 7.3 | [#41](https://github.com/amolofeev/prompt-and-pray/issues/41) Golden-тесты вывода | §5.2 | 7.1 + все командные атомы | 1 |
| | **Итого этап 7** | | | **2.5–3.5** (SPEC: 2–3) |

## Этап 8 — Полировка (#14)

| Атом | Задача | Спец | Зависимости | Оценка |
|---|---|---|---|---|
| 8.1 | [#42](https://github.com/amolofeev/prompt-and-pray/issues/42) Статанализ: fmt/vet/lint/tidy | §5.5 | все атомы 1–7 | 0.5 |
| 8.2 | [#43](https://github.com/amolofeev/prompt-and-pray/issues/43) CI (GitHub Actions) | §2.6 | 8.1 | 0.5 |
| 8.3 | [#44](https://github.com/amolofeev/prompt-and-pray/issues/44) Интеграционные тесты | §5.4 | командные атомы, 7.3 | 1–1.5 |
| 8.4 | [#45](https://github.com/amolofeev/prompt-and-pray/issues/45) README + сверка с API | §4.8, §6 | 8.3, 7.3 | 0.5–1 |
| | **Итого этап 8** | | | **2.5–3.5** (SPEC: 2–3) |

---

## Итого

| | Оценка |
|---|---|
| Сумма по атомам | ~23.5–28 чел.-дней |
| SPEC §7 (этапы) | 19–26 чел.-дней |

Превышение верхней границы объясняется явным учётом тестов (httptest, golden) в каждом
атоме. При необходимости этапы 7–8 (вывод/полировка) можно выполнять параллельно с
командными этапами.

---

## Meta-задачи (доработка агента)

Задачи на доработку процессов/инструментов агента (метка `meta`) создаются по итогам
ревизии сабтасков и ведутся параллельно с реализацией:

- [#46](https://github.com/amolofeev/prompt-and-pray/issues/46) — workflow безопасных
  интеграционных тестов против живого YouTrack (AGENTS.md).
- [#47](https://github.com/amolofeev/prompt-and-pray/issues/47) — трекер прогресса
  `docs/PROGRESS.md` (статусы этапов, решения, находки).
