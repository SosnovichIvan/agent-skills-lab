# Multi-tenant IAM benchmark

Создай production-oriented multi-tenant IAM HTTP-сервис на Go. Работа должна
выполняться строго по одному заданию из task catalog за model turn; не выполняй
будущие задания заранее.

## Общие ограничения

- module path: `benchmark.local/iam`;
- только стандартная библиотека Go;
- основной процесс: `cmd/iamd`;
- HTTP API использует префикс `/v1`;
- in-memory repositories должны быть безопасны для конкурентного доступа;
- время и генерацию ID инкапсулируй так, чтобы доменная логика не зависела от
  прямых вызовов глобального clock;
- секреты, пароли, refresh tokens и API keys нельзя хранить открытым текстом;
- все JSON-ответы используют envelope `{"data": ...}` либо
  `{"error":{"code":"...","message":"...","request_id":"..."}}`;
- неизвестные JSON-поля и несколько JSON-значений в одном body отклоняются;
- не создавай `*_test.go`, benchmark добавляет скрытые проверки вне рабочего
  проекта;
- после каждого задания проект должен проходить `gofmt`, `go test ./...` и
  `go vet ./...`;
- не используй web, внешние зависимости, другой agent CLI, commit или push.

## Контракты безопасности

- Пароль: случайная соль, итеративный HMAC-SHA256 KDF и
  `subtle.ConstantTimeCompare`; минимальная длина — 12 символов.
- Access token: JWT-compatible HS256, claims `sub`, `email`, `iss`, `iat`,
  `exp`, `jti`; validator проверяет формат, `alg`, подпись, issuer и expiry.
- Refresh token и API key: криптографически случайные opaque values; repository
  хранит только SHA-256 hash. Полное значение возвращается ровно один раз.
- Refresh rotation образует token family. Повторное использование заменённого
  token отзывает всю family.
- Пароль, access/refresh tokens, API keys и Authorization header не попадают в
  ошибки, audit events или логи.
- Membership roles принадлежат организации. Access token не содержит роли и
  permissions: authorization читает актуальное membership из repository.

## HTTP API

### Пользователь и сессии

- `POST /v1/register`
- `POST /v1/login`
- `GET /v1/me`
- `POST /v1/token/refresh`
- `POST /v1/logout`
- `POST /v1/logout-all`
- `GET /v1/sessions`
- `DELETE /v1/sessions/{sessionID}`
- `POST /v1/password/change`
- `POST /v1/password-reset/request`
- `POST /v1/password-reset/confirm`
- `POST /v1/email-verification/request`
- `POST /v1/email-verification/confirm`

### Организации и доступ

- `POST /v1/organizations`
- `GET /v1/organizations`
- `GET /v1/organizations/{orgID}/members`
- `POST /v1/organizations/{orgID}/invites`
- `POST /v1/invites/{token}/accept`
- `PATCH /v1/organizations/{orgID}/members/{userID}`
- `DELETE /v1/organizations/{orgID}/members/{userID}`
- `POST /v1/organizations/{orgID}/ownership-transfer`
- `POST /v1/organizations/{orgID}/roles`
- `PUT /v1/organizations/{orgID}/members/{userID}/roles/{roleID}`
- `DELETE /v1/organizations/{orgID}/members/{userID}/roles/{roleID}`
- `POST /v1/organizations/{orgID}/api-keys`
- `GET /v1/organizations/{orgID}/api-keys`
- `DELETE /v1/organizations/{orgID}/api-keys/{keyID}`
- `GET /v1/organizations/{orgID}/audit`

### Operations

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

## Авторизация

Встроенные роли: `owner`, `admin`, `viewer`. Custom roles хранят непустой
набор permissions. Минимальный permission vocabulary:

- `org.read`, `org.manage`;
- `member.read`, `member.manage`;
- `role.read`, `role.manage`;
- `apikey.read`, `apikey.manage`;
- `audit.read`.

Owner нельзя удалить, пока ownership не передан. Последний owner организации
не может потерять owner role. Любой organization endpoint проверяет membership
и tenant boundary по `orgID` из URL.

## Эксплуатация

Конфигурация читает адрес, issuer, HMAC secret, TTL, rate limits и server
timeouts из environment. HMAC secret короче 32 байт запрещён. HTTP server имеет
read/write/idle timeouts и graceful shutdown по SIGINT/SIGTERM. Request ID
возвращается в `X-Request-ID`, входит в error envelope и structured JSON log.
Метрики предоставляют монотонные counters без персональных данных.

