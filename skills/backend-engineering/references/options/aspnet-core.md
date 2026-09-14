# ASP.NET Core

Читайте при ASP.NET Core SDK/service project.

- Сохраняй minimal API или controllers, принятую в сервисе; не смешивай стили
  внутри одного feature без причины.
- Соблюдай DI lifetimes: scoped state не захватывай singleton-объектом.
- Передавай `CancellationToken` через async I/O path и избегай `.Result`/`.Wait()`.
- Используй model binding/validation и единый Problem Details/error mapping.
- Для EF Core определяй transaction boundary, migrations и query shape; учитывай
  tracking и N+1.
- Сохраняй middleware order: exception handling, forwarded headers, auth,
  authorization и endpoints чувствительны к порядку.
- Не считай endpoint policy заменой resource-level authorization.
- Запускай `dotnet format`/analyzers, tests и release build согласно scripts/CI.
