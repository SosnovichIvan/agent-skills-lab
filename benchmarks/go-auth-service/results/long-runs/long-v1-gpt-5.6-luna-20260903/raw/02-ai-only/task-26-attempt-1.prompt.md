# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `7.2`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "7.2",
  "title": "Реализовать назначение и отзыв organization roles",
  "done_when": [
    "PUT/DELETE role assignment реализованы",
    "role и member обязаны принадлежать одному org",
    "последний owner не может потерять owner role"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

