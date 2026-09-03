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
  "run_id": "t15-9c06e956809b41c7",
  "state_id": "iam-long-standalone",
  "based_on_revision": 29,
  "revision": 29,
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
    "id": "4.3",
    "title": "Добавить reuse detection и отзыв refresh token family",
    "done_when": [
      "повторное использование rotated token обнаруживается",
      "вся family отзывается",
      "последующие tokens этой family отклоняются"
    ],
    "source_refs": [
      "user-prompt"
    ],
    "working_files": []
  },
  "next_action": "Добавить reuse detection и отзыв refresh token family",
  "last_observation": "Реализован POST /v1/token/refresh с rotation: refresh token заменяется новым, старый отзывается, replay отзывает token family, ответ возвращает access/refresh пару. Login также выдаёт refresh token.",
  "limits": {
    "max_turns": 8,
    "max_result_bytes": 8192
  }
}
```



Предыдущая попытка не прошла внешний gate. Исправь только текущую задачу.
Protocol error: none
Missing markers: ['4.3:reuse detection']
Command failures: none
