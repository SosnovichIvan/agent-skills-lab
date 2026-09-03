Задача 1.3 выполнена.

Добавлено:

- `User` с отдельным `PasswordMaterial`;
- явные `Email` и `VerificationState`;
- безопасный `PublicProfile`;
- JSON-теги, исключающие password material из сериализации;
- checkbox `1.3` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.