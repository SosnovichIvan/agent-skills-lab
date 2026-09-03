# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `6.4`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "6.4",
  "title": "Реализовать изменение, удаление member и ownership transfer",
  "done_when": [
    "реализованы PATCH/DELETE member",
    "последний owner защищён",
    "ownership transfer выполняется атомарно"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
после успешной проверки отметь только checkbox 6.4 в OpenSpec tasks.md как [x]

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

