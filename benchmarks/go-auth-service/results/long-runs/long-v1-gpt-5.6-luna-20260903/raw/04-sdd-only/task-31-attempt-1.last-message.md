Задача 8.3 выполнена.

Добавлено:

- request ID в `X-Request-ID`, error envelope и structured JSON logs;
- логи без URL, body и secret material;
- `/healthz`, `/readyz`, `/metrics`;
- monotonic request/error counters;
- checkbox `8.3` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.