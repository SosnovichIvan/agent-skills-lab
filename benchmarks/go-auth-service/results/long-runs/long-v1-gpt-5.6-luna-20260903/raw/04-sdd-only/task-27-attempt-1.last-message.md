Задача 7.3 выполнена.

Добавлено:

- permission middleware для organization endpoints;
- проверка актуального membership и roles repository;
- tenant isolation по organization ID;
- JWT claims не содержат roles или permissions;
- checkbox `7.3` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.