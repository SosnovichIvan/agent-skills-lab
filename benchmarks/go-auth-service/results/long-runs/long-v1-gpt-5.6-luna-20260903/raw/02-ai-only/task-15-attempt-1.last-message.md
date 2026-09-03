Задача 4.3 выполнена.

Добавлены:

- detection повторного использования revoked refresh token
- атомарный отзыв всей token family
- отклонение последующих tokens отозванной family
- защита от race condition при параллельной rotation

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`