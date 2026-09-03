# Промпт architecture/integration review

Используй этот текст, когда controller установил `quality.review_required` или
`pending_bridge`. Prompt не встроен в `statectl` и подходит любому agent CLI.

```text
Явно используй <EXECUTION_STATE_INVOCATION> для review текущего state
<STATE_PATH>. Не реализуй следующую feature до завершения review.

Проверь только сохранённые artifacts, context map, global invariants, публичные
contracts и изменения завершённых chunks. Оцени:
- корректность wiring и dependency direction;
- отсутствие дублирующих моделей/реализаций одного контракта;
- согласованность error, auth, tenant и idempotency semantics;
- чрезмерно выросшие файлы или модули;
- наличие сквозной проверки для cross-area изменения.

Если pending_bridge=true, следующий рабочий chunk обязан иметь
kind=integration и проверять связь affected areas. Не заменяй его локальным
unit/static check.

Верни краткий summary, конкретный evidence и при необходимости список
исправляющих chunks. Не возвращай chain-of-thought или сырые логи. Coordinator
передаёт принятый summary/evidence в `statectl architecture-review`; новые
обязательные исправления сначала добавляет в исходный task ledger.
```
