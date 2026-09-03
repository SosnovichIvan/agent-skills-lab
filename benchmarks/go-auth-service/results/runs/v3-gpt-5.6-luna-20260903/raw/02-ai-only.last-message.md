Готово. Создан production-oriented Go HTTP-сервис аутентификации:

- `/register`, `/login`, защищённый `/me`;
- JWT-compatible HS256 access tokens с проверкой подписи, алгоритма, issuer и TTL;
- Bearer middleware;
- нормализация и валидация email;
- PBKDF2-style HMAC-SHA256 KDF с солью и constant-time сравнением;
- конкурентно-безопасное in-memory хранилище;
- конфигурация через environment, обязательный секрет минимум 32 байта;
- timeouts и graceful shutdown;
- единый формат JSON-ответов и ошибок;
- только стандартная библиотека.

Проверки:

- `gofmt` выполнен;
- `go test ./...` успешно прошёл;
- `go vet ./...` успешно прошёл;
- сетевой smoke-тест заблокирован ограничением sandbox на `bind`, не ошибкой приложения.

Главный файл запуска: [cmd/authservice/main.go](/Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/results/runs/v3-gpt-5.6-luna-20260903/projects/02-ai-only/cmd/authservice/main.go)