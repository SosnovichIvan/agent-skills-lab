# Nuxt

Читайте вместе с `vue.md` при подтверждённом Nuxt.

- Сохраняй file-based routing, layouts, middleware и auto-import conventions
  версии проекта; не создавай параллельную ручную регистрацию.
- Имена внутри `pages`, `layouts`, `middleware`, `plugins`, `server/api` и других
  convention directories определяют поведение Nuxt и имеют приоритет над общим
  naming fallback. Сохраняй dynamic/optional/catch-all route syntax версии проекта.
- Выбирай server/client/universal module осознанно. Browser APIs защищай
  client boundary, а server secrets не импортируй в client graph.
- Используй принятые Nuxt data utilities и keys; согласуй SSR payload, caching,
  refresh и error behavior.
- Private runtime config остаётся на server; публичные значения объявляются
  через предусмотренную public boundary.
- Server routes и middleware не обходят domain authorization и validation.
- Сохраняй plugins order/scope и не делай request-specific state глобальным.
- Проверяй SSR output, hydration, direct navigation, client transition и
  production build выбранного deployment preset.
