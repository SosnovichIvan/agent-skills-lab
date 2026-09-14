# SQLite

Применяй к версии библиотеки, реально используемой приложением, и его connection setup.

- Проверь включение foreign_keys на фактических соединениях; декларация FOREIGN
  KEY сама по себе не доказывает enforcement. Изменение PRAGMA внутри transaction
  не следует считать работающим переключением.
- ALTER TABLE имеет собственные ограничения. При rebuild сохраняй constraints,
  indexes, triggers и зависимые views, явно перечисляй переносимые columns.
- Перед выбором rebuild проверь поддержку прямого ALTER/DROP COLUMN в версии
  приложения и ограничения зависимостей; rebuild требуется не для каждого удаления.
- Проверь foreign_key_check и целостность после тестовой миграции. Успешная
  вставка в новую таблицу не доказывает сохранение всех зависимостей.
- WAL улучшает совместное чтение/запись, но не делает SQLite системой со многими
  одновременными writers. Проверь busy handling и длину write transaction.
- SQLite-тесты полезны для SQLite. Не выводи из них корректность типов, DDL и
  isolation другого движка.

Первоисточники:
[Foreign Key Support](https://www.sqlite.org/foreignkeys.html),
[ALTER TABLE](https://www.sqlite.org/lang_altertable.html).
