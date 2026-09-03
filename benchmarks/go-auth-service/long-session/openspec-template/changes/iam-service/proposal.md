# Multi-tenant IAM service

## Why

Базовой регистрации и access token недостаточно для управления длительными
пользовательскими сессиями и доступом внутри нескольких организаций. Нужен
самостоятельный IAM-сервис с refresh rotation, lifecycle аккаунта,
organization-scoped RBAC, API keys и проверяемым audit trail.

## Scope

В change входят пользователи, access/refresh sessions, password и email flows,
организации, membership, roles, permissions, API keys, rate limiting,
idempotency, audit и operational endpoints.

В scope не входят внешняя база, email provider, OAuth/OIDC, UI, внешние Go
модули и тестовые файлы, созданные агентом.

