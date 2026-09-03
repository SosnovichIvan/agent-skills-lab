# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `4.1`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "4.1",
  "title": "Реализовать opaque refresh tokens и session repository",
  "done_when": [
    "refresh token создаётся через crypto/rand",
    "хранится только SHA-256 hash",
    "session содержит user expiry family и revoked state"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
после успешной проверки отметь только checkbox 4.1 в OpenSpec tasks.md как [x]

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

