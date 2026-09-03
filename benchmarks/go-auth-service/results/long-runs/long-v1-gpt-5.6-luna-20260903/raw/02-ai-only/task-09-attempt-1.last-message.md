Задача 3.1 выполнена.

Добавлен HS256 access-token issuer:

- три Raw Base64URL части
- claims `sub`, `email`, `iss`, `iat`, `exp`, `jti`
- криптографически случайный `jti`
- подпись HMAC-SHA256
- `Clock` используется через dependency injection
- issuer подключён к `App`

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`