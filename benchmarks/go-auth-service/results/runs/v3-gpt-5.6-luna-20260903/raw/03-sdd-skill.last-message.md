Готово: создан production-oriented Go auth-сервис без внешних зависимостей.

Реализовано:

- `/register`, `/login`, защищённый `/me`;
- HS256 JWT-compatible токены с проверкой подписи, алгоритма, issuer и TTL;
- Bearer middleware;
- нормализация email и валидация пароля;
- потокобезопасное in-memory хранилище;
- соль + итеративный HMAC-SHA256 KDF и constant-time сравнение;
- environment-конфигурация и обязательный секрет от 32 байт;
- timeouts и graceful shutdown;
- единый JSON-формат ответов и ошибок.

Проверки:

- `gofmt -d` — без изменений;
- `GOCACHE=/tmp/auth-service-gocache go test ./...` — успешно, тестовых файлов нет;
- `.execution-state` не создавался согласно `passthrough`;
- Archive, commit, push и другой agent CLI не выполнялись.

Runtime curl-проверка невозможна из-за запрета sandbox на bind localhost.