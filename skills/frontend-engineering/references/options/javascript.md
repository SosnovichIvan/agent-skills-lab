# JavaScript frontend

Читайте эту справку, когда изменяемый workspace использует JavaScript без
TypeScript.

- Сохраняй module mode, supported runtime/browser targets и project lint rules.
- Проверяй внешние JSON/event payload на runtime boundary; JSDoc не заменяет
  валидацию.
- Используй JSDoc types только если это принято в проекте; не начинай скрытую
  TypeScript-миграцию в рамках несвязанной задачи.
- Различай отсутствующее, `null` и пустое значение согласно API contract.
- Не меняй public exports и package boundaries без проверки потребителей.
- Не добавляй transpilation/polyfill, если browser/runtime matrix этого не требует.
