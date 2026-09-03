# Long-session baseline: следующий turn

Продолжай ту же session и сохрани ранее принятые контракты. В этом turn выполни
только задачу `4.3`; не реализуй будущие задания.

Текущая задача:

```json
{
  "id": "4.3",
  "title": "Добавить reuse detection и отзыв refresh token family",
  "done_when": [
    "повторное использование rotated token обнаруживается",
    "вся family отзывается",
    "последующие tokens этой family отклоняются"
  ]
}
```

Повторно открывай `benchmark-requirements.md` или большие исходники только при
необходимости. Не создавай `.execution-state`, `*_test.go` или внешние
зависимости; не используй web, другой agent CLI, commit или push.
OpenSpec отсутствует; не создавай task ledger

Выполни `gofmt`, `go test ./...` и `go vet ./...`, затем остановись.

