Задача 1.2 выполнена.

Добавлено:

- криптографический генератор доменных ID через `crypto/rand`;
- интерфейс `Clock` и `RealClock`;
- явная передача `Clock` и `IDGenerator` через application dependencies;
- типизированные ошибки: `not_found`, `conflict`, `invalid`, `unauthorized`;
- checkbox `1.2` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.