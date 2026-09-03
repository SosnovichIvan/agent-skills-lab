# Вариант 3: OpenSpec SDD с execution-state v3

Это независимый запуск в новом model context. Явно используй `execution-state`:
прочитай `{SKILL_PATH}/SKILL.md` как инструкции skill. Источник требований —
`openspec`, change — `auth-service`, профиль реализации — `base-agent`.

У change один связный итоговый продукт; весь Apply вместе с compile-check
образует один semantic chunk, handoff не ожидается. Примени правила
маршрутизации skill. Если выбран `passthrough`, не создавай `.execution-state`,
не читай execution-state references, prompts или исходники helpers, а выполни
обычный OpenSpec Apply по `proposal.md`, `specs/`, `design.md` и `tasks.md`.
Отмечай checkbox только после выполнения соответствующей работы и проверки.

Не выполняй Archive и не запускай другой agent CLI.
