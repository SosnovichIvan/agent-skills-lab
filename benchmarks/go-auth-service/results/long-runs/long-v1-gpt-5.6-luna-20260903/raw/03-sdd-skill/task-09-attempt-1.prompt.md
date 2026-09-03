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
  "run_id": "t09-0ca74436f7e64c6b",
  "state_id": "iam-long-openspec",
  "based_on_revision": 17,
  "revision": 17,
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
    "id": "3.1",
    "title": "Реализовать создание JWT-compatible HS256 access token — token имеет три base64url части; claims содержат sub email iss iat exp jti; jti генерируется криптографически",
    "done_when": [
      "Реализовать создание JWT-compatible HS256 access token — token имеет три base64url части; claims содержат sub email iss iat exp jti; jti генерируется криптографически"
    ],
    "source_refs": [
      "openspec/changes/iam-service/tasks.md#3.1"
    ],
    "working_files": []
  },
  "next_action": "Реализовать создание JWT-compatible HS256 access token — token имеет три base64url части; claims содержат sub email iss iat exp jti; jti генерируется криптографически",
  "last_observation": "Добавлены строгий JSON decoder с запретом unknown fields и multiple JSON values, ограничение body до 1 MiB и envelope-ответы для register.",
  "limits": {
    "max_turns": 8,
    "max_result_bytes": 8192
  }
}
```

