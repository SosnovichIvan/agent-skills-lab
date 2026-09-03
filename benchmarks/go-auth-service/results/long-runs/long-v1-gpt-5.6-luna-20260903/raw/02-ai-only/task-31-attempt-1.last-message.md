Задача 8.3 выполнена.

Добавлены:

- structured JSON logs через `log/slog`
- `X-Request-ID` в response и logs
- redaction invite tokens из logged paths
- monotonic atomic metrics
- `GET /metrics`
- существующие `/healthz` и `/readyz` сохранены

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`