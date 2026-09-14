# Svelte

Читайте при подтверждённой зависимости Svelte.

- Сохраняй реактивную модель версии проекта; не смешивай legacy syntax и runes
  style в рамках несвязанного изменения.
- Если проект не задаёт иное, component file и export называй `UserCard.svelte`,
  обычные modules — `userCard.ts`, тест — по basename и suffix runner проекта.
- Derived state вычисляй декларативно; effects оставляй для синхронизации с
  внешней системой и обеспечивай cleanup.
- Не дублируй state между props, local state и store без одного владельца.
- Сохраняй component props/events/snippets/slots contract принятой версии.
- DOM и browser APIs используй только в browser lifecycle/boundary.
- Store создавай для нескольких независимых потребителей; component-local state
  не выноси глобально ради удобства теста.
- Проверяй реактивные обновления, teardown и доступное поведение компонента.

Для SvelteKit дополнительно читай `sveltekit.md`.
