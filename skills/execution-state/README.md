# execution-state

`execution-state` — explicit-only skill для долгих агентных задач. Он хранит
компактное проверяемое состояние между semantic chunks и может передавать
работу в новый context без transcript.

Core не зависит от Codex, Claude, Gemini или модели. Agent CLI подключается
через capability-based adapter; OpenSpec является отдельным источником задач и
не влияет на выбор runtime.

## Маршрутизация

```text
явный вызов execution-state
├── короткая связная задача → passthrough, state не создаётся
├── reset недоступен        → lite checkpoint
└── длинная задача          → chunk → checkpoint → fresh/compact/manual
```

Источник требований (`standalone` или `openspec`) и профиль выполнения (`lite`
или `reset`) хранятся независимо. Для OpenSpec `tasks.md` остаётся единственным
task ledger; execution-state не создаёт task JSON.

## Совместимость CLI

| Runtime | Lite | Fresh context | Machine output | Особенность |
| --- | ---: | ---: | ---: | --- |
| Codex CLI | Да | Новый `codex exec --ephemeral` | JSONL | Встроенный adapter |
| Claude Code | Да | Новый `claude -p --no-session-persistence` | JSON stream | Встроенный adapter |
| Gemini CLI | Да | Новый non-interactive process | JSON stream | Persistence/schema capabilities ограничены |
| Другой CLI | Да | Через manifest | По manifest | Неизвестное безопасно становится `manual` |
| Любой UI/CLI без subprocess | Да | Manual handoff | Не требуется | Packet передаётся пользователем/runtime |

Auto-discovery не выполняет model request и не выбирает молча между несколькими
установленными CLI. Vendor-specific argv живёт только в adapters; state и worker
protocol остаются одинаковыми.

Skill не может стереть контекст уже работающей модели. В профиле `reset`
`statectl` создаёт ограниченный packet и проверяемый launch plan, а доверенный
внешний runner открывает новую CLI-сессию и передаёт ей конкретизированный
worker prompt. Resume/fork старой сессии строгим reset не считаются.

## Требования

| Компонент | Когда нужен | Для чего |
| --- | --- | --- |
| Полная папка `execution-state/` | Всегда | `SKILL.md` использует относительные `scripts/`, `references/` и `prompts/` |
| Python 3.9+ | Профили `lite` и `reset` | Запуск `scripts/statectl.py`; используются только модули стандартной библиотеки |
| Права чтения и записи в проекте | Профили `lite` и `reset` | Создание `.execution-state/`, checkpoint и worker packet |
| Один agent CLI или совместимый custom adapter | Автоматический `reset` | Создание нового model context |
| Аккаунт, авторизация и сеть выбранного CLI | Реальный fresh worker | Выполнение model request |
| Доверенный host-agent или внешний runner | Автоматический `reset` | Проверка и исполнение launch plan, который возвращает `statectl` |
| `tasks.md` с checkbox и стабильными ID | Только `source=openspec` | Источник задач и статусов OpenSpec |

Внешние Python-пакеты, виртуальное окружение и OpenSpec CLI не требуются.
Node.js может требоваться конкретному agent CLI, но не core. Git нужен только
для клонирования, а Go — только для воспроизведения benchmark.

Автоматический `reset` состоит из двух частей. `statectl runtime-plan` проверяет
state/lease и возвращает `executes:false`, безопасный argv и stdin framing.
Фактический запуск делает host-agent или доверенный runner в рамках обычных
permissions. Без него используй `manual-handoff` либо `lite`; экономия токенов
внутри текущей сессии тогда не гарантируется.

Windows-код имеет portable fallback для блокировок, но CI на Windows пока не
настроен. Подтверждённая среда текущей версии — macOS с Python 3.9; Linux
покрывается POSIX-механизмами, но также должен проверяться в CI перед
production-развёртыванием.

## Установка

Клонируй репозиторий и не отделяй `SKILL.md` от остальных файлов:

```bash
git clone https://github.com/SosnovichIvan/agent-skills-lab.git
cd agent-skills-lab
python3 -c 'import sys; assert sys.version_info >= (3, 9), sys.version'
python3 skills/execution-state/scripts/statectl.py --help
```

Затем подключи каталог `skills/execution-state` в native skills-directory
целевого агента. Можно скопировать папку или создать symlink:

| Агент | User scope | Project scope | Явный вызов |
| --- | --- | --- | --- |
| Codex | `~/.agents/skills/execution-state/` | `.agents/skills/execution-state/` | `$execution-state` |
| Claude Code | `~/.claude/skills/execution-state/` | `.claude/skills/execution-state/` | `/execution-state` |
| Gemini CLI | `gemini skills link <PATH>` | `.gemini/skills/execution-state/` или `.agents/skills/execution-state/` | явно попросить активировать `execution-state` |
| Другой агент | По документации агента | По документации агента | native invocation либо абсолютный путь к `SKILL.md` |

Актуальные каталоги и команды описаны в официальных руководствах
[Codex](https://developers.openai.com/codex/skills),
[Claude Code](https://code.claude.com/docs/en/slash-commands) и
[Gemini CLI](https://geminicli.com/docs/cli/using-agent-skills/).

Проверь установку и runtime:

```bash
python3 <PATH>/execution-state/scripts/statectl.py \
  route --source standalone --expected-turns 3 --adapter manual

python3 <PATH>/execution-state/scripts/statectl.py \
  runtime-probe --adapter auto
```

Первый вызов должен вернуть `decision: "passthrough"`. Второй не делает model
request: он запускает только короткий `<cli> --version`. Если найдено несколько
CLI, `auto` намеренно вернёт `manual`; передай нужный adapter явно. Probe пока
не проверяет минимальную версию и каждый launch flag, поэтому перед первым
reset дополнительно проверь `<cli> --help` и сформированный `runtime-plan`.

## Использование

- [Универсальный пользовательский prompt](prompts/use-execution-state.md)
- [Интеграция в документацию OpenSpec](prompts/openspec-documentation-integration.md)
- [Worker prompt новой сессии](prompts/worker.md)
- [Инструкции skill](SKILL.md)

Единственный публичный helper — `scripts/statectl.py`. Он выбирает route,
обслуживает schema/revision, создаёт packet и строит launch plan. Он не запускает
платный agent CLI самостоятельно.

`packet` — изменяющая операция: она повышает revision, создаёт единственный
активный `worker_lease` и возвращает `run_id`. `runtime-plan` принимает packet
только вместе с соответствующим `--id` либо `--state`. Результат worker
принимается через `observe`, `complete` или `block` с новой revision и тем же
`--run-id`; повторный packet до снятия lease отклоняется.

Для стороннего CLI передай `--adapter <ID> --manifest <PATH>` и только после
проверки manifest добавь `--trust-custom-adapter`. Этот флаг разрешает безопасный
version probe указанного executable, но не разрешает model run, bypass
permissions или другие внешние действия.

Подробности:

- [compact state schema](references/state-schema.md);
- [универсальный runtime contract](references/runtime-contract.md);
- [CLI adapters](references/cli-adapters.md);
- [OpenSpec-lite](references/openspec-integration.md);
- [worker protocol](references/worker-protocol.md).

## Long-session benchmark: 32 semantic chunks

Длинный benchmark наконец проверяет `reset`, а не только стоимость
`passthrough`. Один multi-tenant IAM service на Go развивался 32
последовательными task в четырёх вариантах. Контрольный профиль зафиксирован:
`gpt-5.6-luna`, reasoning effort `medium`, Codex CLI `0.147.0`, один запуск на
вариант.

| Конфигурация | Model wall time | Total tokens | Final gate |
| --- | ---: | ---: | ---: |
| Skill без SDD | 4 648,180 с | 9 582 866 | Да |
| AI без skill и SDD | 2 399,566 с | 160 811 196 | Да |
| OpenSpec + skill | 5 147,156 с | 12 669 140 | Да |
| OpenSpec без skill | 3 127,131 с | 251 146 478 | Да |

Raw-экономия skill составила `94,04%` токенов без SDD и `94,96%` с OpenSpec.
Цена — рост model wall time на `93,71%` и `64,60%`: fresh workers заново
ориентируются в растущем codebase. Квартильная стоимость skill тоже выросла,
хотя намного медленнее baseline. Следовательно, transcript reset работает, но
одного компактного state недостаточно для постоянной стоимости chunk; следующая
версия должна переносить компактную индекс-карту релевантного кода и сокращать
cold-start latency.

В raw OpenSpec-данных сохранены три лишних repair-turn, вызванных двумя
false-negative markers. В normalized OpenSpec-паре после исключения только этих
заведомо лишних turn экономия составляет `95,05%` токенов, а временной штраф —
`55,74%`. Все четыре итоговых проекта повторно прошли единый исправленный final
verifier. Полная методика, квартильные данные, repairs и ограничения:
[long-session benchmark](../../benchmarks/go-auth-service/long-session/README.md).

## Benchmark v3: Go auth service на контрольной модели

Контрольный профиль для проверок и гипотез зафиксирован в harness:
`gpt-5.6-luna`, reasoning effort `medium`. Параметры смены модели удалены из
CLI скрипта, поэтому обычный запуск не может незаметно создать несопоставимый
результат. Изменение контрольной модели требует явной правки констант и review.

Дата измерения: 3 сентября 2026 года; Codex CLI `0.147.0`. Каждый вариант
запускался один раз в отдельном изолированном проекте и новой
`codex exec --ephemeral`-сессии без пользовательских config/rules.
Standalone-проекты были пустыми; SDD-проекты содержали только одинаковый
заранее подготовленный OpenSpec fixture.

Старое задание на Go auth service является одним связным semantic chunk без
handoff. В обоих skill-логах агент явно сообщил о выборе `passthrough`, не читал
дополнительные execution-state references и не создавал `.execution-state`.
Поэтому этот опыт проверяет стоимость маршрутизации и skill-инструкций на одной
задаче, но не экономию `reset` на длинной сессии.

Все четыре результата прошли внешний `go test ./...`, `go vet ./...`, `gofmt`,
проверку отсутствия `*_test.go` и шесть простых substring-проверок обязательных
маркеров. Это compile/static gate, а не поведенческое тестирование. В обоих
OpenSpec-проектах завершены все семь checkbox.

| Конфигурация | Профиль | Время агента | Input tokens (cached) | Output | Total | Команды |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| execution-state v3, без SDD | passthrough | 160,593 с | 185 978 (165 632) | 6 250 | 192 228 | 8 |
| AI без skill и SDD | — | 201,703 с | 236 615 (192 768) | 9 233 | 245 848 | 8 |
| OpenSpec + execution-state v3 | passthrough | 195,911 с | 282 337 (259 584) | 8 370 | 290 707 | 10 |
| OpenSpec без skill | — | 222,049 с | 288 908 (245 248) | 9 408 | 298 316 | 9 |

Парные различия skill относительно соответствующего baseline:

| Пара | Время | Total tokens | Команды | Символы command output |
| --- | ---: | ---: | ---: | ---: |
| Standalone | −20,38% | −21,81% | 0,00% | −43,15% |
| OpenSpec | −11,77% | −2,55% | +11,11% | −58,40% |

### Сравнение execution-state v2 и v3

Обе версии измерены на `gpt-5.6-luna` с reasoning effort `medium`. Самая
наглядная разница — эффект skill внутри каждой версии относительно её baseline:

| Версия | Standalone: время / токены | OpenSpec: время / токены |
| --- | ---: | ---: |
| v2 | +157,89% / +242,21% | +8,80% / +29,32% |
| v3 | −20,38% / −21,81% | −11,77% / −2,55% |

Абсолютное изменение v3 относительно v2 по каждой конфигурации:

| Конфигурация | Время | Total tokens |
| --- | ---: | ---: |
| Skill без SDD | −64,29% | −73,34% |
| AI без skill и SDD | +15,66% | +16,69% |
| OpenSpec + skill | −39,48% | −55,65% |
| OpenSpec без skill | −25,38% | −41,15% |

Baseline тоже заметно изменился между запусками, особенно в OpenSpec. Поэтому
эти цифры показывают наблюдаемую разницу версий на одной модели, но не позволяют
приписать весь эффект только новой архитектуре skill.

### Анализ v3

В standalone обе стороны выполнили по 8 команд, но skill-run получил на 8 169
символов command output меньше. Его output снизился на 2 983 токена, total — на
53 620 токенов, а время — на 41,110 секунды.

В OpenSpec skill-run выполнил на одну команду больше, но прочитал на 15 007
символов command output меньше. Cached input вырос на 14 336 токенов (`+5,85%`),
однако output и uncached input снизились; итог — `−7 609` total tokens и
`−26,138` секунды.

Вывод: на этом повторе v3 выиграл в обеих парах и устранил наблюдавшийся в v2
перерасход. Результат поддерживает гипотезу, что ранний `passthrough` и запрет
лишних references уменьшают контекст, но `n=1` не доказывает причинность. Нужны
рандомизированный порядок и несколько повторов. Отдельный длинный benchmark с
32 настоящими fresh-context handoff теперь приведён
[выше](#long-session-benchmark-32-semantic-chunks).

### Воспроизводимость v3

Запуск выполняется без параметров выбора модели:

```bash
python3 benchmarks/go-auth-service/scripts/run_benchmark_v3.py \
  --run-id v3-gpt-5.6-luna-<DATE>
```

- [Задание](../../benchmarks/go-auth-service/prompts/base-task.md)
- [Harness v3](../../benchmarks/go-auth-service/scripts/run_benchmark_v3.py)
- [Метрики v3](../../benchmarks/go-auth-service/results/runs/v3-gpt-5.6-luna-20260903/metrics.json)
- [Prompts v3](../../benchmarks/go-auth-service/prompts/v3)
- [Raw JSONL и проекты](../../benchmarks/go-auth-service/results/runs/v3-gpt-5.6-luna-20260903)

## Benchmark v2: Go auth service

Дата измерения: 2 сентября 2026 года. Это исторический baseline предыдущей
реализации, а не результат нового capability-based core.

Одинаковый Go auth service создавался в четырёх новых проектах. Модель —
`gpt-5.6-luna`, reasoning effort `medium`; каждый запуск начинался отдельной
`codex exec --ephemeral`-сессией. Проверка `go test ./...` прошла во всех
вариантах, тестовые файлы не создавались.

| Конфигурация | Время | Input tokens (cached) | Output | Total |
| --- | ---: | ---: | ---: | ---: |
| Skill v2, без SDD | 449,751 с | 700 953 (652 032) | 20 040 | 720 993 |
| Без skill и SDD | 174,394 с | 203 245 (171 520) | 7 442 | 210 687 |
| OpenSpec + skill v2 | 323,734 с | 641 751 (595 456) | 13 724 | 655 475 |
| OpenSpec без skill | 297,559 с | 495 865 (449 280) | 11 011 | 506 876 |

Парные различия:

- standalone skill: время `+157,89%`, total tokens `+242,21%`;
- OpenSpec + skill: время `+8,80%`, total tokens `+29,32%`.

### Почему v2 стал дороже

Оба skill-run выполнялись в одной накопительной сессии: checkpoint не запускал
reset, а worker packet не использовался. В standalone было 20 tool calls против
9 и 31 104 символа служебного вывода до кодинга против 90. В OpenSpec число
tool calls почти совпало — 20 против 19, но служебный вывод до кодинга составил
28 514 против 3 511 символов.

В OpenSpec-паре разница input составила 145 886 токенов, разница cached input —
146 176, а uncached input у skill был на 290 токенов меньше. Следовательно,
перерасход почти полностью возник из повторения раннего skill/state context.

Новая версия адресует выявленные причины через passthrough, один `statectl`,
OpenSpec-lite, жёсткие лимиты и adapter-driven fresh context. Benchmark v3 выше
проверяет только passthrough; длинный reset-профиль измерен отдельно в разделе
[Long-session benchmark](#long-session-benchmark-32-semantic-chunks). Цифры v2
нельзя выдавать как результат v3 или длинного benchmark.

### Ограничения измерения

Это один запуск на вариант (`n=1`), compile-only проверка и одна относительно
короткая задача. Sandbox блокировал HTTP bind и добавлял повторные команды.
На момент v2 для подтверждения требовались 32–64 последовательных chunks,
одинаковый внешний quality evaluator, per-chunk usage, рандомизированный порядок
и несколько повторов. Первый такой 32-chunk прогон теперь опубликован в разделе
[Long-session benchmark](#long-session-benchmark-32-semantic-chunks); повторы и
рандомизация всё ещё нужны.

## Воспроизводимость v2

- [Задание](../../benchmarks/go-auth-service/prompts/base-task.md)
- [Harness](../../benchmarks/go-auth-service/scripts/run_benchmark.py)
- [Метрики](../../benchmarks/go-auth-service/results/metrics.json)
- [Raw JSONL](../../benchmarks/go-auth-service/results/raw)
- [Итоговые проекты](../../benchmarks/go-auth-service/projects)
