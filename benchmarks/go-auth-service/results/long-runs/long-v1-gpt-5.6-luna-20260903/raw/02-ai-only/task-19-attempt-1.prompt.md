# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `5.3`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "5.3",
  "title": "Реализовать email verification request и confirm",
  "done_when": [
    "verification token одноразовый и хранится как hash",
    "confirm отмечает email verified",
    "повторное подтверждение идемпотентно"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

