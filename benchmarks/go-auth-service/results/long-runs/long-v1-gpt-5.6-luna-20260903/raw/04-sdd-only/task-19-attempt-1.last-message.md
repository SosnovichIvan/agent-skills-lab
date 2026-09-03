Задача 5.3 выполнена.

Добавлено:

- email verification request/confirm endpoints;
- одноразовые verification tokens, хранящиеся только как hash;
- установка `email_verified`;
- идемпотентное повторное подтверждение;
- checkbox `5.3` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.