# Long-session baseline: turn 1 из 32

Это начало одной накапливающейся model session. Выполняй задания строго по
одному за turn. В этом turn реализуй только `1.1`; будущие задания не
выполняй.

Прочитай `benchmark-requirements.md` и OpenSpec proposal.md, design.md, specs/iam/spec.md и tasks.md. Сохраняй решения и
контракты для следующих сообщений этой же session.

Текущая задача:

```json
{
  "id": "1.1",
  "title": "Создать go.mod, package layout, config и dependency wiring",
  "done_when": [
    "module path равен benchmark.local/iam",
    "cmd/iamd компилируется",
    "конфигурация валидирует HMAC secret и timeouts"
  ]
}
```

Ограничения:

- не создавай `.execution-state` и `*_test.go`;
- не используй execution-state skill, web, внешние зависимости, другой agent
  CLI, commit или push;
- после успешной проверки отметь только checkbox 1.1 в OpenSpec tasks.md как [x]
- выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

