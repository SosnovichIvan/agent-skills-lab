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
  "run_id": "t08-99c3738aeb1b4a08",
  "state_id": "iam-long-openspec",
  "based_on_revision": 15,
  "revision": 15,
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
    "id": "2.4",
    "title": "Добавить строгий JSON decoder, body limits и общий envelope — unknown fields отклоняются; body ограничен по размеру; success/error envelope применяется к существующим handlers",
    "done_when": [
      "Добавить строгий JSON decoder, body limits и общий envelope — unknown fields отклоняются; body ограничен по размеру; success/error envelope применяется к существующим handlers"
    ],
    "source_refs": [
      "openspec/changes/iam-service/tasks.md#2.4"
    ],
    "working_files": []
  },
  "next_action": "Добавить строгий JSON decoder, body limits и общий envelope — unknown fields отклоняются; body ограничен по размеру; success/error envelope применяется к существующим handlers",
  "last_observation": "Реализован POST /v1/register: создаёт пользователя, отклоняет повторный email как conflict и возвращает только public profile в data envelope.",
  "limits": {
    "max_turns": 8,
    "max_result_bytes": 8192
  }
}
```

