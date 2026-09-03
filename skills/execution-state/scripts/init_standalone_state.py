#!/usr/bin/env python3
"""Initialize execution state for a long task that does not use OpenSpec."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from state_lib import (
    StateError,
    atomic_write_json,
    state_path,
    task_file_name,
    validate_state,
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Create standalone .execution-state from a user task"
    )
    parser.add_argument("--task-id", required=True, help="Stable task identifier")
    parser.add_argument("--goal", required=True, help="Verifiable overall goal")
    parser.add_argument("--task-title", required=True, help="First semantic task")
    parser.add_argument(
        "--done-when",
        action="append",
        required=True,
        help="Observable completion criterion; repeat for multiple criteria",
    )
    parser.add_argument(
        "--implementation-skill",
        required=True,
        help="Explicit implementation skill invocation, for example '$backend-api'",
    )
    parser.add_argument(
        "--constraint",
        action="append",
        default=[],
        help="Task constraint; repeat for multiple constraints",
    )
    parser.add_argument("--project-root", default=".", help="Project root")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = Path(args.project_root).resolve()

    try:
        task_name = task_file_name(args.task_id)
        if not args.implementation_skill.startswith("$"):
            raise StateError("--implementation-skill must start with '$'")

        target_state = state_path(root, args.task_id)
        if target_state.exists() or target_state.parent.exists():
            raise StateError(
                f"Execution directory already exists: {target_state.parent}. "
                "Refusing to overwrite resumable state."
            )

        task = {
            "id": args.task_id,
            "title": args.task_title,
            "status": "ready",
            "dependencies": [],
            "source": {
                "task_ref": "user-prompt",
                "requirements": [],
                "design_refs": [],
            },
            "done_when": args.done_when,
            "steps": [],
            "working_files": [],
            "artifacts": [],
            "verification": [],
            "blockers": [],
            "last_observation": "",
            "result_summary": "",
        }
        state = {
            "schema_version": 2,
            "mode": "standalone",
            "state_revision": 0,
            "implementation_skill": args.implementation_skill,
            "goal": args.goal,
            "current_status": "planned",
            "constraints": [
                "Do not expand the user task or permissions beyond the launch prompt",
                *args.constraint,
            ],
            "task_order": [args.task_id],
            "active_task_id": args.task_id,
            "next_action": f"Refine and start standalone task {args.task_id}",
            "decisions": [],
            "artifacts": [],
            "blockers": [],
            "last_observation": "Standalone execution state initialized from user prompt",
            "checkpoint": {
                "can_reset": True,
                "reason": "Initial standalone state is persisted and can be resumed",
            },
        }
        errors = validate_state(
            state, target_state, task_overrides={args.task_id: task}
        )
        if errors:
            raise StateError("Generated invalid state:\n- " + "\n- ".join(errors))

        atomic_write_json(target_state.parent / "tasks" / task_name, task)
        atomic_write_json(target_state, state)
    except (OSError, StateError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    print(target_state)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
