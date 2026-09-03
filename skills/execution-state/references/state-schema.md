# Compact state schema v3

`statectl` — единственная публичная точка управления. Все команды печатают одну
короткую JSON-строку; packet записывается только в файл.

## Размещение

```text
.execution-state/<id>/
├── state.json
└── tasks.json       # только standalone; не входит в worker packet
```

В OpenSpec `tasks.json` отсутствует: task ledger уже находится в `tasks.md`.
Prompt templates и runtime adapter manifests не копируются в state.

## `state.json`

```json
{
  "schema_version": 3,
  "id": "add-rate-limit",
  "revision": 4,
  "source": {
    "kind": "openspec",
    "change": "add-rate-limit",
    "tasks_path": "openspec/changes/add-rate-limit/tasks.md"
  },
  "execution": {
    "profile": "reset",
    "adapter": "codex",
    "capabilities": {
      "fresh_context": true,
      "in_place_compaction": false,
      "machine_output": true,
      "session_persistence_control": true,
      "usage_metrics": true
    }
  },
  "implementation_ref": "/backend-api",
  "goal": "Завершить проверяемый OpenSpec change",
  "status": "in_progress",
  "constraints": ["Не менять публичный API"],
  "active_task": {
    "id": "2.1",
    "title": "Реализовать middleware",
    "status": "in_progress",
    "done_when": ["Реализовать middleware"],
    "evidence": [],
    "artifacts": ["src/middleware/rate_limit.py"]
  },
  "next_action": "Выполнить узкую проверку",
  "observation": "Код изменён, проверка ещё не выполнена",
  "blocker": null,
  "artifacts": ["src/middleware/rate_limit.py"],
  "checkpoint": {
    "ready": false,
    "reason": "new observation",
    "revision": 4
  },
  "worker_lease": null
}
```

`implementation_ref` — непрозрачная ссылка в синтаксисе текущего CLI либо
`base-agent`; core не требует конкретного префикса. Adapter/capabilities —
ненормативный runtime snapshot и не входят в worker packet. При смене CLI их
нужно определить заново.

Статусы state: `planned`, `in_progress`, `blocked`, `complete`. Active task
имеет статус `in_progress` или `blocked`. `begin`, `observe`, `complete`, `block`
и `checkpoint` требуют `--expected-revision`; для безопасного handoff всегда
передавай его и в `packet`. Конфликт не меняет файлы.

## Worker lease и revision

`packet` — не read-only export. При успешном вызове helper создаёт новый
packet-файл, повышает state revision на 1 и записывает:

```json
{
  "worker_lease": {
    "run_id": "auth-worker-1",
    "task_id": "1",
    "based_on_revision": 1
  }
}
```

`based_on_revision` packet равен уже повышенной revision и должен без изменений
вернуться в worker result. Пока lease активен, второй `packet`, `begin` и
`checkpoint` отклоняются. Результат worker принимается через `observe`,
`complete` или `block` только с совпадающими `--run-id` и
`--expected-revision`; успешная операция снимает lease и снова повышает
revision.

## Standalone task index

```json
{
  "schema_version": 1,
  "tasks": [
    {
      "id": "auth-token",
      "title": "Реализовать token validation",
      "status": "pending",
      "done_when": ["Целевая проверка проходит"]
    }
  ]
}
```

Завершённые summaries/evidence могут оставаться здесь для аудита, но
`statectl packet` передаёт только активную задачу.

## Основные команды

`<STATECTL>` означает `statectl.py`, запущенный доступным Python 3 interpreter.

```text
<STATECTL> route --source standalone --expected-turns 4

<STATECTL> init --id auth --project-root . --source standalone \
  --profile lite --implementation-ref base-agent --goal "Готовый сервис" \
  --task-id 1 --task-title "Реализовать сервис" --done-when "Проверка проходит"

<STATECTL> init --id change-id --project-root . --source openspec \
  --profile reset --adapter <RESET_CAPABLE_ADAPTER> \
  --implementation-ref /backend-api --goal "Apply change" \
  --change change-id --tasks-path openspec/changes/change-id/tasks.md

<STATECTL> packet --id auth --project-root . --expected-revision 0 \
  --run-id auth-worker-1 --output worker-request.json

<STATECTL> runtime-plan --id auth --project-root . --adapter manual \
  --prompt <CONCRETE_WORKER_PROMPT> --packet worker-request.json

<STATECTL> complete --id auth --project-root . --expected-revision 1 \
  --run-id auth-worker-1 --summary "Chunk готов" \
  --evidence "project check: pass"

<STATECTL> validate --id auth --project-root .
```

`runtime-plan` всегда требует state binding: укажи `--id` вместе с
`--project-root` либо `--state` вместе с соответствующим `--project-root`. Он
сверяет packet с активным lease, revision и содержимым state. Для custom adapter
дополнительно нужны `--adapter <ID>`, `--manifest <PATH>` и
`--trust-custom-adapter`; trust разрешает его executable только для version
probe, а `runtime-plan` agent не запускает.

Для нескольких standalone-задач передай `--tasks-file` с объектом
`{"tasks":[...]}` или повторяй `--task-json`. Не печатай содержимое task index
и packet в model context без необходимости.

## Лимиты и запрещённые данные

- `state.json` — 8 KiB;
- packet — 12 KiB;
- observation/blocker — 2 KiB;
- evidence текущего chunk — 4 KiB суммарно;
- raw logs, transcript, messages, history и reasoning запрещены;
- секреты и персональные данные не должны попадать ни в одно поле.

Профиль `reset` валиден только с подтверждённой `fresh_context` либо
`in_place_compaction`. `lite` не утверждает, что текущая история очищена.
