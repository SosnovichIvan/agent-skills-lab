# MySQL / InnoDB

Применяй к подтверждённой версии MySQL и storage engine. MariaDB имеет отдельные
версии и возможности; не переноси поведение автоматически.

- Для ALTER проверь поддержку ALGORITHM и LOCK для конкретной операции/версии.
  Online DDL может ждать metadata lock; длинная транзакция способна задержать rollout.
- Проверь implicit commit для используемого DDL: transaction wrapper runner
  не означает, что вся миграция откатится целиком.
- Unique/index сравнение зависит от collation, prefix и NULL semantics. Проверь
  реальные данные, прежде чем менять case/accent sensitivity или длину ключа.
- При конкурентных range-запросах учитывай isolation и фактические locks;
  узкий WHERE без подходящего индекса не гарантирует узкую блокировку.
- Не обещай INSTANT/INPLACE по названию операции без проверки версии и ограничений.
  Если допустима только одна стратегия, задай её явно вместо неожиданного fallback.

Первоисточник:
[InnoDB Online DDL Operations, MySQL 8.4](https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-operations.html).
