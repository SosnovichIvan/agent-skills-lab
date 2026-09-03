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
  "run_id": "t10-7b9a8796e74049d5",
  "state_id": "iam-long-openspec",
  "based_on_revision": 19,
  "revision": 19,
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
    "id": "3.2",
    "title": "Реализовать строгую валидацию access token — проверяются format alg signature issuer expiry; сравнение подписи constant-time; malformed claims безопасно отклоняются",
    "done_when": [
      "Реализовать строгую валидацию access token — проверяются format alg signature issuer expiry; сравнение подписи constant-time; malformed claims безопасно отклоняются"
    ],
    "source_refs": [
      "openspec/changes/iam-service/tasks.md#3.2"
    ],
    "working_files": []
  },
  "next_action": "Реализовать строгую валидацию access token — проверяются format alg signature issuer expiry; сравнение подписи constant-time; malformed claims безопасно отклоняются",
  "last_observation": "Реализовано создание JWT-compatible HS256 access token с claims sub, email, iss, iat, exp, jti и криптографически случайным jti.",
  "limits": {
    "max_turns": 8,
    "max_result_bytes": 8192
  }
}
```

