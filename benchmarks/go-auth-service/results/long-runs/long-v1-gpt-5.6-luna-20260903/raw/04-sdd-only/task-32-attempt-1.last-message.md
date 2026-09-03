Задача 8.4 выполнена.

Проверено:

- `cmd/iamd` связывает production dependencies;
- HTTP server использует read/write/idle timeouts;
- graceful shutdown по SIGINT/SIGTERM;
- только стандартная библиотека;
- test files и `.execution-state` отсутствуют;
- checkbox `8.4` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.