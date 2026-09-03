Готово. Создан production-oriented Go HTTP-сервис без внешних зависимостей:

- `/register`, `/login`, защищённый `/me`
- JWT-compatible HS256 с проверкой подписи, алгоритма, issuer и TTL
- Bearer middleware
- Нормализация и валидация email
- HMAC-SHA256 KDF с солью и 120 000 итераций
- Конкурентно-безопасное in-memory хранилище
- Единый JSON-формат ответов и ошибок
- Graceful shutdown и server timeouts
- Конфигурация через `AUTH_SECRET`, `AUTH_ISSUER`, `AUTH_ADDR`, `AUTH_TOKEN_TTL`

Файлы:

- [go.mod](/Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/02-ai-only/go.mod)
- [main.go](/Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/02-ai-only/cmd/authservice/main.go)
- [password.go](/Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/02-ai-only/internal/password/password.go)
- [memory.go](/Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/02-ai-only/internal/repository/memory.go)
- [service.go](/Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/02-ai-only/internal/service/service.go)
- [token.go](/Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/02-ai-only/internal/token/token.go)
- [http.go](/Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/02-ai-only/internal/transport/http.go)

Проверка `gofmt` и `go test ./...` прошла успешно.