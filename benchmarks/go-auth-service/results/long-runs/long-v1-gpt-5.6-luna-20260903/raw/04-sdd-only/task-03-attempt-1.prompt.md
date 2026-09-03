# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `1.3`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "1.3",
  "title": "Добавить модели пользователя и безопасный публичный профиль",
  "done_when": [
    "User отделяет password material от public profile",
    "email и verification state представлены явно",
    "JSON никогда не сериализует password material"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
после успешной проверки отметь только checkbox 1.3 в OpenSpec tasks.md как [x]

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

