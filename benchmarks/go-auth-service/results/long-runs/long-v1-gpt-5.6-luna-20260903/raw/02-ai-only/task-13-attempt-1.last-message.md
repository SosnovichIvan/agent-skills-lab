Задача 4.1 выполнена.

Добавлены:

- opaque refresh token через `crypto/rand`
- SHA-256 hashing для хранения token
- `Session` с `user`, `expiry`, `family` и `revoked`
- конкурентный `SessionRepository`
- поиск session по hash и отзыв session/family
- repository подключён к `App`

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`