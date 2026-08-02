# docs/PROGRESS.md — трекер прогресса по задачам youtrack-cli

Статусы: `✅ закрыт` — задача выполнена (DoD) и закрыта; `🟡 открыт` — в работе/ждёт
закрытия. Декомпозиция и зависимости — [docs/PLAN.md](PLAN.md), обязательные
требования — [docs/SPEC.md](SPEC.md) (rev 1.1.1), процессные правила (DoR/DoD) —
[AGENTS.md](../AGENTS.md).

История подпроекта `yt` эпохи монорепо (задачи #5–#52 репозитория
[prompt-and-pray](https://github.com/amolofeev/prompt-and-pray)) — архив в
`docs/PROGRESS.md` того репозитория; сюда ведётся трекер задач этого репозитория.

Обновляется после закрытия каждого этапа/атома: статус, новые решения/отклонения,
находки интеграционных проверок, дата сверки с `openapi.json` (§4.8).

**Последнее обновление:** 2026-08-02 (миграция из prompt-and-pray завершена, #53).

## Этап M5. Интеграция и смоук (#58, #67)

- **2026-08-02:** интеграционные тесты против живого сервера
  `http://localhost:8080` (YouTrack API 2025.3) — все зелёные:
  read-only — `auth status`, `whoami`, `issue list/view`, `comment list`,
  `search`, `suggest`, `command assist`, `project list`, `tag list`;
  мутирующие (`YT_INTEGRATION_MUTATE=1`) — `create` (резолв проекта по
  shortName/name/ring-id), `edit`, `command` (batch + атомарность), `comment create`,
  `delete` (смоук-ишью удаляются в `t.Cleanup`).
- Смоук CLI: `yt auth status` — `✓ Authenticated` (admin); `yt version` —
  `0.0.1-pre-alpha`. В CI интеграционные тесты не запускаются (SPEC §5.4).

