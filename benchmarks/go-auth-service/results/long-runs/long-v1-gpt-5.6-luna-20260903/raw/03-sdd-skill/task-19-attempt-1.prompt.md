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
  "run_id": "t19-081b6f7d157643dc",
  "state_id": "iam-long-openspec",
  "based_on_revision": 37,
  "revision": 37,
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
    "id": "5.3",
    "title": "Реализовать email verification request и confirm — verification token одноразовый и хранится как hash; confirm отмечает email verified; повторное подтверждение идемпотентно",
    "done_when": [
      "Реализовать email verification request и confirm — verification token одноразовый и хранится как hash; confirm отмечает email verified; повторное подтверждение идемпотентно"
    ],
    "source_refs": [
      "openspec/changes/iam-service/tasks.md#5.3"
    ],
    "working_files": []
  },
  "next_action": "Реализовать email verification request и confirm — verification token одноразовый и хранится как hash; confirm отмечает email verified; повторное подтверждение идемпотентно",
  "last_observation": "Реализованы password reset request/confirm: токены случайные, хранятся только как SHA-256 hash и атомарно одноразово consume-ятся; confirm меняет пароль и отзывает все refresh sessions; request не раскрывает существование email.",
  "limits": {
    "max_turns": 8,
    "max_result_bytes": 8192
  }
}
```

