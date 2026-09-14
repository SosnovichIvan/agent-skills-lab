# Node.js and TypeScript backend

Читайте для backend workspace на Node.js с Express, Fastify или NestJS.

- Сохраняй runtime/module mode, package manager и framework lifecycle проекта.
- В Express/Fastify размещай parsing и protocol validation в route/schema layer,
  а бизнес-решения — в service/domain layer.
- В NestJS сохраняй module/provider scopes, guards, pipes, interceptors и filters;
  не превращай controller в application service.
- Обрабатывай promise rejection и async errors принятым framework mechanism.
- Используй AbortSignal/таймауты для поддерживающих их I/O clients.
- Валидируй runtime input schema; TypeScript type сам по себе не валидирует JSON.
- Не блокируй event loop CPU-heavy или sync I/O работой на request path.
- Закрывай connections/workers через lifecycle hooks и graceful shutdown.
- Запускай typecheck, lint, unit/integration tests и production build/scripts.
