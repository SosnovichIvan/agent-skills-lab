# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `3.2`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "3.2",
  "title": "Реализовать строгую валидацию access token",
  "done_when": [
    "проверяются format alg signature issuer expiry",
    "сравнение подписи constant-time",
    "malformed claims безопасно отклоняются"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
после успешной проверки отметь только checkbox 3.2 в OpenSpec tasks.md как [x]

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

