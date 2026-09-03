# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `5.2`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "5.2",
  "title": "Реализовать password reset request и confirm",
  "done_when": [
    "request не раскрывает существование email",
    "reset token хранится только как hash и одноразовый",
    "confirm меняет пароль и отзывает sessions"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

