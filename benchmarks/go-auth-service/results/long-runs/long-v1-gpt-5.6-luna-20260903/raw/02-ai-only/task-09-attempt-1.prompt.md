# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `3.1`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "3.1",
  "title": "Реализовать создание JWT-compatible HS256 access token",
  "done_when": [
    "token имеет три base64url части",
    "claims содержат sub email iss iat exp jti",
    "jti генерируется криптографически"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

