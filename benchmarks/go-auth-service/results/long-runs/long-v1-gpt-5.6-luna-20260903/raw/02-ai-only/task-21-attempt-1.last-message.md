Задача 6.1 выполнена.

Добавлены:

- `Organization` с обязательным `OwnerID`
- `Membership`
- конкурентные organization и membership repositories
- уникальность membership по `(organizationID, userID)`
- CRUD/list операции
- repositories подключены к `App`

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`