Задача 8.2 выполнена.

Добавлены:

- tamper-evident audit log с SHA-256 hash chain
- audit events для security и organization mutations
- redacted event model без секретов
- `GET /v1/organizations/{orgID}/audit`
- bounded cursor pagination с limit до 100
- tenant и `audit.read` permission checks

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`