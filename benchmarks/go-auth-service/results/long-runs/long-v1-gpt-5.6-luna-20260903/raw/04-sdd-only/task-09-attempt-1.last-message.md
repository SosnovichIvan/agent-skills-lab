Задача 3.1 выполнена.

Добавлено:

- JWT-compatible access tokens из трёх base64url частей;
- HS256 подпись;
- claims `sub`, `email`, `iss`, `iat`, `exp`, `jti`;
- криптографический `jti` через ID generator;
- checkbox `3.1` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.