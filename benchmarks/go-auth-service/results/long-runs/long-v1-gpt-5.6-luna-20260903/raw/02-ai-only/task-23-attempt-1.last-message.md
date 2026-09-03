Задача 6.3 выполнена.

Добавлены:

- opaque expiring invite tokens
- хранение invite token только как hash
- одноразовый invite consume
- invite creation с membership-проверкой организации
- accept с проверкой email пользователя
- member list только для участника той же организации
- защита от повторного создания membership

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`