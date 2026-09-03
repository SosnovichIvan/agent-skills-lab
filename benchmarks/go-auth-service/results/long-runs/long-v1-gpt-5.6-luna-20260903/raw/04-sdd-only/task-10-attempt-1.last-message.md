Задача 3.2 выполнена.

Добавлено:

- строгая проверка формата JWT и base64url частей;
- проверка `alg=HS256`, подписи, issuer и expiry;
- constant-time сравнение подписи;
- безопасное отклонение malformed claims с generic-ошибкой;
- checkbox `3.2` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.