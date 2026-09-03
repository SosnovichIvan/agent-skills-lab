# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `4.2`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "4.2",
  "title": "Реализовать POST /v1/token/refresh с rotation",
  "done_when": [
    "валидный refresh token заменяется новым",
    "старый token становится использованным",
    "ответ возвращает новую access/refresh пару"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
после успешной проверки отметь только checkbox 4.2 в OpenSpec tasks.md как [x]

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

