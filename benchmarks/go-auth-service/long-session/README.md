# Long-session benchmark: multi-tenant IAM

Этот benchmark проверяет `execution-state` в режиме, для которого он создан:
один продукт развивается последовательностью из 32 semantic chunks, а объём
кода и число связанных решений растут от шага к шагу.

## Зафиксированный профиль

- дата: 3 сентября 2026 года;
- модель: `gpt-5.6-luna`;
- reasoning effort: `medium`;
- Codex CLI: `0.147.0`;
- Go: `1.25.13 darwin/arm64`;
- задача: multi-tenant IAM service на Go, только стандартная библиотека;
- 32 одинаковых упорядоченных task, максимум две model-попытки на task;
- тестовые файлы и внешние Go-модули запрещены;
- один запуск на вариант (`n=1`).

Модель и reasoning effort зафиксированы константами в harness. Обычный запуск
не принимает параметры, которые могли бы незаметно сменить контрольный профиль.

| Вариант | Требования | Контекст между task |
| --- | --- | --- |
| `01-skill-standalone` | единый requirements-файл | новый ephemeral worker + execution state |
| `02-ai-only` | единый requirements-файл | одна persistent resume-сессия |
| `03-sdd-skill` | OpenSpec | новый ephemeral worker + execution state |
| `04-sdd-only` | OpenSpec | одна persistent resume-сессия |

Skill-варианты получают один bootstrap-turn, после чего каждый task выполняется
в новом контексте по revision-bound packet. Baseline-варианты сохраняют одну
сессию на все 32 task. OpenSpec-варианты используют один и тот же `proposal`,
`design`, `spec` и `tasks.md`; checkbox обновляется строго по одному за шаг.

После каждого task внешний harness запускает `gofmt`, `go test ./...`,
`go vet ./...`, проверяет отсутствие test files и внешних modules, затем
проверяет маркеры текущего контракта. Финальный gate повторяет проверки для всех
32 контрактов. После исправления семантических маркеров все четыре итоговых
проекта повторно прошли одну и ту же финальную версию verifier.

`duration_seconds` — сумма wall time model-процессов, а не полное время harness:
в неё не входят внешние verifier-вызовы и паузы между запусками. `Total tokens`
берётся из Codex JSONL usage; cached input является частью input, а не прибавкой
к нему.

## Raw-результаты

Raw сохраняет все model-turn, включая repairs, которые позднее оказались
вызваны ошибочными verifier markers.

| Конфигурация | Task attempts | Model wall time | Input (cached) | Output | Total tokens | Final gate |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Skill без SDD | 33 | 4 648,180 с | 9 409 676 (8 057 344) | 173 190 | 9 582 866 | Да |
| AI без skill и SDD | 34 | 2 399,566 с | 159 448 900 (154 678 016) | 1 362 296 | 160 811 196 | Да |
| OpenSpec + skill | 34 | 5 147,156 с | 12 480 678 (11 171 328) | 188 462 | 12 669 140 | Да |
| OpenSpec без skill | 34 | 3 127,131 с | 249 466 633 (244 286 720) | 1 679 845 | 251 146 478 | Да |

У skill-вариантов total time и tokens включают ещё один bootstrap-turn, который
не входит в колонку `Task attempts`.

Парное отличие skill относительно baseline:

| Пара | Total tokens | Model wall time |
| --- | ---: | ---: |
| Standalone | −94,04% | +93,71% |
| OpenSpec | −94,96% | +64,60% |

Skill дал ожидаемую экономию токенов в длинной сессии, но не ускорение. Новый
worker не перечитывает transcript, однако заново ориентируется в растущем
репозитории. Cold start и rehydration кода доминируют во времени.

## Рост по квартилям

В таблице приведён raw `Total tokens` для task 1–8, 9–16, 17–24 и 25–32.

| Конфигурация | 1–8 | 9–16 | 17–24 | 25–32 |
| --- | ---: | ---: | ---: | ---: |
| Skill без SDD | 1 439 766 | 2 150 884 | 2 522 539 | 3 399 479 |
| AI без skill и SDD | 5 698 850 | 18 861 594 | 51 235 551 | 85 015 201 |
| OpenSpec + skill | 1 522 066 | 2 415 698 | 3 283 669 | 5 415 461 |
| OpenSpec без skill | 6 917 627 | 24 351 446 | 63 096 504 | 156 780 901 |

У persistent baseline последний квартиль дороже первого в `14,92x` без SDD и
в `22,66x` с OpenSpec. У skill рост существенно меньше, но не константный:
`2,36x` без SDD и `3,56x` с OpenSpec. Значит, transcript reset работает, однако
гипотеза «стоимость каждого chunk постоянна» для растущего codebase не
подтверждается. Execution state остаётся мал: финальные файлы занимают 1 547 и
1 526 байт, а рост создаёт повторное чтение кода и SDD-источников.

## Repairs и normalized-срез

Во время первого запуска обнаружены слишком буквальные markers:

- task `6.3` принимал только `type Invite struct`, хотя корректная реализация
  использовала `InviteToken`;
- task `7.2` искал цельную строку `/roles/` и функцию `revokeRole`, хотя
  корректные routers разбирали path по сегментам, а один вариант использовал
  имя `removeRole`.

Markers заменены на семантические варианты. Исправный код был принят через
verifier-only recovery без дополнительного model-turn. Raw-метрики не
переписаны. Для анализа ниже исключена только вторая, заведомо лишняя попытка
после уже корректной первой реализации: две попытки у `03-sdd-skill` и одна у
`04-sdd-only`.

| Конфигурация | Normalized task attempts | Model wall time | Total tokens |
| --- | ---: | ---: | ---: |
| Skill без SDD | 33 | 4 648,180 с | 9 582 866 |
| AI без skill и SDD | 34 | 2 399,566 с | 160 811 196 |
| OpenSpec + skill | 32 | 4 823,231 с | 11 738 808 |
| OpenSpec без skill | 33 | 3 096,915 с | 237 029 511 |

В normalized OpenSpec-паре skill экономит `95,05%` токенов и занимает на
`55,74%` больше model time. Добавление OpenSpec к skill увеличило tokens на
`22,50%`, а время — на `3,77%`. Добавление OpenSpec без skill увеличило tokens
на `47,40%`, а время — на `29,06%` относительно AI-only.

Подтверждённые implementation repairs:

- Skill без SDD: один repair на task `4.3`;
- AI без skill и SDD: repairs на `2.4` и `5.4`;
- OpenSpec + skill: ни одного после исключения false-negative markers;
- OpenSpec без skill: один repair на `5.4`; `7.2` был verifier false negative.

На этом единственном прогоне OpenSpec + skill дал лучший first-pass result, но
`n=1` недостаточно для вывода о причинности.

## Выводы для следующей версии skill

1. Reset между semantic chunks действительно устраняет квадратичный рост
   transcript: экономия в обеих парах составила около 95%.
2. Полный cold worker слишком медленный. Нужен warm pool либо быстрый
   fresh-context launch без повторной инициализации CLI.
3. State должен переносить компактную карту релевантных файлов и символов с
   revision/hash, а packet — разрешать точечное чтение вместо повторного scan.
4. Размер chunk следует выбирать по связности контрактов и ожидаемому числу
   затрагиваемых файлов; слишком мелкие chunks умножают rehydration overhead.
5. Verifier должен проверять поведение или семантическую структуру, а не имена
   типов, функций и способ сборки URL.
6. Для окончательного вывода нужны рандомизированный порядок, минимум 3–5
   повторов и скрытые поведенческие/security tests.

## Воспроизводимость

Полный запуск:

```bash
python3 benchmarks/go-auth-service/long-session/run_long_benchmark.py \
  --run-id long-v1-gpt-5.6-luna-<DATE>
```

Продолжить конкретный вариант после обычного прерывания можно тем же `run-id` и
`--variant`. `--recover-verifier-failure` допустим только после независимого
подтверждения, что текущий код проходит исправленный verifier; флаг не должен
использоваться для принятия реального implementation failure.

- [Требования](requirements.md)
- [Каталог 32 task](tasks.json)
- [Harness](run_long_benchmark.py)
- [Внешний verifier](verify_long_project.py)
- [OpenSpec fixture](openspec-template)
- [Prompts](prompts)
- [Полные метрики](../results/long-runs/long-v1-gpt-5.6-luna-20260903/metrics.json)
- [Raw JSONL и итоговые проекты](../results/long-runs/long-v1-gpt-5.6-luna-20260903)

Перед валидным измерением baseline был обнаружен и устранён дефект harness:
`codex exec resume` запускался без явного `workspace-write` и становился
read-only. Невалидный частичный вариант был пересоздан; его usage не входит в
приведённые метрики.
