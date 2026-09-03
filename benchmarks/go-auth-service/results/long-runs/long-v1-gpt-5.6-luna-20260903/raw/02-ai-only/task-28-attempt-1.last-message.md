Задача 7.4 выполнена.

Добавлены:

- create/list/revoke API key endpoints
- crypto-random opaque API keys
- хранение только SHA-256 hash
- scoped permissions для API keys
- tenant isolation по `OrganizationID`
- API keys поддерживают `Authorization: ApiKey ...`
- raw key возвращается только при создании

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`