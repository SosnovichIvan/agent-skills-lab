Задача 6.2 выполнена.

Добавлено:

- `POST /v1/organizations` с автоматическим owner membership;
- `GET /v1/organizations` только по memberships текущего пользователя;
- tenant ID генерируется сервером и не берётся из body;
- checkbox `6.2` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.