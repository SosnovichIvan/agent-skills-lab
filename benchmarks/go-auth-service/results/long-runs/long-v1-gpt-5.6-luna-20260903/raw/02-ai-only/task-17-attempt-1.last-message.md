Задача 5.1 выполнена.

Добавлен защищённый `POST /v1/password/change`:

- проверяет текущий пароль
- валидирует новый пароль через policy
- сохраняет только новый password hash
- отзывает все refresh sessions пользователя после успеха
- возвращает public profile без credential material

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`