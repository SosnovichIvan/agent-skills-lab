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

Глобальные инварианты качества, которые нельзя нарушить ни в одном шаге:
- <INVARIANT_1>
- <INVARIANT_2>

Сначала выбери passthrough, lite или reset по правилам execution-state.
Passthrough не должен создавать state. Для OpenSpec используй tasks.md как
единственный task ledger и не создавай task JSON. Для reset используй только
подтверждённые capabilities выбранного CLI-adapter; неизвестный или
неоднозначный runtime должен перейти в manual handoff.

До реализации составь semantic chunks. Для каждого сложного шага укажи kind,
cohesion_key, affected_areas, ограниченные reads/writes, contracts,
regression_checks и requires_bridge. После cross-area изменения сразу планируй
kind=integration. Не создавай chunks для чтения, рассуждения или одной команды.

Выполняй работу semantic chunks, а не по одному shell-действию. Обновляй state
после законченного результата, проверки или блокера. Не сохраняй историю,
рассуждения, секреты и сырые логи. Обновляй context map только компактными
путями, назначениями, areas и symbols. Завершай chunk лишь после passed status
для каждого объявленного regression check. Выполняй обязательный architecture
review и integration bridge до следующей обычной реализации.

Перед сменой контекста создай валидный
checkpoint и worker packet в новый файл. Учти, что packet повышает revision и
создаёт единственный worker lease. Передай `runtime-plan` тот же state через
`--id` либо `--state`. Принимай результат только с совпадающими `run_id` и
`based_on_revision`; передай их соответственно в `--run-id` и
`--expected-revision` команды `observe`, `complete` или `block`.

Следуй `quality.next_handoff`: продолжай тот же context для одинакового
cohesion_key, используй reset при смене ключа и подтверждённой capability,
иначе оставляй checkpoint/manual handoff. Не вызывай новый worker для каждого
микрошагa.

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
