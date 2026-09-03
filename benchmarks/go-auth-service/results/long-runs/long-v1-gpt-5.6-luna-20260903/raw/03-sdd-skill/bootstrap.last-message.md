Выбран маршрут: `source=openspec`, `execution.profile=reset`, `runtime adapter=Codex CLI`, `implementation profile=base-agent`.

`passthrough` недопустим, потому что задача содержит 32 последовательных semantic chunks и обязательный fresh-context handoff после каждого. Passthrough не создаёт execution state и не поддерживает checkpoints, revision-bound handoffs и продолжение между контекстами.

State не создавал, другой agent CLI не запускал.