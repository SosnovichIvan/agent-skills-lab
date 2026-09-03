# Универсальный промпт запуска

Скопируй шаблон и используй нативный синтаксис skills текущего agent CLI.
`execution-state` и профиль реализации должны быть реально доступны; core не
требует `$` либо другого конкретного префикса.

```text
Явно используй <EXECUTION_STATE_INVOCATION> как координатор долгой задачи.
Для предметной реализации используй <IMPLEMENTATION_REF>.

Источник требований: <standalone|openspec>
OpenSpec change ID, если применимо: <CHANGE_ID>
Корень проекта: <PROJECT_ROOT>
Runtime adapter: <auto|manual|ADAPTER_ID>
Custom adapter manifest, если используется: <none|MANIFEST_PATH>
Доверие custom executable после проверки manifest: <no|yes>

Описание задачи:
<TASK_DESCRIPTION>

Ожидаемый проверяемый результат:
<EXPECTED_RESULT>

Критерии завершения:
- <DONE_WHEN_1>
- <DONE_WHEN_2>

Ограничения и границы разрешённых действий:
<CONSTRAINTS>

Сначала выбери passthrough, lite или reset по правилам execution-state.
Passthrough не должен создавать state. Для OpenSpec используй tasks.md как
единственный task ledger и не создавай task JSON. Для reset используй только
подтверждённые capabilities выбранного CLI-adapter; неизвестный или
неоднозначный runtime должен перейти в manual handoff.

Выполняй работу semantic chunks, а не по одному shell-действию. Обновляй state
после законченного результата, проверки или блокера. Не сохраняй историю,
рассуждения, секреты и сырые логи. Перед сменой контекста создай валидный
checkpoint и worker packet в новый файл. Учти, что packet повышает revision и
создаёт единственный worker lease. Передай `runtime-plan` тот же state через
`--id` либо `--state`. Принимай результат только с совпадающими `run_id` и
`based_on_revision`; передай их соответственно в `--run-id` и
`--expected-revision` команды `observe`, `complete` или `block`.

Custom adapter используй только при явном выборе его ID, проверенном manifest и
осознанном `--trust-custom-adapter`: trust разрешает version probe executable,
но не model run и не обход permissions.

Не включай permission bypass, не запускай другой платный agent CLI и не
выполняй commit, push, merge или публикацию без соответствующего разрешения.
В финале укажи результат, проверки, блокеры и путь к checkpoint, если state
создавался.
```

Примеры `<IMPLEMENTATION_REF>`: CLI-native вызов skill, путь к правилам либо
`base-agent`. Значение сохраняется как непрозрачная ссылка и не переписывается
core.
