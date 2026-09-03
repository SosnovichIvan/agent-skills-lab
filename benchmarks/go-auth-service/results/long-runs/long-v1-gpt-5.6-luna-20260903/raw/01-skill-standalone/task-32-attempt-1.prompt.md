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
  "run_id": "t32-eb6e8a7b831c4558",
  "state_id": "iam-long-standalone",
  "based_on_revision": 63,
  "revision": 63,
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
    "id": "8.4",
    "title": "Завершить wiring, server timeouts, graceful shutdown и итоговую проверку",
    "done_when": [
      "cmd/iamd связывает все production dependencies",
      "server имеет read write idle timeouts и graceful shutdown",
      "gofmt go test и go vet проходят без test files и внешних modules"
    ],
    "source_refs": [
      "user-prompt"
    ],
    "working_files": []
  },
  "next_action": "Завершить wiring, server timeouts, graceful shutdown и итоговую проверку",
  "last_observation": "Добавлены structured JSON access logs без секретов, сквозной X-Request-ID и operations endpoints healthz, readyz и metrics с монотонными process counters.",
  "limits": {
    "max_turns": 8,
    "max_result_bytes": 8192
  }
}
```

