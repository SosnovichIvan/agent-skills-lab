# Задачи стабилизации и валидации эксперимента

Этот backlog должен быть выполнен до следующего полного запуска `3 × 3 × 32`.

Статусы:

- `[ ]` — не начато;
- `[-]` — выполняется;
- `[x]` — завершено и подтверждено указанной проверкой.

## P0. Надёжность runner

- [x] **VAL-001 — Разделить статус выполнения и качество результата.**
  Хранить отдельно `execution_status: running|complete|interrupted|infra_error`
  и `quality_outcome: pass|fail|not_evaluated`. Непройденный контракт продукта
  не должен означать поломку эксперимента.
  Проверка: unit test для всех допустимых переходов и запрета неоднозначного
  `status=failed`.

- [x] **VAL-002 — Добавить durable heartbeat.**
  Во время model call сохранять `active_attempt`, task ID, PID/runner ID,
  `started_at` и регулярно обновляемый `heartbeat_at` атомарной записью.
  Проверка: timestamp обновляется во время долгого fake-worker вызова и
  перестаёт обновляться после принудительной остановки.

- [x] **VAL-003 — Обнаруживать stale run.**
  `run_experiment.py` не должен бесконечно ждать `status=running`. После
  настраиваемого TTL он обязан классифицировать запуск как `interrupted`.
  Проверка: fixture со старым heartbeat распознаётся без `sleep` и без model
  request.

- [x] **VAL-004 — Возобновлять запуск с durable checkpoint.**
  При прерывании после завершённого chunk продолжать со следующего task. При
  прерывании с активным worker lease безопасно переиспользовать тот же
  `run_id`/revision либо явно закрыть lease как infrastructure interruption.
  Проверка: fault-injection останавливает runner внутри task, повторный запуск
  завершается без повторного выполнения уже принятых chunks.

- [x] **VAL-005 — Сохранять потоковый orchestration log.**
  Записывать начало и окончание subprocess, exit code, timeout/signal и путь к
  raw stdout/stderr до обновления metrics.
  Проверка: `SIGTERM`, timeout и ненулевой exit code различимы в отчёте.

- [x] **VAL-006 — Продолжать остальные варианты после любого outcome.**
  Product failure одного варианта не должен останавливать другие варианты и
  повторы.
  Проверка: fake control возвращает quality fail, после чего candidate и
  baseline всё равно выполняются.

## P0. Достоверность verifier

- [x] **VAL-007 — Устранить ложный отказ invite model.**
  Проверка должна принимать семантически эквивалентные `Invite`,
  `InviteToken`, `OrganizationInvite` и type alias либо использовать Go AST.
  Проверка: regression fixtures для всех четырёх вариантов и отрицательный
  fixture без invite-модели.

- [x] **VAL-008 — Перевести структурные Go-проверки с regex на AST.**
  Модели, methods и HTTP wiring проверять через parser/AST там, где имя или
  форматирование не является частью контракта.
  Проверка: table-driven fixtures с несколькими корректными архитектурами.

- [x] **VAL-009 — Не раскрывать внутренние marker strings worker-агенту.**
  Repair получает нарушенный пользовательский контракт и наблюдаемое
  поведение, а не имя regex. Это исключает исправления-комментарии ради gate.
  Проверка: сохранённый repair prompt не содержит `Missing markers` и названий
  внутренних regex-групп.

- [x] **VAL-010 — Добавить per-chunk behavioral gates.**
  Для критичных HTTP-контрактов выполнять узкие black-box проверки сразу после
  соответствующего chunk, а не только в финале.
  Проверка: намеренно сломанные idempotency, custom-role и request-ID fixtures
  отклоняются на своих задачах.

- [x] **VAL-011 — Проверить verifier на альтернативных корректных реализациях.**
  Минимум три независимо структурированных fixture должны проходить одинаковые
  контракты; каждый известный дефект должен иметь отрицательный fixture.
  Проверка: documented verifier matrix без false positive/negative на наборе.

## P1. Architecture review как реальный gate

- [x] **VAL-012 — Ввести structured review result.**
  Reviewer возвращает `verdict`, `blockers`, `planned_gaps`, `recommendations`
  и checks по JSON Schema, а не свободный Markdown.
  Проверка: malformed result отклоняется, все четыре класса данных сохраняются
  раздельно.

- [x] **VAL-013 — Передавать reviewer план будущих задач.**
  Reviewer должен отличать дефект завершённой реализации от функции, которая
  запланирована будущим chunk.
  Проверка: отсутствие `/metrics` до task `8.3` классифицируется как
  `planned_gap`, а потеря уже реализованного `/login` — как blocker.

- [x] **VAL-014 — Не снимать review gate при blocker.**
  `verdict=blocked` создаёт recovery chunk или переводит state в blocked;
  следующий обычный implementation packet запрещён.
  Проверка: controller не выдаёт packet до принятого recovery result.

- [x] **VAL-015 — Подтверждать устранение blocker.**
  Recovery chunk содержит конкретный contract и regression check, после чего
  повторный review закрывает blocker.
  Проверка: полный сценарий `review → recovery → re-review → continue`.

## P1. Проверка механики execution-state

- [x] **VAL-016 — Передавать языковую политику непосредственно worker.**
  Packet или обязательный runtime envelope должен явно задавать краткий
  английский для operational output и сохранение языка source requirements.
  Проверка: fresh worker, не читавший `SKILL.md`, возвращает английские summary,
  evidence, blocker и context purpose.

- [x] **VAL-017 — Заполнить `reads` и `writes` в benchmark catalog.**
  Каждая задача должна иметь ожидаемые области чтения и записи либо явное
  обоснование `discovery_required`.
  Проверка: preflight отклоняет пустые `reads`/`writes` без такого обоснования.

- [x] **VAL-018 — Проверить релевантность context map.**
  Packet должен включать реализованные контракты и файлы, необходимые текущему
  chunk, не перенося историю и сырые логи.
  Проверка: для задач `3.2`, `4.3`, `6.4`, `7.1` и `8.1` определены ожидаемые
  context entries; precision/coverage записываются в metrics.

- [x] **VAL-019 — Сделать `quality.next_handoff` источником решения.**
  Harness не должен самостоятельно решать continuation только по
  `cohesion_key`; он исполняет подтверждённое решение controller/runtime plan.
  Проверка: fixtures принудительно выбирают `continue`, `reset` и `checkpoint`,
  а фактический session ID соответствует решению.

- [x] **VAL-020 — Проверять фактическую смену контекста.**
  Для `reset` фиксировать новый session ID; для `continue` — сохранение текущего;
  manual/checkpoint не выдавать за fresh context.
  Проверка: metrics содержат expected/actual handoff и отклоняют расхождение.

- [x] **VAL-021 — Валидировать язык сохранённого operational state.**
  Не добавлять ненадёжный language detector в product helper. В benchmark
  проверять согласованный набор полей через controlled worker fixtures и
  отдельно разрешать Unicode identifiers/source quotes.
  Проверка: русское source requirement сохраняется, а сгенерированные
  operational поля имеют ожидаемый английский fixture output.

## P2. Метрики и воспроизводимость

- [x] **VAL-022 — Разделить token-метрики.**
  Отдельно показывать uncached input, cached input, output, reasoning и total;
  не называть cached tokens фактически оплачиваемыми без данных billing.
  Проверка: агрегаты равны сумме raw turn usage.

- [x] **VAL-023 — Добавить метрики стоимости handoff.**
  Считать fresh/continue/checkpoint, cold-start time, число прочитанных файлов,
  packet/context bytes и repair overhead на chunk.
  Проверка: итоговый отчёт строится без чтения model transcript.

- [x] **VAL-024 — Зафиксировать полный environment manifest.**
  Сохранять model, reasoning, CLI/Python/Go versions, commit SHA, verifier hash,
  task-catalog hash и runtime capabilities.
  Проверка: resume отклоняется при несовместимом manifest.

- [x] **VAL-025 — Генерировать итоговый validity report.**
  Отчёт отдельно показывает protocol validity, infrastructure interruptions и
  product quality каждого варианта.
  Проверка: неполный run не попадает в сравнительную статистику автоматически.

## P0. Очистка и фиксация версии скила

- [x] **VAL-026 — Удалить параллельную legacy-реализацию state.**
  После проверки отсутствия актуальных callers удалить
  `init_execution_state.py`, `init_standalone_state.py`,
  `apply_state_patch.py`, `build_worker_packet.py`,
  `validate_execution_state.py` и `state_lib.py`. Единственным публичным
  helper остаётся `statectl.py`.
  Проверка: repository search не находит imports/вызовов удалённых entrypoints,
  а init, transition, packet и validation scenarios проходят через `statectl`.

- [x] **VAL-027 — Удалить встроенную миграцию schema v3.**
  Удалить `statectl migrate`, migration implementation, тест и упоминания v3 из
  актуальной документации. Предыдущий Git tag остаётся источником старого
  migration path; новая версия принимает только собственную schema.
  Проверка: `statectl` отклоняет старую schema с понятной ошибкой, а в runtime
  package нет строк `migrate` и `schema v3`.

- [x] **VAL-028 — Выпустить новый строгий worker protocol.**
  Удалить дублирующее legacy-поле packet `revision`, сделать `state_id`
  обязательным и добавить переносимую языковую политику непосредственно в
  worker request. Изменение оформить как новую версию protocol, не менять v2
  молча.
  Проверка: producer, validator, runtime binding, JSON Schema и worker fixtures
  принимают только новый канонический envelope.

- [x] **VAL-029 — Удалить или маршрутизировать orphan references.**
  Перенести уникальные checkpoint-инварианты в `runtime-contract.md`, удалить
  `checkpoint-policy.md` и проверить, что каждый оставшийся reference доступен
  по осмысленной ссылке из `SKILL.md` или другого routed reference.
  Проверка: link audit не находит orphan или broken references.

- [x] **VAL-030 — Вынести тесты из устанавливаемой папки skill.**
  Переместить `skills/execution-state/tests` в отдельный repository-level test
  каталог. Пользовательский archive должен содержать только runtime и
  документацию.
  Проверка: test discovery работает из нового пути, а packaged skill не содержит
  test fixtures и `__pycache__`.

- [x] **VAL-031 — Удалить пустые и сгенерированные файлы из skill package.**
  Удалить пустую `skills/execution-state/prompts/`, caches, bytecode и другие
  локальные артефакты. Prompt templates не возвращать без отдельного продуктового
  решения.
  Проверка: package inventory соответствует allowlist и `git status` не содержит
  generated artifacts.

- [x] **VAL-032 — Отделить продуктовый README от истории исследований.**
  В README skill оставить установку, текущие contracts, ограничения и ссылку
  только на последний валидный сравнительный report. Старые исследования
  доступны через Git history, но не публикуются рядом с текущей статистикой.
  Проверка: README не приписывает старую статистику текущей версии и не содержит
  broken links.

- [x] **VAL-033 — Сделать quality benchmark единственным актуальным harness.**
  Перенести необходимые standalone runner/tasks/schema из legacy long-session
  в канонический quality benchmark и удалить неиспользуемые SDD-ветки,
  `run_benchmark.py`, `run_benchmark_v3.py`, старые prompts/projects и поддержку
  worker/result v1.
  Проверка: current experiment запускается без импортов и путей к legacy harness;
  repository search находит старые версии только в архивном report или Git
  history.

- [x] **VAL-034 — Убрать raw benchmark artifacts из Git.**
  В репозитории оставить только последний curated report и минимальные
  regression fixtures; generated `quality-runs/` хранить вне Git. Предыдущие
  отчёты и агрегаты доступны через Git history.
  Проверка: raw JSONL, stderr, worker packets и generated projects не tracked,
  а опубликованный отчёт содержит hashes локальных исходных агрегатов.

- [x] **VAL-035 — Добавить единый release manifest.**
  Создать машинно-читаемый источник версии скила, state/task schemas, worker
  protocol, adapter schema и минимальной Python version. `statectl version` и
  документация должны использовать те же значения.
  Проверка: consistency test отклоняет любое расхождение constants, manifest и
  docs.

- [x] **VAL-036 — Проверять минимальный release archive.**
  Собирать skill по allowlist, сохранять SHA-256 и запрещать legacy scripts,
  tests, raw benchmark data, caches и пустые каталоги.
  Проверка: archive распаковывается в чистой директории, проходит skill
  validation и controller smoke-test.

- [ ] **VAL-037 — Зафиксировать immutable release tag.**
  До завершения validation использовать prerelease-версию. После всех gates
  создать стабильный SemVer tag и использовать его SHA как control следующего
  эксперимента.
  Проверка: tag указывает на проверенный commit, archive checksum совпадает, а
  manifest candidate/control хранит полный commit SHA.

## Обязательная последовательность запусков

- [x] **GATE-0 — Release hygiene.** Завершены VAL-026—VAL-036; package inventory
  соответствует allowlist, legacy runtime отсутствует, prerelease archive и
  checksum воспроизводимы.
- [x] **GATE-1 — Unit и fixture suite.** Все тесты skill, runner, statectl,
  verifier и fault-injection проходят локально.
- [x] **GATE-2 — Resume smoke.** Fake worker принудительно останавливается и
  успешно возобновляется без model usage.
- [x] **GATE-3 — Model canary.** Один candidate, 8 задач,
  `review_interval=4`: два review, один принудительный recovery и один reset.
- [x] **GATE-4 — Сокращённое сравнение.** Control, candidate и baseline по
  одному разу на первых 16 задачах; все execution runs должны завершиться,
  независимо от quality outcome. Проверено запуском
  `gate4-short-rerun-gpt-5.6-luna-20260907`; все три execution status равны
  `complete`. Control получил продуктовый fail на задаче `3.3`, candidate и
  baseline завершили 16/16. Результаты и ограничения сравнения зафиксированы в
  `../reports/gate4-short-gpt-5.6-luna-20260907.md`.
- [ ] **GATE-5 — Полный эксперимент.** Только после GATE-0—GATE-4 запустить три
  повтора всех трёх вариантов на 32 задачах. Стабильный release tag из VAL-037
  создаётся только после валидного полного результата.

Полный эксперимент считается валидным, если все девять execution runs имеют
`execution_status=complete`, immutable manifests совпадают, нет необъяснённых
stale attempts, а quality outcome рассчитан для каждого варианта. Прохождение
всех продуктовых контрактов не является условием валидности эксперимента — это
измеряемый результат.
