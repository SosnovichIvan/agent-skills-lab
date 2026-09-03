Задача 8.2 выполнена.

Добавлено:

- tamper-evident audit repository с SHA-256 hash chain;
- audit events для security mutations;
- redacted metadata без секретов;
- `GET /v1/organizations/{orgID}/audit`;
- bounded cursor pagination;
- checkbox `8.2` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.