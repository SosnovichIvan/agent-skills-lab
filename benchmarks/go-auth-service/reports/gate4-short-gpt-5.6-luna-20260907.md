# GATE-4 shortened comparison

Дата: 2026-09-07

Профиль: `gpt-5.6-luna`, reasoning `medium`

Run ID: `gate4-short-rerun-gpt-5.6-luna-20260907`

Candidate commit: `5b5fae588ecdda4dc4c427a32601e9f6a6ef1aa5`

Каждый вариант был запущен один раз в новом пустом проекте и новой начальной
сессии с лимитом первых 16 задач. До повторного запуска исправлена ложная
readiness-проверка: промежуточная реализация до operations-блока ещё не обязана
иметь `GET /healthz`, поэтому готовностью процесса теперь считается любой
полученный HTTP-ответ. Конкретные контракты по-прежнему проверяются отдельными
behavioral gates.

## Валидность

| Вариант | Execution | Quality | Принято задач | Model attempts |
| --- | --- | --- | ---: | ---: |
| Control skill v2 | `complete` | `fail` | 10/16 | 13 |
| Candidate skill v3 | `complete` | `fail` | 16/16 | 20 |
| AI only | `complete` | `fail` | 16/16 | 17 |

Все три execution run завершились без infrastructure interruption. Control
терминально завершился после двух неудачных попыток задачи `3.3`: сервер
запускался, но фактически создавал разные request ID для заголовка и JSON body.
Это продуктовый quality failure, а не ошибка orchestration. Поэтому control
валиден для проверки выполнения GATE-4, но его время и токены нельзя напрямую
сравнивать с полными 16-task runs.

Candidate и AI only дошли до 16/16. Их итоговый `quality=fail` ожидаем для
сокращённого каталога: полный финальный verifier также проверяет idempotency,
organizations и custom roles из ещё не выполненной второй половины задач. Оба
варианта прошли доступный на задаче `3.3` контракт `request_id_consistency`.

## Сопоставимые результаты 16/16

| Показатель | Candidate skill v3 | AI only | Изменение candidate |
| --- | ---: | ---: | ---: |
| Wall time модели | 2,135.023 s | 1,212.111 s | +76.14% |
| Input tokens | 7,344,078 | 28,319,231 | -74.07% |
| Cached input tokens | 6,361,856 | 27,152,384 | -76.57% |
| Uncached input tokens | 982,222 | 1,166,847 | -15.82% |
| Output tokens | 152,304 | 377,011 | -59.60% |
| Reasoning output tokens | 36,482 | 91,929 | -60.32% |
| Total input + output | 7,496,382 | 28,696,242 | -73.88% |
| Repair tokens | 0 | 2,296,689 | -100.00% |

`total` — техническая сумма input + output, а не оценка billed tokens. Cached и
uncached input поэтому приведены отдельно.

Candidate выполнил три architecture review и один recovery, закрыв blocker
ошибочного HTTP error-envelope wiring. Эти дополнительные model calls и три
reset handoff объясняют увеличение wall time. Одновременно структурированное
состояние резко сократило повторную передачу накопленной истории. Наиболее
консервативный результат — сокращение uncached input на 15.82%; сокращение
total tokens на 73.88% в основном связано с cached input и не должно
интерпретироваться как такая же экономия стоимости без billing-данных.

## Control diagnostic

До продуктового отказа на задаче `3.3` control принял 10 задач, потратил
1,267.438 s и 5,701,253 total tokens, включая 934,834 uncached input и 783,114
repair tokens. Эти значения не включаются в 16-task performance comparison.

## Integrity

- Experiment manifest SHA-256:
  `a9fc1280cc453bae16befa7fd735167be0ab385b4b73ef26d5d6b26bb920894c`.
- Task catalog SHA-256:
  `95910c94969f05ab9101116fdc0e5aec6b980502a3293291956c7373dcb260c9`.
- Behavior verifier SHA-256:
  `b18e7e2490976c8dc9c9e6efc9cc9bce04b11f40d4eb24409422093635684a4d`.
- Control metrics SHA-256:
  `8d8bc0c28d1ea19535ec6ece7de17a55b33061dd084331f5b035bfdb250677bd`.
- Candidate metrics SHA-256:
  `babd8bd248b71dfa6d20a5ab715a17beeec7ced3f2122f9133c899a81e1cc738`.
- AI-only metrics SHA-256:
  `3f82e949ababca52f8a1df4b539f991d6a4f88f8d11a3ed6929a8d6e70901d9e`.

Raw model transcripts, generated projects и worker packets остаются в ignored
benchmark output и не публикуются.
