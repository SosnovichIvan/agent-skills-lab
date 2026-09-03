Задача 4.4 выполнена.

Добавлены:

- `POST /v1/logout`
- `POST /v1/logout-all`
- `GET /v1/sessions`
- `DELETE /v1/sessions/{sessionID}`
- проверка владения session через authenticated user claims
- безопасные session views без refresh-token hash

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`