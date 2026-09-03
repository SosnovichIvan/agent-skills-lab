# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `2.4`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "2.4",
  "title": "Добавить строгий JSON decoder, body limits и общий envelope",
  "done_when": [
    "unknown fields отклоняются",
    "body ограничен по размеру",
    "success/error envelope применяется к существующим handlers"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

