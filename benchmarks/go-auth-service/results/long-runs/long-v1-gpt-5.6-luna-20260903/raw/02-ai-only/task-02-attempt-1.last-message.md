Задача 1.2 выполнена.

Добавлены:

- доменные ID с генерацией через `crypto/rand`
- `domain.Clock` и `SystemClock`
- dependency injection Clock через `app.NewWithClock`
- типизированные ошибки: `not found`, `conflict`, `invalid`, `unauthorized`

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`