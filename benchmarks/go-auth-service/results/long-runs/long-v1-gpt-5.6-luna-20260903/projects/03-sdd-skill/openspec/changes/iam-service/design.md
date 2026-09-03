# Design

## Architecture

Сервис разделяет domain, repositories, security primitives, application
services, HTTP transport и process wiring. Module path фиксирован как
`benchmark.local/iam`, entrypoint — `cmd/iamd`.

Repositories находятся в памяти и защищают состояние от data races. Доменная
логика получает clock и ID generator как зависимости. HTTP handlers зависят от
application services, а не от concrete repositories.

## Security decisions

- Password material: random salt + iterative HMAC-SHA256 KDF, constant-time
  comparison.
- Access token: short-lived JWT-compatible HS256.
- Refresh tokens, reset/verification tokens, invites и API keys: opaque random
  secrets, в repositories хранится только SHA-256 hash.
- Refresh token rotation использует family и reuse detection.
- Roles и permissions всегда organization-scoped и не кодируются в access
  token; authorization использует актуальное membership state.
- Audit и logs не содержат password/token/key material.

## Execution

Change специально разбит на 32 последовательных semantic chunks. Каждый turn
реализует ровно один checkbox и сохраняет компилируемое состояние проекта.
OpenSpec `tasks.md` — единственный task ledger для SDD-вариантов.

