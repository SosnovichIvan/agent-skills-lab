Задача 4.2 выполнена.

Добавлено:

- `POST /v1/token/refresh`;
- rotation refresh token;
- атомарная замена session и отзыв старого token;
- выдача новой access/refresh пары;
- login теперь создаёт initial refresh session;
- checkbox `4.2` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.