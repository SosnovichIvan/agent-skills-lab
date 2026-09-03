# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `8.2`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "8.2",
  "title": "Добавить tamper-evident audit log и pagination",
  "done_when": [
    "security mutations создают audit events",
    "events связаны hash chain",
    "audit endpoint поддерживает bounded cursor pagination и redaction"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
после успешной проверки отметь только checkbox 8.2 в OpenSpec tasks.md как [x]

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

