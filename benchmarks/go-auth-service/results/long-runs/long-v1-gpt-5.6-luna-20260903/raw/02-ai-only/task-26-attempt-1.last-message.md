Задача 7.2 выполнена.

Добавлены:

- PUT role assignment
- DELETE role assignment
- проверка принадлежности role и member одной организации
- защита последнего owner role
- конкурентный `RoleAssignmentRepository`
- role assignment wiring в `App`

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`