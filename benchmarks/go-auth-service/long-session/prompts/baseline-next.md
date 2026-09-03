# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `{{TASK_ID}}`; не реализуй будущие задания.

Текущая задача:

```json
{{TASK_JSON}}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
{{CHECKBOX_GUIDANCE}}

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

