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
  "run_id": "t26-d032e95d42464c15",
  "state_id": "iam-long-standalone",
  "based_on_revision": 51,
  "revision": 51,
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
    "id": "7.2",
    "title": "Реализовать назначение и отзыв organization roles",
    "done_when": [
      "PUT/DELETE role assignment реализованы",
      "role и member обязаны принадлежать одному org",
      "последний owner не может потерять owner role"
    ],
    "source_refs": [
      "user-prompt"
    ],
    "working_files": []
  },
  "next_action": "Реализовать назначение и отзыв organization roles",
  "last_observation": "Добавлены встроенные роли owner/admin/viewer с фиксированными permissions, custom organization roles с непустым permission set и tenant-scoped role repository. Реализованы API создания, просмотра, назначения и снятия ролей.",
  "limits": {
    "max_turns": 8,
    "max_result_bytes": 8192
  }
}
```

