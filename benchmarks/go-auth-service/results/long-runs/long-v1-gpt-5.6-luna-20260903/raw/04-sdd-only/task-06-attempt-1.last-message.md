Задача 2.2 выполнена.

Добавлено:

- итеративный HMAC-SHA256 KDF со случайной солью;
- `subtle.ConstantTimeCompare` для проверки;
- хранение только derived password material;
- checkbox `2.2` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.