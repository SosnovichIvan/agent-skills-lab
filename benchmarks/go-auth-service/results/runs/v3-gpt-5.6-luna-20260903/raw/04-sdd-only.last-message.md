Готово.

- Реализован Go HTTP auth-сервис без внешних зависимостей.
- Добавлены `/register`, `/login`, защищённый `/me`.
- Реализованы in-memory repository, KDF HMAC-SHA256, HS256 JWT-compatible tokens, Bearer middleware.
- Добавлены env-конфигурация, timeouts и graceful shutdown.
- Все задачи в [`tasks.md`](openspec/changes/auth-service/tasks.md) отмечены выполненными.
- Выполнены `gofmt` и `GOCACHE=/tmp/auth-service-gocache go test ./...` — сборка успешна, тестовых файлов нет.

Основные файлы: [`main.go`](cmd/auth-service/main.go), [`transport.go`](internal/auth/transport.go), [`token.go`](internal/auth/token.go), [`password.go`](internal/auth/password.go).