# Long-session baseline: turn 1 из 32

Это начало одной накапливающейся model session. Выполняй задания строго по
одному за turn. В этом turn реализуй только `{{TASK_ID}}`; будущие задания не
выполняй.

Прочитай `benchmark-requirements.md` и {{SOURCE_GUIDANCE}}. Сохраняй решения и
контракты для следующих сообщений этой же session.

Текущая задача:

```json
{{TASK_JSON}}
```

Ограничения:

- не создавай `.execution-state` и `*_test.go`;
- не используй execution-state skill, web, внешние зависимости, другой agent
  CLI, commit или push;
- {{CHECKBOX_GUIDANCE}}
- выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

