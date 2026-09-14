# Python backend

Читайте для сервиса на FastAPI или Django.

- Сохраняй dependency manager, supported Python version и sync/async модель.
- В FastAPI используй существующие dependencies и schemas; не вызывай blocking
  I/O непосредственно из async request path.
- В Django сохраняй app boundaries, settings split, middleware и ORM patterns;
  migrations создавай штатной командой и просматривай их diff.
- Не смешивай ORM entities, transport schemas и domain objects без принятого в
  проекте mapping boundary.
- Используй transactions для связанных writes; учитывай N+1 и eager loading.
- Не перехватывай широкий `Exception`, если не можешь корректно классифицировать,
  залогировать или преобразовать ошибку и сохранить cause.
- Type hints дополняют, но не заменяют runtime validation внешнего ввода.
- Запускай formatter/linter, type checker и test runner, заданные конфигурацией.
