# Vue

Читайте при подтверждённой зависимости Vue.

## Components и reactivity

- Сохраняй Options API или Composition API, принятый в соседнем коде.
- Для SFC сохрани единый `PascalCase.vue` или `kebab-case.vue`; если соглашения
  нет, используй `UserCard.vue`. Component identifier остаётся PascalCase,
  composable — `useUser.ts`, тест повторяет basename и runner suffix.
- В Composition API группируй код по логическому concern; reusable stateful
  logic оформляй composable с `use`-именем.
- Composable возвращает ясный минимальный contract. При возврате нескольких
  reactive values предпочитай форму, которая не теряет reactivity при destructure.
- Вычисляемое состояние держи в `computed`; watcher используй для side effect,
  а не для копирования derived state.
- Очищай listeners/subscriptions в lifecycle и выполняй DOM side effects только
  на browser-side lifecycle при SSR.
- Сохраняй props, emitted events, slots и `v-model` contract компонента.

## State, UI и проверка

- Локальный `ref/reactive` оставляй в component/composable. Store добавляй для
  state с несколькими независимыми потребителями; следуй Pinia/Vuex проекта.
- Не создавай request-shared singleton state в SSR-приложении.
- Сохраняй router guards, async component boundaries и SFC styling conventions.
- Проверяй component behavior через публичный UI, reactivity после обновления,
  cleanup и hydration, если SSR включён.

Для Nuxt дополнительно читай `nuxt.md`.
