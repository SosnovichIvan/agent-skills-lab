Готово. Реализован production-oriented Go auth service:

- `POST /register`, `POST /login`, защищённый `GET /me`
- JWT-compatible HS256 с проверкой подписи, алгоритма, issuer и TTL
- Bearer middleware
- HMAC-SHA256 KDF с солью и constant-time сравнением
- потокобезопасное in-memory хранилище
- environment-конфигурация и обязательный секрет от 32 байт
- HTTP timeouts и graceful shutdown
- единый JSON-формат ошибок

Основные файлы: [main.go](</Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/03-sdd-skill/main.go>), [http.go](</Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/03-sdd-skill/http.go>), [token.go](</Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/03-sdd-skill/token.go>), [password.go](</Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/03-sdd-skill/password.go>).

Проверки успешны:

- `gofmt`
- `go test ./...`
- `go vet ./...`
- `go build ./...`
- execution-state валиден

HTTP smoke-check не выполнен из-за ограничения sandbox на bind порта. Все задачи `auth-service` отмечены выполненными, Archive не выполнялся.