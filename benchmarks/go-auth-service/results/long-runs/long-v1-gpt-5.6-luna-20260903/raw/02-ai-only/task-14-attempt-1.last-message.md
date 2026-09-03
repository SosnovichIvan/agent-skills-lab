Задача 4.2 выполнена.

Добавлены:

- `POST /v1/token/refresh`
- атомарная refresh-token rotation
- старый token помечается revoked и повторно не принимается
- новая access/refresh пара
- создание initial refresh session/family при успешном login
- хранение только hash refresh token

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`