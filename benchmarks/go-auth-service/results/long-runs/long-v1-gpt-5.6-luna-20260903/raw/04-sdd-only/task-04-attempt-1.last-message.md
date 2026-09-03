Задача 1.4 выполнена.

Добавлено:

- конкурентный `UserRepository` с `sync.RWMutex`;
- уникальный индекс по normalized email;
- deep-copy пользователей и `PasswordMaterial` при сохранении и выдаче;
- операции create/get/find/update/delete;
- checkbox `1.4` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.