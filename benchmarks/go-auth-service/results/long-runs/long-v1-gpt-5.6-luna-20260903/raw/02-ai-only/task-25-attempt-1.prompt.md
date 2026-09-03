# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `7.1`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "7.1",
  "title": "Добавить встроенные и custom organization roles",
  "done_when": [
    "owner admin viewer имеют фиксированные permissions",
    "custom role имеет непустой permission set",
    "roles принадлежат одной организации"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

