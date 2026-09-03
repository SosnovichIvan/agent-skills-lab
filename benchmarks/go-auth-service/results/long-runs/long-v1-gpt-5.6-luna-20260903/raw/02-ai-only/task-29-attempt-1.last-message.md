Задача 8.1 выполнена.

Добавлены:

- rate limiting по IP
- rate limiting по authenticated identity
- использование конфигурируемых rate limits
- потокобезопасный idempotency store
- безопасный replay для register и invite
- сохранение исходных status/headers/body при replay

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`