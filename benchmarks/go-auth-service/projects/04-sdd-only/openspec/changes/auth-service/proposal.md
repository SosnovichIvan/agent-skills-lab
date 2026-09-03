# Auth service

## Why

Нужен самостоятельный HTTP-сервис для регистрации пользователей, выдачи
access tokens и авторизации защищённых запросов без внешней инфраструктуры.

## Scope

В change входят in-memory пользователи, регистрация, вход, создание и проверка
JWT-compatible HS256 tokens, middleware и защищённый endpoint текущего
пользователя. Персистентная база, refresh tokens, OAuth и тестовые файлы не
входят в scope.
