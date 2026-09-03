# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `6.2`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "6.2",
  "title": "Реализовать создание и просмотр организаций",
  "done_when": [
    "POST organizations создаёт owner membership",
    "GET organizations возвращает только memberships пользователя",
    "tenant IDs не доверяются из request body"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

