# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `1.2`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "1.2",
  "title": "Добавить доменные ID, clock abstraction и типизированные ошибки",
  "done_when": [
    "ID генерируются через crypto/rand",
    "доменная логика получает Clock через dependency",
    "ошибки различают not found, conflict, invalid и unauthorized"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

