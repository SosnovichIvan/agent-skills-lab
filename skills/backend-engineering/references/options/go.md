# Go backend

Читайте при `go.mod` в изменяемом сервисе.

- Сохраняй module/workspace и package boundaries проекта; не вводи framework,
  ORM или DI container без подтверждённой необходимости.
- Передавай `context.Context` через I/O path. Timeout создавай на границе
  конкретного внешнего вызова и всегда вызывай cancel.
- Оборачивай ошибки с контекстом через `%w`, классифицируй через `errors.Is/As`.
- Не запускай goroutine без владельца, cancellation и обработки panic/error.
- Защищай shared state mutex/atomic/channel согласно существующей модели; не
  делай copy объекта с mutex.
- Закрывай response bodies, rows, files и transactions по всем путям.
- Следуй выбранному HTTP router/gRPC/message framework из `go.mod` и соседнего
  кода; не подменяй его стандартной библиотекой или наоборот локально.
- Выполни `gofmt`, generation check, `go vet`, package tests и релевантный
  `go test -race -count=1`, если среда и стоимость позволяют.
