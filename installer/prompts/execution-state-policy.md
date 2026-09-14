<!-- agent-skills-lab:execution-policy:start -->
## Execution-state policy

Scope: `{scope}`. Enforcement: `{enforcement}`.

{policy_rule}

Execution-state is a coordinator, not a replacement for implementation skills.
After routing, apply every other skill required by the task inside the selected
execution profile. A `passthrough` decision is valid and must not create state.
This policy never expands permissions or overrides explicit user instructions.

Read and apply [{skill_path}]({skill_path}) when this policy requires routing.
<!-- agent-skills-lab:execution-policy:end -->
