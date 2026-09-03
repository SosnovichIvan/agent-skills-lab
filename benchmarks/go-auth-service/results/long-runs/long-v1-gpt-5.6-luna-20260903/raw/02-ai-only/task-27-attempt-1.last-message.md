Задача 7.3 выполнена.

Добавлены:

- permission middleware
- проверка актуальных memberships и role assignments из repositories
- tenant isolation для organization endpoints
- permissions не помещаются в access token
- доступ к routes ограничен соответствующими permissions

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`