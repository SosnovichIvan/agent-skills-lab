Задача 7.4 выполнена.

Добавлено:

- create/list/revoke API-key endpoints;
- raw key возвращается только при создании;
- repository хранит только SHA-256 hash;
- scopes ограничиваются известными permissions и organization tenant;
- checkbox `7.4` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.