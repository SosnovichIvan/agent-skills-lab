# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `4.4`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "4.4",
  "title": "Добавить logout, logout-all и session management",
  "done_when": [
    "реализованы logout и logout-all",
    "реализованы GET sessions и DELETE session",
    "пользователь не может управлять чужой session"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

