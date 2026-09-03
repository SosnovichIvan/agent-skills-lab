---
name: execution-state
description: "Координирует долгие агентные задачи через компактное состояние, semantic chunks, checkpoints и смену контекста; работает с OpenSpec или самостоятельно в любом agent CLI. Применять только при явном вызове в синтаксисе текущего CLI."
---

# Выполнение через состояние

Используй execution state как переносимую рабочую память между смысловыми
частями долгой задачи. Core не зависит от Codex, Claude или другого агента:
текущий runtime выбирается по возможностям через CLI-adapter.

## Входные условия

- Skill должен быть явно вызван в синтаксисе текущего агента.
- Пользователь должен задать задачу, проверяемый результат и границы действий.
- Ссылка на skill или профиль реализации передаётся как непрозрачное значение;
  это может быть `$name`, `/name`, путь к правилам или `base-agent`.
- Сам skill не расширяет полномочия и не включает обход permissions.

Пользовательские шаблоны в `prompts/` отделены от scripts. Во время обычного
выполнения не читай prompt templates и исходники helpers: запускающий запрос уже
содержит нужные значения. Исходники helper читай только для диагностики ошибки.

## Маршрутизация

Сначала оцени число semantic chunks, ожидаемый handoff и объём одноразовой
истории. Источник и профиль выполнения — независимые оси:

- `source`: `standalone` или `openspec`;
- `execution.profile`: `lite` или `reset`;
- `passthrough` не создаёт state вообще.

Для очевидно короткой связной задачи без handoff выбери `passthrough` и сразу
используй skill реализации. На этом workflow execution-state заканчивается:
не читай его references, не запускай helpers и не создавай state. В OpenSpec
продолжай обычный Apply по его источникам истины. В сомнительном случае
используй `statectl route`. Не обещай экономию токенов в `lite`: этот профиль
даёт только checkpoint.

Только если для OpenSpec Apply после маршрутизации выбран `lite` или `reset`,
прочитай [правила OpenSpec-lite](references/openspec-integration.md). Для
`reset` дополнительно прочитай
[контракт runtime](references/runtime-contract.md). Остальные references
открывай только когда соответствующая операция действительно нужна.

## Единственный helper

Используй только `scripts/statectl.py`, найденный относительно этого `SKILL.md`.
Запускай его доступным Python 3 interpreter; не предполагай путь проекта или имя
конкретного agent CLI. Не вызывай все подкоманды с `--help`.

Основные операции:

```text
statectl route
statectl init
statectl begin
statectl observe
statectl complete | statectl block
statectl checkpoint
statectl packet --expected-revision <N> --run-id <ID> --output <file>
statectl validate
statectl runtime-probe | statectl runtime-plan
```

Точный интерфейс и компактная schema описаны в
[state-schema.md](references/state-schema.md). Helper сам проверяет revision,
переходы и лимиты; не создавай ручные JSON Patch-файлы и не запускай отдельную
валидацию после каждой успешной операции.

## Рабочий цикл

1. Создай или возобнови state только после выбора `lite` либо `reset`.
2. Выполняй один semantic chunk: законченный результат вместе с его узкой
   проверкой, обычно несколько внутренних tool/model шагов.
3. Обнови state один раз после значимого результата, блокера или смены
   подсистемы. Не записывай чтения файлов и внутренние рассуждения.
4. Завершай chunk только с verification evidence; OpenSpec checkbox меняет
   deterministic core, а не worker.
5. Перед сменой контекста создай валидный checkpoint. Затем один раз вызови
   `packet`: эта операция повышает revision и создаёт `worker_lease`, связанный
   с `run_id`. Сохрани `run_id` и новую revision из ответа helper.
6. Построй `runtime-plan`, обязательно привязав его к тому же state через
   `--id` либо `--state`; один packet без state недостаточен.
7. Выбери стратегию только по подтверждённым capabilities runtime:
   fresh process, управляемая compaction, manual handoff или `lite` fallback.
8. Новый worker получает packet, релевантные source refs и текущие файлы, но не
   историю разговора, предыдущие рассуждения или сырые логи.
9. Принимая ответ worker, вызови `observe`, `complete` или `block` с
   `--expected-revision`, равной `based_on_revision` packet, и с тем же
   `--run-id`. Это снимает lease; без совпадения state не меняется.

Fresh process запускай через adapter как argv-массив без shell и только в рамках
исходных permissions. Если adapter неизвестен или выбор неоднозначен, используй
manual handoff; не угадывай флаги CLI. Подробнее:
[cli-adapters.md](references/cli-adapters.md).

Custom manifest не доверяй неявно. Флаг `--trust-custom-adapter` допустим только
после явного выбора и проверки manifest: он разрешает запуск указанного
executable с `--version`, хотя сам `runtime-plan` model request не выполняет.

## Инварианты компактности

- `state.json` — не более 8 KiB, worker packet — не более 12 KiB.
- Последнее наблюдение — не более 2 KiB; полный вывод остаётся в отдельном логе.
- OpenSpec `tasks.md` остаётся единственным task ledger; task JSON не создаются.
- В worker packet нет завершённых задач, истории и runtime/vendor metadata.
- Один coordinator изменяет state; worker возвращает revision-bound result по
  [worker protocol](references/worker-protocol.md).
- Для одного state одновременно существует не более одного `worker_lease`;
  повторный packet до приёмки результата запрещён.
- Секреты, access tokens, пароли и персональные данные в state не записываются.
