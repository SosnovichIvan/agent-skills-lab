# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `3.4`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "3.4",
  "title": "Добавить Bearer middleware и GET /v1/me",
  "done_when": [
    "middleware извлекает Bearer token",
    "claims передаются через context",
    "GET /v1/me требует валидный token"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

