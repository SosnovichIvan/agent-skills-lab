Задача 3.2 выполнена.

Добавлена строгая валидация access token:

- проверка формата из трёх частей и Base64URL
- проверка `alg=HS256` и `typ=JWT`
- проверка подписи через `subtle.ConstantTimeCompare`
- проверка обязательных claims, issuer и expiry
- отклонение malformed claims и trailing JSON

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`