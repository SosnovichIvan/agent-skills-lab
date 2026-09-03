Задача 5.4 выполнена.

Добавлены:

- login failure tracking по normalized identity
- конфигурация `IAM_LOGIN_MAX_FAILURES`
- конфигурация `IAM_LOGIN_LOCK_DURATION`
- временная блокировка после порога ошибок
- сброс счётчика после успешного login
- потокобезопасный login guard

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`