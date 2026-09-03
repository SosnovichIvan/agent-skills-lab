Задача 6.2 выполнена.

Добавлены:

- `POST /v1/organizations`
- автоматическое создание owner membership
- `GET /v1/organizations`
- выдача только организаций из memberships текущего пользователя
- server-side генерация organization ID
- body не принимает tenant ID

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`