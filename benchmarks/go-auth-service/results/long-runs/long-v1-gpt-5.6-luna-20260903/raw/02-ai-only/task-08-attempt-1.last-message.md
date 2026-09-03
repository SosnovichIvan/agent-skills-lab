Задача 2.4 выполнена.

Добавлены:

- общий лимит request body — 1 MiB
- отклонение oversized body с `413 body_too_large`
- strict JSON decoding для неизвестных полей и нескольких JSON-значений
- success envelope для `/healthz` и `/readyz`
- единый error envelope для существующих handlers

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`