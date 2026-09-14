# SvelteKit

Читайте вместе с `svelte.md` при подтверждённом SvelteKit.

- Сохраняй file-based routes, layouts, `load`, actions, hooks и error/redirect
  conventions версии проекта.
- Зарезервированные файлы `+page`, `+layout`, `+server`, `+error` и route params
  `[id]` сохраняют framework spelling; общий naming fallback к ним не применяется.
- Не импортируй private server modules в universal/client graph.
- Разделяй public environment values и server secrets штатными boundaries.
- Сохраняй dependency tracking и invalidation data loaders; не создавай скрытые
  повторные запросы между server load и client code.
- Forms используют progressive enhancement, когда это принято; pending, field
  errors и повторная отправка имеют определённое поведение.
- Auth в hooks/layout улучшает UX, но resource authorization остаётся на server
  operation boundary.
- Проверяй SSR, hydration, direct URL, client navigation и adapter production build.
