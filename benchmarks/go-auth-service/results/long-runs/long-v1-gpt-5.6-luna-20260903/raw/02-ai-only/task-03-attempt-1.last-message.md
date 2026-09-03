Задача 1.3 выполнена.

Добавлены:

- `User` с отдельным `PasswordMaterial`
- явные `Email` и `EmailVerificationState`
- `PublicProfile` без credential-полей
- JSON-теги, исключающие password material из сериализации

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`