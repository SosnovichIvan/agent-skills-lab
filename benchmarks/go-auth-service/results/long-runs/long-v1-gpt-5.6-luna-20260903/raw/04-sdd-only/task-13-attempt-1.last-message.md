Задача 4.1 выполнена.

Добавлено:

- opaque refresh token через `crypto/rand`;
- SHA-256 hashing без хранения raw token;
- модель session с user, expiry, family и revoked state;
- конкурентный `SessionRepository` с безопасными копиями;
- checkbox `4.1` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.