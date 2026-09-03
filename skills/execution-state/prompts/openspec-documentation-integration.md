# Промпт интеграции с документацией OpenSpec

Этот промпт один раз добавляет правила запуска в существующий OpenSpec-проект.
Он не выполняет change и не создаёт execution state.

```text
Интегрируй skill execution-state в документацию текущего OpenSpec SDD-проекта.
Используй нативный синтаксис явного вызова skills целевого agent CLI; не
привязывай правила проекта к Codex, Claude или одному провайдеру.

Не запускай execution-state в рамках этой задачи, не создавай
.execution-state и не изменяй код продукта либо артефакты существующих changes.

Изучи openspec/config.yaml, принятую schema, корневые agent instructions и
существующую пользовательскую документацию. Дополняй подходящие места вместо
создания дублей.

Зафиксируй правила:
1. execution-state запускается только явным вызовом пользователя.
2. Для Apply указываются change ID, задача, ожидаемый результат, ограничения,
   CLI-native implementation skill/profile и runtime adapter либо auto.
3. Сначала выбирается passthrough, lite или reset. Короткая связная задача
   использует passthrough без state.
4. proposal/specs/design/tasks.md остаются источником истины. В OpenSpec-lite
   создаётся только компактный overlay; task JSON запрещены.
5. Checkbox меняет deterministic controller только после verification evidence.
6. Reset выполняется между semantic chunks и только через подтверждённую
   capability текущего CLI. Неизвестный runtime использует manual handoff.
7. Agent CLI adapter не меняет смысл задачи, state, permissions или OpenSpec.
8. Создание packet повышает revision и создаёт единственный worker lease.
   Runtime plan всегда связывается с тем же state через ID или путь. Результат
   принимается только с совпадающими run_id и based_on_revision.
9. Custom manifest требует явного выбора и доверия; trust разрешает version
   probe executable, но не model run и не permission bypass.
10. Archive и внешние действия требуют обычного отдельного разрешения.

Добавь пользовательский шаблон:

--- BEGIN EXECUTION PROMPT ---
Явно используй <EXECUTION_STATE_INVOCATION> как координатор.
Используй <IMPLEMENTATION_REF> для предметной реализации.

Источник требований: openspec
OpenSpec change ID: <CHANGE_ID>
Корень проекта: <PROJECT_ROOT>
Runtime adapter: <auto|manual|ADAPTER_ID>
Custom adapter manifest, если используется: <none|MANIFEST_PATH>
Доверие custom executable после проверки manifest: <no|yes>

Задача и ожидаемый результат:
<TASK_AND_RESULT>

Ограничения:
<CONSTRAINTS>

Работай только в рамках утверждённых OpenSpec-артефактов. Выбери профиль по
правилам execution-state, выполняй semantic chunks и оставляй валидный
checkpoint перед reset/handoff. После packet сохрани выданные run_id и revision,
свяжи runtime-plan с тем же state и используй их при приёмке worker result. Не
Archive без отдельного разрешения.
--- END EXECUTION PROMPT ---

После изменений проверь синтаксис конфигурации и внутренние ссылки. В ответе
перечисли изменённые документы и выполненные проверки.
```
