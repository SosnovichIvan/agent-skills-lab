Задача 7.1 выполнена.

Добавлены:

- встроенные роли `owner`, `admin`, `viewer`
- фиксированные permission sets
- custom roles с обязательным непустым permission set
- обязательная принадлежность role к organization
- конкурентный `RoleRepository` с защитой permission maps
- repository подключён к `App`

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`