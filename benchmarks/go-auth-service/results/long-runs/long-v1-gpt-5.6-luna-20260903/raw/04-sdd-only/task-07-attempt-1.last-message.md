Задача 2.3 выполнена.

Добавлено:

- `POST /v1/register`;
- создание пользователя с normalized email и password material;
- conflict при повторном email;
- ответ только с `PublicProfile` в `{"data": ...}`;
- checkbox `2.3` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.