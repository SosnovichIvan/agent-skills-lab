Задача 5.4 исправлена.

Теперь явно реализованы:

- `failedLoginCounter` по normalized identity
- конфигурируемые threshold и lock duration
- временная блокировка после порога
- reset failed login counter после успешного login
- потокобезопасная синхронизация

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`