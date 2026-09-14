# Spring Boot

Читайте при подтверждённом Spring Boot в Maven/Gradle service.

- Сохраняй package/module boundaries и выбранный MVC или reactive stack.
- Controllers владеют HTTP mapping/validation; application services — use cases;
  repositories — persistence.
- Используй constructor injection. Не добавляй field injection в новый код.
- Определи transaction boundary на application operation; не удерживай
  transaction во время медленного внешнего I/O без необходимости.
- Сохраняй exception mapping через принятый `ControllerAdvice`/error contract.
- Не смешивай blocking JPA/JDBC операции с reactive request path.
- Учитывай lazy loading, N+1, migration order и optimistic locking.
- Для configuration properties сохраняй validation; secrets не помещай в
  checked-in profiles.
- Запускай выбранный Maven/Gradle wrapper, tests, static analysis и packaging.
