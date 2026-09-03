# Worker prompt новой сессии

Этот текст передаётся fresh context вместе с packet-файлом. До построения launch
plan координатор или доверенный wrapper создаёт отдельную конкретизированную
копию и подставляет путь и CLI-native invocation. Adapter не интерпретирует
шаблон, а `statectl` не хранит prompt внутри Python-кода.

```text
Явно используй <EXECUTION_STATE_INVOCATION> и доступный профиль реализации из
worker packet.

Ты выполняешь ровно один semantic chunk. Источник задачи:
<PACKET_PATH>

Прочитай packet, проверь protocol, state_id, run_id и based_on_revision, затем
открой только указанные source_refs и working_files. Не запрашивай и не
восстанавливай предыдущий transcript. Не изменяй execution state, task ledger
или OpenSpec checkbox: это делает coordinator после приёмки результата.

Работай в пределах goal, constraints и task.done_when. Выполни узкую проверку.
Верни только result envelope execution-state.result/v1 по worker protocol.
Не включай chain-of-thought, полные логи, секреты и лишнее описание.
Скопируй `run_id`, `based_on_revision` и `task.id` из packet без изменений: по
ним coordinator проверяет активный lease и принимает результат.

Если задача заблокирована, верни проверенные факты, точный blocker и одно
предлагаемое next_action. Не помечай результат complete без evidence.
```
