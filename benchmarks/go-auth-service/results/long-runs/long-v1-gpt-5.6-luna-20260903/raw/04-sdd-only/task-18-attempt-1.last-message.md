Задача 5.2 выполнена.

Добавлено:

- password reset request с одинаковым ответом для существующего и неизвестного email;
- opaque reset token, хранящийся только как SHA-256 hash;
- атомарное одноразовое consume с expiry;
- смена пароля и отзыв всех refresh sessions;
- checkbox `5.2` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.