Готово. Создан production-oriented Go auth-сервис без внешних зависимостей и OpenSpec.

Основные файлы:

- [main.go](<.../main.go>)
- [http.go](<.../http.go>)
- [service.go](<.../service.go>)
- [repository.go](<.../repository.go>)
- [token.go](<.../token.go>)
- [password.go](<.../password.go>)
- [config.go](<.../config.go>)
- [go.mod](<.../go.mod>)

Реализованы регистрация, login, HS256 JWT, Bearer middleware, `/me`, нормализация email, KDF-паролей, concurrent in-memory repository, env-конфигурация, таймауты и graceful shutdown.

Проверки:

- `gofmt -w *.go`
- `go test ./...` — успешно, тестовых файлов нет
- execution state валиден и завершён: [.execution-state/go-auth-service/state.json](</Users/ivansosnovich/Documents/codex/skils/benchmarks/go-auth-service/projects/01-skill-standalone/.execution-state/go-auth-service/state.json>)

Live smoke-test заблокирован sandbox-политикой на bind локального порта, не ошибкой компиляции или логики запуска.