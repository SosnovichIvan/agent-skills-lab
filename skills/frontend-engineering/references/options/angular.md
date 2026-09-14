# Angular

Читайте при `@angular/core` или `angular.json` в целевом workspace.

## Structure и dependency injection

- Сохраняй standalone/module организацию и не мигрируй её попутно.
- Следуй naming convention установленной версии/генератора. Если соглашения нет,
  актуальный fallback: слова разделяются дефисами (`user-profile.ts`), test —
  `user-profile.spec.ts`, а template/style companion files используют тот же
  basename. Старую последовательную схему `*.component.ts` не переименовывай
  в рамках несвязанной задачи.
- Группируй код по feature area, а не в глобальные каталоги всех components,
  directives и services.
- Используй dependency injection и providers с тем scope, который принят рядом.
- Не помещай mutable request/component state в root singleton service.
- Component координирует view, а domain/use-case logic остаётся в подходящем
  service/store/module согласно выбранному structure profile.

## Reactivity, forms и routing

- Состояние реализуй существующей моделью: signals, RxJS или store; не создавай
  второй подход внутри одного feature.
- Для RxJS сохраняй cancellation/teardown и не создавай nested subscriptions.
- Не преобразуй signal/Observable туда и обратно без boundary; derived state
  выражай `computed` или stream operator, а не императивной синхронизацией.
- Формы делай в принятом reactive/template-driven стиле и отображай validation.
- Async validator и submit имеют pending/error state и защиту от повторной отправки.
- Route guards не заменяют server authorization; сохраняй resolvers и lazy
  boundaries, если они используются.

## Templates, HTTP и проверка

- Предпочитай безопасные template bindings; не обходи sanitization без
  документированного trusted source.
- Сохраняй typed HTTP client, interceptors и единый error mapping. Не выполняй
  transport call напрямую из template-oriented component, если проект отделяет data layer.
- При `OnPush`/signals не полагайся на неотслеживаемую mutation объекта.
- Запускай Angular CLI build/test/lint через scripts проекта и проверяй template
  type errors.
- Проверяй route navigation, DI scope, form errors и cleanup Observable/effect.
