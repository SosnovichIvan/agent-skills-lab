Выбран route: `reset`.

- source: `standalone`
- workload: 32 semantic chunks с fresh-context handoff
- runtime adapter: `codex`
- implementation profile: `base-agent`
- подтверждены `fresh_context` и управление сессией

`passthrough` недопустим, поскольку он вообще не создаёт execution state и не обеспечивает checkpoint/revision-bound handoff между чанками.

State не создавал, другой agent CLI не запускал.