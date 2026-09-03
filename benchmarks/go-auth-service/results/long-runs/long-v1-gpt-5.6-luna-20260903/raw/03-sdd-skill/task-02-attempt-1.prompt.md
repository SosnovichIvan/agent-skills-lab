# Fresh worker: один semantic chunk

Ты новый worker без истории предыдущих model turns. Выполни ровно одну задачу
из worker packet ниже и остановись. Не реализуй будущие задачи заранее.

Прочитай `benchmark-requirements.md`, перечисленные packet source refs и только
те файлы проекта, которые нужны текущей задаче. Существующий код и текущие
OpenSpec-артефакты являются источниками уже реализованных контрактов.

Ограничения:

- изменяй только текущий проект;
- не создавай `*_test.go`;
- не используй web, внешние зависимости, другой agent CLI, commit или push;
- не редактируй `.execution-state/`;
- при source `openspec` не меняй checkbox: controller сделает это после
  проверки;
- выполни `gofmt`, `go test ./...` и `go vet ./...` перед завершением;
- финальный ответ должен соответствовать переданной JSON Schema и повторять
  `run_id`, `based_on_revision` и `task_id` из packet.

## Worker packet

```json
{
  "protocol": "execution-state.worker/v1",
  "packet_version": 1,
  "run_id": "t02-6943b3faec8645c1",
  "state_id": "iam-long-openspec",
  "based_on_revision": 3,
  "revision": 3,
  "source": {
    "kind": "openspec",
    "change": "iam-service"
  },
  "goal": "Завершить 32 проверяемых semantic chunks multi-tenant IAM service",
  "implementation_ref": "base-agent",
  "constraints": [
    "Выполнять ровно одну задачу на worker",
    "Использовать только стандартную библиотеку Go",
    "Не создавать test files и внешние зависимости",
    "После каждого task должны проходить gofmt, go test и go vet"
  ],
  "task": {
    "id": "1.2",
    "title": "Добавить доменные ID, clock abstraction и типизированные ошибки — ID генерируются через crypto/rand; доменная логика получает Clock через dependency; ошибки различают not found, conflict, invalid и unauthorized",
    "done_when": [
      "Добавить доменные ID, clock abstraction и типизированные ошибки — ID генерируются через crypto/rand; доменная логика получает Clock через dependency; ошибки различают not found, conflict, invalid и unauthorized"
    ],
    "source_refs": [
      "openspec/changes/iam-service/tasks.md#1.2"
    ],
    "working_files": []
  },
  "next_action": "Добавить доменные ID, clock abstraction и типизированные ошибки — ID генерируются через crypto/rand; доменная логика получает Clock через dependency; ошибки различают not found, conflict, invalid и unauthorized",
  "last_observation": "Создан Go-модуль benchmark.local/iam, package layout, конфигурация с валидацией HMAC secret и timeouts, dependency wiring и cmd/iamd с graceful shutdown.",
  "limits": {
    "max_turns": 8,
    "max_result_bytes": 8192
  }
}
```

