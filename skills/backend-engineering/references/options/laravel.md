# Laravel

Читайте при `laravel/framework` в `composer.json`.

- Сохраняй route/controller/action/service организацию проекта; controller не
  должен владеть сложной бизнес-логикой.
- Используй Form Request или принятый validator для внешнего ввода.
- Применяй policies/gates для authorization и проверяй владение конкретным
  ресурсом, а не только роль.
- Для связанных writes используй transaction; учитывай eager loading и N+1.
- Миграции должны иметь безопасный rollout/rollback в рамках правил проекта.
- Jobs/listeners проектируй идемпотентными, с явными retry/backoff и handling
  окончательного failure.
- Не читай secrets напрямую вне принятого config layer и не кэшируй env values
  способом, конфликтующим с deployment.
- Запускай formatter/static analysis, PHPUnit/Pest и framework checks из scripts.
