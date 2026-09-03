# Worker protocol v1

Coordinator — единственный писатель state и OpenSpec `tasks.md`. Worker получает
один bounded semantic chunk, изменяет только разрешённые рабочие артефакты и
возвращает результат, связанный с revision.

## Request

```json
{
  "protocol": "execution-state.worker/v1",
  "packet_version": 1,
  "run_id": "generated-id",
  "state_id": "add-rate-limit",
  "based_on_revision": 8,
  "revision": 8,
  "source": {"kind": "openspec", "change": "add-rate-limit"},
  "goal": "Завершить проверяемый результат change",
  "implementation_ref": "/backend-api",
  "constraints": ["Не менять публичный API"],
  "task": {
    "id": "2.1",
    "title": "Реализовать middleware",
    "done_when": ["Целевая проверка проходит"],
    "source_refs": ["openspec/changes/add-rate-limit/tasks.md#2.1"],
    "working_files": ["src/middleware/rate_limit.py"]
  },
  "next_action": "Реализовать middleware и выполнить узкую проверку",
  "last_observation": "Зависимый контракт уже реализован",
  "limits": {"max_turns": 8, "max_result_bytes": 8192}
}
```

До создания packet state в этом примере имел revision 7. `statectl packet`
повысил её до 8, создал lease и записал 8 в `based_on_revision`; поле
`revision` дублирует то же значение для совместимости. Создание второго packet
для этого state запрещено до приёмки или блокировки текущего результата.

`implementation_ref` — непрозрачная CLI-native ссылка. Worker использует её,
только если соответствующий skill/профиль реально доступен; controller не
интерпретирует префикс.

Packet не содержит историю, завершённые задачи, скрытые рассуждения, полные
логи и vendor/runtime metadata. Worker сам открывает только перечисленные
source refs и рабочие файлы.

## Result

```json
{
  "protocol": "execution-state.result/v1",
  "run_id": "generated-id",
  "based_on_revision": 8,
  "task_id": "2.1",
  "status": "complete",
  "summary": "Middleware реализован",
  "artifacts": [
    {"path": "src/middleware/rate_limit.py", "purpose": "Rate limiting"}
  ],
  "verification": [
    {"command": "project-specific-check", "exit_code": 0, "summary": "pass"}
  ],
  "blockers": [],
  "next_action": null
}
```

Не возвращай chain-of-thought. Если CLI не поддерживает structured output,
последний ответ содержит один JSON между маркерами:

```text
---EXECUTION_STATE_RESULT_V1---
{...}
---END_EXECUTION_STATE_RESULT_V1---
```

## Приём

Coordinator проверяет `protocol`, `run_id`, revision, task ID, изменённые файлы,
`done_when` и evidence. Затем он вызывает высокоуровневую операцию `statectl`.
После packet любая приёмка обязана передать lease-пару:

```text
<STATECTL> complete --id add-rate-limit --project-root . \
  --expected-revision 8 --run-id generated-id \
  --summary "Middleware реализован" --evidence "project-specific-check: pass"
```

Для частичного результата используется `observe`, для проверенного блокера —
`block`; обе команды получают те же `--expected-revision` и `--run-id`.
Успешная операция снимает lease и повышает revision. Несовпадение любого
значения оставляет state и OpenSpec checkbox без изменений.

При невалидном envelope:

- revision и checkbox не меняются;
- изменённые файлы не скрываются и считаются `unverified_partial`;
- разрешена одна попытка восстановить только формат результата;
- затем создаётся recovery chunk или блокер.

Один state допускает только один активный worker lease. Параллельность возможна
лишь через независимые state ID с непересекающимися файлами и контрактами, если
это не расширяет исходный scope; у каждого state остаётся один coordinator.
