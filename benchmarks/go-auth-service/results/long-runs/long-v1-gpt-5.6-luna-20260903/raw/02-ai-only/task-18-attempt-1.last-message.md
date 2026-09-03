Задача 5.2 выполнена.

Добавлены:

- `POST /v1/password-reset/request` без раскрытия существования email
- opaque reset token с хранением только hash
- одноразовый `Consume`
- `POST /v1/password-reset/confirm`
- смена пароля и отзыв всех refresh sessions
- проверка expiry reset token

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`