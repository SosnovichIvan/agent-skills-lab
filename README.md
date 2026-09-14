# Agent Skills Lab

Этот репозиторий — рабочее пространство для разработки и развития навыков (skills) и инструкций для агентов. Его цель — сделать работу агентов более предсказуемой, эффективной и удобной: собирать переиспользуемые навыки, правила и практики в одном месте.

## Структура

- [skills](skills/README.md) — навыки, расширяющие возможности агентов.
- [agents](agents/README.md) — правила, роли и инструкции для агентов.
- [`skills/catalog.json`](skills/catalog.json) — каталог skills, adapters,
  выбираемых references и зависимостей.
- [`install.py`](install.py) — интерактивная и non-interactive установка в
  целевой проект.
- [`tests`](tests) — проверки установщика и runtime skills.
- [`benchmarks`](benchmarks/go-auth-service/quality/README.md) — воспроизводимый
  quality/long-session эксперимент для `execution-state`.

## Требования

- Python `3.9+`; сторонние Python-пакеты для установщика не нужны.
- Локальная копия этого репозитория.
- Целевой проект и право записи в выбранные каталог skills и agent-файл.
- Для reset-профиля `execution-state` — установленный CLI целевого агента. Без
  подтверждённого CLI skill использует безопасный manual/lite fallback.

Установщик не устанавливает Codex, Claude Code, Gemini CLI, Python или
зависимости целевого проекта.

## Установка skills

Запусти интерактивный установщик из корня репозитория:

```bash
python3 install.py
```

Установщик:

1. показывает доступные skills с описаниями;
2. позволяет выбрать один или несколько skills;
3. для модульного skill отдельно предлагает языки, frameworks и структуру;
4. спрашивает путь проекта и папку размещения skills;
5. спрашивает путь до файла инструкций агента;
6. собирает каждый skill только с выбранными references и регистрирует его в
   управляемом блоке agent-файла.

Установщик поддерживает native adapters `codex`, `claude`, `gemini` и `generic`.
Их можно выбрать интерактивно или флагом `--agent`. Если `execution-state`
выбран напрямую, по умолчанию каждая задача проходит его adaptive routing;
короткие задачи продолжаются через `passthrough` без state.

| Adapter | Skills по умолчанию | Agent-файл | Runtime enforcement |
| --- | --- | --- | --- |
| `codex` | `.agents/skills` | `AGENTS.md` | `instructions` или trusted project hooks в `strict` |
| `claude` | `.claude/skills` | `CLAUDE.md` | только `instructions` |
| `gemini` | `.gemini/skills` | `GEMINI.md` | только `instructions` |
| `generic` | задаётся пользователем | задаётся пользователем | только `instructions` |

Пути можно переопределить через `--skills-dir` и `--agent-file`. Для CLI,
версия которого использует другие точки обнаружения, выбери `generic` и укажи
пути из его документации.

Reference нельзя установить отдельно: он объявлен в
[`skills/catalog.json`](skills/catalog.json), принадлежит ровно одному skill и
копируется только вместе с ним. Зависимые профили добавляются автоматически:
например, Next.js также устанавливает React.

Каждый пользовательский skill в каталоге устанавливается самостоятельно и не
требует другого skill. Выбор нескольких skills означает только их установку и
регистрацию в agent-файле: порядок применения, ответственность каждого шага и
формат передачи результата задаёт внешний prompt задачи. Например,
`ui-ux-design` может подготовить design contract, а `frontend-engineering` —
реализовать его, но ни один из них не запускает другой автоматически.

Поле `requires_skills` поддерживается manifest schema v2 только как
низкоуровневый механизм для будущей неразделимой runtime-зависимости. Его не
следует применять для композиции рабочих навыков. Установщик умеет разрешать
такой dependency graph и отклонять циклы. Manifest сохраняет installer/skill
version, dependency origin, выбранный agent adapter, execution policy и SHA-256
фактически собранного skill.

Область действия execution policy:

| Scope | Когда применяется `execution-state` |
| --- | --- |
| `explicit` | Только при явном вызове пользователем. |
| `dependent_tasks` | Совместимость с техническими runtime-зависимостями из manifest. |
| `all_tasks` | Для каждой задачи; adaptive router может вернуть `passthrough`. |

`instructions` записывает обязательное правило в agent-файл, но его соблюдение
остаётся ответственностью агента. `strict` дополнительно блокирует tool calls в
Codex до успешного `statectl route`; этот режим доступен только с adapter
`codex`, scope `all_tasks`, включённой функцией hooks и доверенной project
configuration. Он не является административной или неизменяемой политикой.

Для `frontend-engineering` структура обязательна и выбирается ровно одна:
`structure-fsd`, `structure-flexible` или собственный файл через
`--custom-structure`. Framework при этом можно не выбирать.

Для CI или повторяемой установки доступен non-interactive режим:

```bash
python3 install.py \
  --skill frontend-engineering \
  --option frontend-engineering:typescript \
  --option frontend-engineering:react \
  --option frontend-engineering:structure-fsd \
  --project-dir /path/to/project \
  --skills-dir .agents/skills \
  --agent-file AGENTS.md \
  --yes
```

Обязательная маршрутизация и строгий Codex guard:

```bash
python3 install.py \
  --skill execution-state \
  --agent codex \
  --execution-state-policy all_tasks \
  --enforcement strict \
  --project-dir /path/to/project \
  --yes
```

В режиме `strict` проверь и доверь установленные project hooks через `/hooks`.
Для остальных adapters доступен переносимый режим `instructions`.

Вместо встроенной структуры можно передать собственный UTF-8 файл:

```bash
python3 install.py \
  --skill frontend-engineering \
  --option frontend-engineering:typescript \
  --option frontend-engineering:react \
  --custom-structure frontend-engineering=/path/to/frontend-structure.md \
  --project-dir /path/to/project \
  --yes
```

Посмотреть каталог без установки можно командой `python3 install.py --list`, а
проверить выбор и пути без записи — с флагом `--dry-run`. При повторной установке
прежняя папка skill сохраняется вне discoverable skills-directory в
`.agent-skills-lab/backups/`.

## Что изменяется в целевом проекте

Обычная установка создаёт или обновляет:

- `<skills-dir>/<skill>/` — собранный skill только с выбранными references;
- `<skills-dir>/.agent-skills-lab.json` — manifest установки и checksums;
- выбранный agent-файл — управляемые Markdown-блоки со списком skills и
  execution policy;
- `.agent-skills-lab/policy.json` — machine-readable project policy.

Строгая установка Codex дополнительно создаёт `.codex/hooks.json`,
`.codex/hooks/execution_state_policy.py` и
`.agent-skills-lab/codex-strict-context.md`. Во время работы receipts появляются
в `.agent-skills-lab/runtime/`. Установщик сохраняет прежние версии skills в
`.agent-skills-lab/backups/` и откатывает текущую операцию при ошибке записи.

Для общей командной политики обычно коммитят собранные skills, manifest,
agent-файл, `policy.json` и `.codex/` strict-конфигурацию. Runtime receipts и
локальные backups коммитить не следует:

```gitignore
.agent-skills-lab/runtime/
.agent-skills-lab/backups/
```

Повторная установка заменяет только управляемые блоки и hooks этого проекта.
Автоматического удаления всех установленных artifacts пока нет.

## Проверка репозитория

```bash
python3 -m unittest discover -s tests -p 'test*.py'
python3 -m unittest discover -s tests/execution-state -p 'test*.py'
python3 -m unittest discover -s benchmarks/go-auth-service/quality -p 'test*.py'
```

Полный текущий набор — 108 тестов. Для нового или изменённого skill также запусти
`quick_validate.py skills/<skill-name>` из системного `skill-creator` и smoke-test
установки во временный проект.

Каталог включает 14 skills. Для `database-engineering` и `database-review`
выбирается минимум один профиль: PostgreSQL, MySQL/InnoDB или SQLite.
Review skills работают независимо от skills реализации. Тексты, архитектура,
логотипы и иконки имеют отдельные точки входа в [каталоге](skills/README.md).

Поведенческие задачи для Luna/Terra, подготовка изолированных пакетов и
ограничения первой проверки описаны в
[benchmarks/skills](benchmarks/skills/README.md). Unit tests проверяют упаковку
и runtime; они не заменяют оценку результата работы модели.
Пример композиции независимых skills находится во внешнем
[prompt](workflows/prompts/product-change.md).

## Правила работы с Git

- Ветки `main` и `develop` защищены: прямые push запрещены.
- Изменения попадают в эти ветки только через pull request.
- Pull request, созданный владельцем репозитория, может быть объединён с правом bypass.
