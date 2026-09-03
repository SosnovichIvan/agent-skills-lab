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
  "run_id": "t22-5a52e06d3b0d49b4",
  "state_id": "iam-long-standalone",
  "based_on_revision": 43,
  "revision": 43,
  "source": {
    "kind": "standalone"
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
    "id": "6.2",
    "title": "Реализовать создание и просмотр организаций",
    "done_when": [
      "POST organizations создаёт owner membership",
      "GET organizations возвращает только memberships пользователя",
      "tenant IDs не доверяются из request body"
    ],
    "source_refs": [
      "user-prompt"
    ],
    "working_files": []
  },
  "next_action": "Реализовать создание и просмотр организаций",
  "last_observation": "Добавлены Organization и Membership domain-модели и конкурентно-безопасные in-memory repositories. Organization требует OwnerID; Membership обеспечивает атомарную уникальность по паре organization/user.",
  "limits": {
    "max_turns": 8,
    "max_result_bytes": 8192
  }
}
```

