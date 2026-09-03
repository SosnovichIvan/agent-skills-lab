Задача 2.4 исправлена.

Теперь:

- envelopes представлены явными типами `DataEnvelope` и `ErrorEnvelope`
- error envelope содержит `code`, `message`, `request_id`
- `X-Request-ID` добавляется к ответам
- body ограничен 1 MiB
- strict JSON decoder отклоняет unknown fields и несколько JSON-значений
- существующие handlers используют общий envelope

Проверки успешно пройдены:

- `gofmt`
- `go test ./...`
- `go vet ./...`