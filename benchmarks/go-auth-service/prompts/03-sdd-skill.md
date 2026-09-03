# Вариант 3: OpenSpec SDD с execution-state

Это новый пустой контекст без истории предыдущих решений.

Явно используй `$execution-state`. Прочитай skill по пути
`{SKILL_PATH}/SKILL.md` и следуй только релевантным ссылкам из него. Работай в
режиме `openspec` с change `auth-service`. Считай находящиеся в проекте
`proposal.md`, `specs/`, `design.md` и `tasks.md` источником истины.

Для чистоты эксперимента других skills нет. В поле `implementation_skill`
используй `$execution-state` как единственный явно подключённый skill, а сам Go-
код реализуй базовыми возможностями coding agent. State helpers запускай из
`{SKILL_PATH}/scripts/` по абсолютному пути. Не выполняй Archive.
