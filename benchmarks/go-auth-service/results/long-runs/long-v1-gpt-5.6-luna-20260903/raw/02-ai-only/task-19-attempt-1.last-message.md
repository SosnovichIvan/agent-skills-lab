Задача 5.3 выполнена.

Добавлены:

- email verification token через crypto/rand
- хранение token только как hash
- одноразовый atomic consume
- `POST /v1/email-verification/request`
- `POST /v1/email-verification/confirm`
- явное состояние `EmailVerified`
- идемпотентное повторное подтверждение

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`