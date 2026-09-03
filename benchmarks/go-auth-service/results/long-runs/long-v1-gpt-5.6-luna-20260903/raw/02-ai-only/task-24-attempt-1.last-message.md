Задача 6.4 выполнена.

Добавлены:

- `PATCH /v1/organizations/{orgID}/members/{userID}`
- `DELETE /v1/organizations/{orgID}/members/{userID}`
- `POST /v1/organizations/{orgID}/ownership-transfer`
- защита последнего owner
- атомарная membership ownership transfer под lock
- проверка owner и membership по tenant boundary

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`