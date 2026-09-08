# Quality benchmark: standalone only

Этот каталог содержит актуальный standalone-сценарий проверки
`execution-state 1.0.0`. Исторические прогоны и межверсионные control snapshots
не хранятся.

## Зафиксированное решение

Эксперимент создаёт только независимые проекты, которые стартуют из пустого
каталога и новой model session:

1. `execution-state 1.0.0` из указанного Git commit;
2. обычный AI без `execution-state`.

OpenSpec остаётся поддерживаемым источником задач самого навыка, но больше не
является измеряемой переменной. Это уменьшает число вариантов, стоимость
эксперимента и смешение эффектов SDD с эффектом execution state.

## Контроль профиля

- model: `gpt-5.6-luna`;
- reasoning effort: `medium`;
- одинаковые требования, порядок semantic chunks и permissions;
- новый пустой проект и новая сессия для каждого варианта;
- минимум три повтора с чередованием порядка вариантов;
- skill привязывается к полному Git commit SHA;
- verifier хранится вне генерируемого проекта и не передаётся worker-агенту.
- неуспех одного варианта фиксируется в metrics, но не останавливает остальные
  варианты и повторы эксперимента.

Изменение модели, reasoning effort, задания или verifier создаёт новый профиль
эксперимента и требует отдельного review. Результаты по умолчанию остаются
локальными и не коммитятся.

Профиль хранится в `experiment.json`. После commit версии skill создай
immutable план запуска:

```bash
python3 benchmarks/go-auth-service/quality/prepare_experiment.py \
  --skill-ref <FULL_COMMIT_SHA> --run-id quality-<DATE>
```

Preflight отклоняет незакоммиченный ref, повторное использование run ID и любое
появление SDD/OpenSpec-варианта. Он сохраняет immutable Git-снимок skill и
формирует task catalog schema `1.0.0` с contracts, cohesion keys, external
regression check IDs и финальным integration bridge.

Runner находится в `quality/run_benchmark.py`. Он принимает только
standalone-варианты, task catalog schema `1.0.0` и worker protocol `1.0.0`.
Старые версии protocol и OpenSpec/SDD-ветки не поддерживаются.

## Метрики

Главные метрики:

- пройденные поведенческие контракты;
- число дефектов и repair-turn;
- total, uncached input, cached input, output и reasoning tokens;
- model wall time и end-to-end time;
- число fresh-context handoff;
- expected/actual handoff, cold-start time и repair overhead;
- размер state и worker packet;
- context-map precision/coverage для контрольных chunks;
- число и максимальный размер исходных файлов.

Компиляция, `go vet`, `gofmt` и marker-проверки остаются preflight gate, но не
считаются доказательством корректного поведения. Финальный результат определяет
независимый black-box verifier.

## Первый набор обязательных контрактов

Verifier считает обязательными следующие поведенческие контракты:

- `POST /v1/organizations/{orgID}/roles` существует и создаёт custom role;
- созданную роль можно назначить участнику той же организации;
- `X-Request-ID` совпадает в response header и error envelope;
- повтор с тем же idempotency key и тем же body возвращает прежний результат;
- тот же idempotency key с другим body возвращает HTTP `409 Conflict`, а не
  replay или иной статус.

Команда запуска verifier и формат машинного отчёта находятся в этом каталоге.
Raw-результаты и отчёты остаются вне Git.

## Проверка verifier

Команда:

```bash
python3 benchmarks/go-auth-service/quality/verify_behavior.py \
  --project <GENERATED_PROJECT> --output <REPORT.json>
```

Unit self-check запускается так:

```bash
python3 -m unittest discover \
  -s benchmarks/go-auth-service/quality -p 'test_*.py' -v
```

## Resume smoke без модели

Перед model canary выполни локальную fault-injection проверку:

```bash
python3 benchmarks/go-auth-service/quality/resume_smoke.py \
  --output /tmp/execution-state-resume-smoke
```

Smoke принудительно останавливает локальный fake worker, повторно читает
сохранённые metrics, сохраняет завершённый prefix и запускает только прерванную
задачу. Успешный отчёт содержит `model_requests: 0`, `resume_index: 1` и две
завершённые fake-задачи. Сетевой доступ и agent CLI не используются.
