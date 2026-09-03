# Вариант 1: execution-state v3 без SDD

Это независимый запуск в пустом проекте и новом model context. Явно используй
`execution-state`: прочитай `{SKILL_PATH}/SKILL.md` как инструкции skill.

Источник требований — `standalone`, профиль реализации — `base-agent`. У
задания один итоговый продукт, реализация и compile-check образуют один
semantic chunk, handoff не ожидается. Примени правила маршрутизации skill. Если
выбран `passthrough`, не создавай `.execution-state`, не читай references,
prompts или исходники helpers и сразу реализуй задание базовыми возможностями.

OpenSpec не создавай. Не запускай другой agent CLI.
