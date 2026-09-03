# Design

## Components

- HTTP transport отвечает за JSON, status codes и маршрутизацию.
- Auth service координирует регистрацию и вход.
- In-memory repository использует `sync.RWMutex` и нормализованный email.
- Password component создаёт случайную соль и итеративный HMAC-SHA256 KDF,
  сравнение выполняется через `crypto/subtle`.
- Token component создаёт и валидирует JWT-compatible HS256 access tokens.
- Middleware помещает проверенные claims в request context.

## Runtime

Конфигурация читается из environment. Секрет короче 32 байт считается
ошибкой. HTTP server использует read/write/idle timeouts и graceful shutdown по
SIGINT/SIGTERM. Разрешена только стандартная библиотека Go.
