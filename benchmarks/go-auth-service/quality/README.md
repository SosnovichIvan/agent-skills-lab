# Quality benchmark: standalone only

Этот каталог задаёт обязательный протокол новых экспериментов качества для
`execution-state`.

## Зафиксированное решение

Начиная со следующей версии навыка новые эксперименты не создают варианты с
OpenSpec/SDD. Сравниваются только независимые проекты, которые стартуют из
пустого каталога и из новой model session:

1. зафиксированная предыдущая версия `execution-state` (control);
2. разрабатываемая версия `execution-state` (candidate);
3. обычный AI без `execution-state` (baseline).

OpenSpec остаётся поддерживаемым источником задач самого навыка, но больше не
является измеряемой переменной. Это уменьшает число вариантов, стоимость
эксперимента и смешение эффектов SDD с эффектом execution state.

## Контроль профиля

- model: `gpt-5.6-luna`;
- reasoning effort: `medium`;
- одинаковые требования, порядок semantic chunks и permissions;
- новый пустой проект и новая сессия для каждого варианта;
- минимум три повтора с чередованием порядка вариантов;
- control привязывается к Git commit, candidate — к проверяемому commit;
- verifier хранится вне генерируемого проекта и не передаётся worker-агенту.

Изменение модели, reasoning effort, задания или verifier создаёт новый профиль
эксперимента и требует отдельного review. Старые raw-результаты не
перезаписываются.

Профиль хранится в `experiment.json`. После commit candidate создай immutable
план запуска:

```bash
python3 benchmarks/go-auth-service/quality/prepare_experiment.py \
  --candidate-ref <FULL_COMMIT_SHA> --run-id quality-v1-<DATE>
```

Preflight отклоняет незакоммиченный ref, повторное использование run ID и любое
появление SDD/OpenSpec-варианта. Он формирует schema v2 task catalog с contracts,
cohesion keys, external regression check IDs и финальным integration bridge.

## Метрики

Главные метрики:

- пройденные поведенческие контракты;
- число дефектов и repair-turn;
- total/input/output tokens;
- model wall time и end-to-end time;
- число fresh-context handoff;
- размер state и worker packet;
- число и максимальный размер исходных файлов.

Компиляция, `go vet`, `gofmt` и marker-проверки остаются preflight gate, но не
считаются доказательством корректного поведения. Финальный результат определяет
независимый black-box verifier.

## Первый набор обязательных контрактов

Verifier должен как минимум обнаруживать дефекты, найденные в предыдущем
long-session запуске:

- `POST /v1/organizations/{orgID}/roles` существует и создаёт custom role;
- созданную роль можно назначить участнику той же организации;
- `X-Request-ID` совпадает в response header и error envelope;
- повтор с тем же idempotency key и тем же body возвращает прежний результат;
- тот же idempotency key с другим body возвращает conflict, а не replay.

Команда запуска verifier и формат машинного отчёта будут находиться в этом
каталоге. Отчёт каждого запуска сохраняется рядом с новыми immutable raw
результатами.

## Проверка verifier на старых артефактах

Команда:

```bash
python3 benchmarks/go-auth-service/quality/verify_behavior.py \
  --project <GENERATED_PROJECT> --output <REPORT.json>
```

Verifier был проверен на сохранённых standalone-проектах запуска
`long-v1-gpt-5.6-luna-20260903`:

| Контракт | Старый skill-run | Старый AI-only |
| --- | ---: | ---: |
| Request ID header = error body | pass | fail |
| Одинаковый idempotent request replay | fail | pass |
| Изменённый body с тем же key → conflict | fail | fail |
| Создание custom role | pass | fail |
| Назначение custom role | pass | fail |

Оба проекта собираются, но полный behavioral gate не проходит ни один. Это
подтверждает, что verifier ловит известные дефекты, пропущенные прежним
compile/static gate. Unit self-check запускается так:

```bash
python3 -m unittest discover \
  -s benchmarks/go-auth-service/quality -p 'test_*.py' -v
```
