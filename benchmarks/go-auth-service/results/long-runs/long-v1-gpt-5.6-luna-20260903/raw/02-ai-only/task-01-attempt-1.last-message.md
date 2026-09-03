Задание 1.1 выполнено.

Созданы:

- `go.mod` с module path `benchmark.local/iam`
- `internal/config` с environment-конфигурацией и валидацией HMAC secret, TTL, rate limits и timeouts
- `internal/app` с dependency wiring
- `cmd/iamd` с HTTP server и graceful shutdown

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`