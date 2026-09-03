Задача 6.4 выполнена.

Добавлено:

- `PATCH/DELETE /v1/organizations/{orgID}/members/{userID}`;
- защита последнего owner от удаления или потери роли;
- ownership transfer с атомарным обновлением membership roles;
- checkbox `6.4` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.