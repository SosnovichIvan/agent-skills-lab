# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `8.4`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "8.4",
  "title": "Завершить wiring, server timeouts, graceful shutdown и итоговую проверку",
  "done_when": [
    "cmd/iamd связывает все production dependencies",
    "server имеет read write idle timeouts и graceful shutdown",
    "gofmt go test и go vet проходят без test files и внешних modules"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
после успешной проверки отметь только checkbox 8.4 в OpenSpec tasks.md как [x]

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

