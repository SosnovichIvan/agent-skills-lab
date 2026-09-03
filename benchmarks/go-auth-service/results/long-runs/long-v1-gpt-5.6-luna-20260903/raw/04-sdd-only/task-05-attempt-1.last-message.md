Задача 2.1 выполнена.

Добавлено:

- нормализация email через trim/lowercase и валидацию;
- единая password policy с минимумом 12 Unicode-символов;
- generic password errors без раскрытия пароля;
- repository использует общую нормализацию email;
- checkbox `2.1` отмечен как `[x]`.

`gofmt`, `go test ./...` и `go vet ./...` успешно пройдены.