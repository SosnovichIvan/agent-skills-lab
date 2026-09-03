Задача 3.3 выполнена.

Добавлено:

- `POST /v1/login`;
- проверка credentials и выдача HS256 access token;
- единый `401 unauthorized` для неизвестного email и неверного пароля;
- generic auth error без раскрытия существования email;
- checkbox `3.3` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.