# IAM specification

## General API

Сервис MUST использовать module path `benchmark.local/iam`, стандартную
библиотеку Go и HTTP prefix `/v1`. Success и error responses MUST использовать
единые JSON envelopes. Decoder MUST ограничивать body, запрещать неизвестные
поля и дополнительные JSON values.

## Authentication

Registration MUST нормализовать email, применять password policy и запрещать
повторный email. Login MUST возвращать одинаковую unauthorized ошибку для
неизвестного email и неверного password.

Access tokens MUST быть JWT-compatible HS256 с claims `sub`, `email`, `iss`,
`iat`, `exp`, `jti`. Validator MUST проверять format, algorithm, signature,
issuer и expiration. Protected requests MUST использовать Bearer middleware.

## Sessions and account lifecycle

Refresh tokens MUST быть opaque, храниться только как hashes и вращаться при
каждом использовании. Повторное использование replaced token MUST отзывать всю
token family. Пользователь MUST уметь отзывать одну или все sessions.

Password change/reset MUST отзывать sessions. Reset, email verification и
invite tokens MUST быть одноразовыми, expiring и храниться только как hashes.
Login MUST поддерживать конфигурируемую временную блокировку после серии ошибок.

## Organizations and authorization

Organization MUST иметь хотя бы одного owner. Membership, role и API key MUST
быть ограничены одной организацией. Owner, admin и viewer имеют встроенные
permissions; custom role MUST иметь непустой permission set.

Permissions MUST вычисляться из актуального repository на каждом запросе, а не
из access token. Последний owner MUST быть защищён от удаления или отзыва роли.
Каждый organization endpoint MUST применять tenant isolation.

API keys MUST быть opaque, храниться только как hashes, иметь scopes и никогда
не получать permission за пределами своей организации.

## Security and operations

Rate limiter MUST учитывать IP и authenticated identity. Register и invite MUST
поддерживать idempotency keys. Security mutations MUST создавать redacted audit
events, связанные hash chain. Audit listing MUST иметь bounded cursor
pagination.

Request ID MUST присутствовать в response header, errors и structured JSON
logs. Health, readiness и metrics endpoints MUST не раскрывать персональные
данные. Process MUST проверять configuration, использовать HTTP timeouts и
выполнять graceful shutdown.

