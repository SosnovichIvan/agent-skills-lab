# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `7.3`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "7.3",
  "title": "Добавить permission middleware и tenant isolation",
  "done_when": [
    "permissions читаются из актуального repository",
    "access token не содержит roles",
    "каждый organization endpoint проверяет org membership"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

