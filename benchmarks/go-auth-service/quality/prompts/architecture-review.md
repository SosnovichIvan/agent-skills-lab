# Independent architecture checkpoint

Ты новый reviewer без transcript предыдущих workers. Не реализуй новую feature.

Прочитай `benchmark-requirements.md`, state и compact context map:

- state: `{{STATE_PATH}}`
- причины review: {{REVIEW_REASONS}}

Проверь wiring, dependency direction, согласованность моделей и публичных
контрактов, отсутствие дублирующих реализаций, чрезмерно выросшие файлы и
сквозные связи между затронутыми подсистемами. Выполни `gofmt -l`,
`go test ./...` и `go vet ./...`. Не читай raw model logs, не меняй
`.execution-state`, не используй web, другой agent CLI, commit или push.

Если необходим исправляющий chunk, не исправляй его скрыто: перечисли точный
blocker. Иначе верни короткий итог и конкретные выполненные проверки. Не
возвращай chain-of-thought.
