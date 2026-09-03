# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `6.3`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "6.3",
  "title": "Реализовать invite, accept и просмотр участников",
  "done_when": [
    "invite token opaque hashed и expiring",
    "accept создаёт membership один раз",
    "member list требует membership в той же организации"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
после успешной проверки отметь только checkbox 6.3 в OpenSpec tasks.md как [x]

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

