Задача 6.1 выполнена.

Добавлено:

- модели `Organization` с обязательным `OwnerID` и `Membership`;
- конкурентные organization и membership repositories;
- уникальность membership по `(organizationID, userID)`;
- checkbox `6.1` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.