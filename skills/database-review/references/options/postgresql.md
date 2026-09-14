# PostgreSQL

Применяй к подтверждённой версии PostgreSQL; проверь версии расширений и runner.

- CREATE INDEX CONCURRENTLY нельзя включать в обычный transaction block.
  Неудачный запуск может оставить invalid index; проверь его состояние до retry.
  У concurrent build остаются ожидания, нагрузка и ограничения, это не zero-cost DDL.
- Перед добавлением constraint к большой таблице проверь поддерживаемые способы
  отделить создание от validation, lock и совместимость writers в этой версии.
- EXPLAIN ANALYZE исполняет statement. Для writes/функций с эффектами используй
  изолированную среду; BEGIN/ROLLBACK не гарантирует отсутствия всех побочных эффектов.
- Для уникальности проверь NULL semantics, partial predicate и tenant columns.
  Для конкурентного обновления проверь affected rows и scope блокировки.
- RLS оценивай с фактической application role: owner/superuser и специальные
  привилегии могут менять результат проверки. Не тестируй isolation только админом.

Первоисточники (сверяй с версией проекта):
[CREATE INDEX](https://www.postgresql.org/docs/18/sql-createindex.html),
[EXPLAIN](https://www.postgresql.org/docs/18/sql-explain.html).
