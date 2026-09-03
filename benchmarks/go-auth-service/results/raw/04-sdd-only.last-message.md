Готово. Реализован production-oriented Go auth-сервис:

- `POST /register`, `POST /login`, защищённый `GET /me`
- in-memory repository с `sync.RWMutex`
- HMAC-SHA256 KDF с солью и constant-time сравнением
- JWT-compatible HS256 с проверкой подписи, алгоритма, issuer и TTL
- Bearer middleware
- единый JSON-формат ошибок
- environment-конфигурация и обязательный секрет от 32 байт
- HTTP timeouts и graceful shutdown
- только стандартная библиотека Go

Проверка:

```text
go test ./...
PASS
```

Тестовых файлов не создавал, `gofmt` выполнен. HTTP smoke-тест невозможен в sandbox из-за запрета `bind`.