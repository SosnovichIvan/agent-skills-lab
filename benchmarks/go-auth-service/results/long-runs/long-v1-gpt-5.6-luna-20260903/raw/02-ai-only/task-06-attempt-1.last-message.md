Задача 2.2 выполнена.

Добавлены:

- salt через `crypto/rand`
- итеративный HMAC-SHA256 KDF
- проверка через `subtle.ConstantTimeCompare`
- хранение только `PasswordMaterial`, без открытого пароля
- `HashPassword`, `VerifyPassword` и `CheckPassword`

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`