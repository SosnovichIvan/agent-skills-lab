# execution-state long-session bootstrap

Это отдельный контрольный bootstrap-turn. Не реализуй код и не меняй проект.

Явно используй skill по пути `{{SKILL_PATH}}/SKILL.md`. Оцени workload:

- source: `{{SOURCE}}`;
- 32 последовательных semantic chunks;
- после каждого chunk требуется fresh-context handoff;
- runtime adapter: Codex CLI;
- implementation profile: `base-agent`.

Подтверди выбранный route и кратко укажи, почему `passthrough` здесь
недопустим. Не запускай другой agent CLI и не создавай state: deterministic
harness сделает init после bootstrap.

