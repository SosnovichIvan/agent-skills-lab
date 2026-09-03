Задание 1.1 выполнено.

Добавлено:

- `go.mod` с module path `benchmark.local/iam`
- package layout: `internal/config`, `internal/app`, `cmd/iamd`
- environment-конфигурация с проверкой HMAC secret ≥ 32 байт и положительных timeouts
- HTTP server wiring с graceful shutdown
- checkbox `1.1` отмечен как `[x]`

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`

Запрещённые файлы не создавались.