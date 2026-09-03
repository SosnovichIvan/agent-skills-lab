Задача 2.4 выполнена.

Добавлено:

- строгий JSON decoder с запретом неизвестных полей и нескольких JSON-значений;
- лимит request body в 1 MiB;
- общие `data/error` envelopes;
- регистрационный handler переведён на общие HTTP helpers;
- checkbox `2.4` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.