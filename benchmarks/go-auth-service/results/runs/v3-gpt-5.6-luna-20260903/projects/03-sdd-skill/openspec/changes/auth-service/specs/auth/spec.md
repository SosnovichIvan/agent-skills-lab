# Authentication specification

## Registration

Сервис MUST принимать email и пароль через `POST /register`, нормализовать
email, проверять его формат и минимальную длину пароля. Успешная регистрация
MUST возвращать созданный публичный профиль без password material. Повторный
email MUST завершаться конфликтом.

## Login and token creation

`POST /login` MUST проверять email и пароль и возвращать access token. Token
MUST иметь JWT-compatible три части, header `alg=HS256`, claims `sub`, `email`,
`iss`, `iat`, `exp` и криптографически случайный `jti`.

## Token validation and authorization

Validator MUST отклонять некорректный формат, неподдерживаемый algorithm,
неверную подпись, issuer или истёкший token. Authorization middleware MUST
принимать схему Bearer без учёта регистра и передавать проверенные claims в
context. `GET /me` MUST быть доступен только с валидным token.

## Operational behavior

Хранилище MUST быть безопасно для конкурентного доступа. Пароли MUST храниться
только как случайная соль и результат итеративного KDF. Сервис MUST требовать
секрет не короче 32 байт, иметь HTTP timeouts и graceful shutdown. Все ответы
MUST быть JSON с единообразными ошибками.
