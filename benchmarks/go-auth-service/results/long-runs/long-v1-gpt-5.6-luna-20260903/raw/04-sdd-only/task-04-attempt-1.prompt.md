# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `1.4`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "1.4",
  "title": "Реализовать конкурентный user repository и email index",
  "done_when": [
    "repository использует синхронизацию",
    "normalized email уникален",
    "возвращаемые значения не позволяют менять внутреннее состояние без lock"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
после успешной проверки отметь только checkbox 1.4 в OpenSpec tasks.md как [x]

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

