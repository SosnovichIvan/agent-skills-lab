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
  "run_id": "t11-a479b6c3e3e74abf",
  "state_id": "iam-long-standalone",
  "based_on_revision": 21,
  "revision": 21,
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
    "id": "3.3",
    "title": "Реализовать POST /v1/login и единые auth errors",
    "done_when": [
      "валидные credentials возвращают access token",
      "неверные credentials возвращают одинаковую unauthorized ошибку",
      "ответ не раскрывает существование email"
    ],
    "source_refs": [
      "user-prompt"
    ],
    "working_files": []
  },
  "next_action": "Реализовать POST /v1/login и единые auth errors",
  "last_observation": "Реализована строгая валидация HS256 access token: формат JWT, alg/typ, подпись constant-time, issuer, expiry и обязательные claims с безопасным отклонением malformed JSON.",
  "limits": {
    "max_turns": 8,
    "max_result_bytes": 8192
  }
}
```

