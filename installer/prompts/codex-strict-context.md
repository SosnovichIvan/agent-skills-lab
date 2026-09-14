The strict execution-state guard is active for this task. As the first tool
call, run `python3 "{statectl_path}" route` with an honest workload estimate.
Then read and apply `{skill_path}`. A short task may use `passthrough`; do not
create state for it. Other tool calls are blocked until routing succeeds.
