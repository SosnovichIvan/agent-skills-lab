# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `3.3`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "3.3",
  "title": "Реализовать POST /v1/login и единые auth errors",
  "done_when": [
    "валидные credentials возвращают access token",
    "неверные credentials возвращают одинаковую unauthorized ошибку",
    "ответ не раскрывает существование email"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

