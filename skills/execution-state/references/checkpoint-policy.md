# Checkpoint policy

Checkpoint — минимальное валидное состояние, из которого новый agent context
продолжит работу без transcript.

## Когда завершать semantic chunk

Сохрани checkpoint, когда:

- получен законченный проверяемый результат;
- появился блокер или требуется permission/решение пользователя;
- следующая работа относится к другой подсистеме или набору файлов;
- накопленный вывод больше не нужен, но будет повторяться в следующих calls;
- достигнут лимит turns, времени либо output текущего worker.

Не создавай checkpoint после каждого чтения, команды или внутренней гипотезы.
Не разрывай незавершённую миграцию, интерактивную операцию либо реализацию до
узкой проверки, если это оставит проект в неопределённом состоянии.

## Инвариант reset-ready

Перед `checkpoint.ready=true` должны выполняться все условия:

1. State проходит `statectl validate` и укладывается в лимит.
2. Записано последнее полезное наблюдение, но не сырой лог.
3. `next_action` содержит одно конкретное действие.
4. Рабочие артефакты доступны в project root и перечислены в active task.
5. Completion подтверждён evidence; иначе задача остаётся `in_progress` или
   `blocked`.
6. Блокер точно указывает недостающий ввод и продолжение после разблокировки.
7. OpenSpec checkbox синхронизирован deterministic controller.
8. Все объявленные `regression_checks` имеют structured status `passed`, ни
   один переданный check не имеет status `failed`.
9. Если `quality.review_required=true`, записан отдельный
   `architecture-review`; если `pending_bridge=true`, следующая задача имеет
   `kind=integration`.

`checkpoint` сам повышает revision. Последующий `packet` является ещё одной
изменяющей операцией: он снова повышает revision, оставляет checkpoint ready и
создаёт `worker_lease` для конкретных `run_id`, task ID и новой revision.
Именно эту post-packet revision worker возвращает как `based_on_revision`.
Пока lease активен, нельзя создать второй packet, выполнить `begin` или новый
`checkpoint`.

## Выбор перехода

После checkpoint:

- `continue` выбирай для соседнего chunk с тем же `cohesion_key`, если нет
  architecture/bridge gate и полезный локальный контекст ещё актуален;
- `reset` выбирай при смене `cohesion_key`, если runtime подтвердил fresh
  context или управляемую compaction;
- `checkpoint` означает manual handoff либо lite-продолжение, когда reset
  capability отсутствует или сначала нужен quality gate;
- продолжай текущий context, если следующий шаг зависит от незаписанного
  локального вывода: сначала доведи текущий semantic chunk до устойчивой
  границы, а не переноси скрытую зависимость;
- используй `native-compact` только при подтверждённой управляемой capability;
- используй `fresh-process`, когда adapter гарантирует новый context;
- иначе создай manual handoff packet;
- `embedded-lite` не считается очисткой контекста.

Рекомендация хранится в `quality.next_handoff` и не запускает CLI сама. Так core
остаётся универсальным: конкретный Codex, Claude, Gemini или custom adapter
преобразует решение в доступную runtime-операцию.

После смены CLI или версии runtime capability negotiation выполняется снова.
Resume и fork не считаются reset, если они переносят предыдущую историю.

После ответа worker coordinator снимает lease только через `observe`, `complete`
или `block`, передав одновременно совпадающие `--run-id` и
`--expected-revision`. Если worker не может продолжить, используй проверенный
`block` с этими же значениями; не бросай state с активным lease и не создавай
заменяющий packet.

## Блокер

Сохрани проверенный факт, точный вопрос/действие, уже выполненные безопасные
проверки и одно продолжение. Не объявляй временную неопределённость завершённой
задачей и не записывай секреты в observation.
