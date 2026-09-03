Задача 7.2 выполнена.

Добавлено:

- `PUT/DELETE /v1/organizations/{orgID}/members/{userID}/roles/{roleID}`;
- проверка принадлежности role и member одной организации;
- защита последнего owner от потери owner role;
- автоматическое создание built-in roles для новых организаций;
- checkbox `7.2` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.