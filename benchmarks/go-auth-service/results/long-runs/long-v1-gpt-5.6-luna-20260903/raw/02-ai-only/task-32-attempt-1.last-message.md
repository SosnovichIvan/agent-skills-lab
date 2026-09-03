Задача 8.4 выполнена.

Проверено:

- `cmd/iamd` связывает все repositories, security, audit, metrics и logging dependencies
- HTTP server настроен с read/write/idle timeouts
- graceful shutdown по SIGINT/SIGTERM
- только стандартная библиотека, без test files и внешних modules

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`