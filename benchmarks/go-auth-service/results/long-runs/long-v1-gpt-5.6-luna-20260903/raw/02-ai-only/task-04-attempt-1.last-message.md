Задача 1.4 выполнена.

Добавлен конкурентный `UserRepository`:

- синхронизация через `sync.RWMutex`
- уникальность normalized email
- безопасные deep-copy при сохранении и возврате пользователей
- CRUD и поиск по email
- типизированные ошибки для conflict, invalid и not found

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`