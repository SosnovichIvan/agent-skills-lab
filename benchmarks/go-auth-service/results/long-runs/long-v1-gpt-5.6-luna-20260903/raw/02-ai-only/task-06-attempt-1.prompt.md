# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `2.2`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "2.2",
  "title": "Реализовать password KDF и constant-time verification",
  "done_when": [
    "использованы salt и итеративный HMAC-SHA256",
    "проверка использует subtle.ConstantTimeCompare",
    "открытый пароль не хранится"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

