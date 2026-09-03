Задача 3.4 выполнена.

Добавлены:

- Bearer middleware с проверкой Authorization header
- валидация access token
- передача claims через request context
- защищённый `GET /v1/me`
- безопасный public profile в success envelope
- unauthorized envelope для отсутствующего или невалидного token

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`