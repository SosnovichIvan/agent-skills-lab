#!/usr/bin/env python3
"""Initialize execution state from an OpenSpec change tasks.md file."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from state_lib import StateError, atomic_write_json, parse_openspec_tasks, state_path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Create .execution-state from OpenSpec tasks.md"
    )
    parser.add_argument("--change", required=True, help="OpenSpec change id")
    parser.add_argument(
        "--project-root", default=".", help="Project root containing openspec/"
    )
    parser.add_argument(
        "--goal", help="Explicit execution goal; defaults to completing the change"
    )
    parser.add_argument(
        "--implementation-skill",
        required=True,
        help="Explicit implementation skill invocation, for example '$backend-api'",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = Path(args.project_root).resolve()
    config_file = root / "openspec" / "config.yaml"
    change_dir = root / "openspec" / "changes" / args.change
    tasks_file = change_dir / "tasks.md"
    target_state = state_path(root, args.change)

    try:
        if not args.implementation_skill.startswith("$"):
            raise StateError("--implementation-skill must start with '$'")
        if not config_file.is_file():
            raise StateError(
                f"OpenSpec project config not found: {config_file}. "
                "This skill cannot initialize state outside OpenSpec."
            )
        tasks = parse_openspec_tasks(tasks_file)
        if target_state.exists() or target_state.parent.exists():
            raise StateError(
                f"Execution directory already exists: {target_state.parent}. "
                "Refusing to overwrite resumable state."
            )

        active_id: str | None = None
        task_documents: list[dict[str, object]] = []
        for item in tasks:
            checked = bool(item["checked"])
            if checked:
                status = "complete"
            elif active_id is None:
                status = "ready"
                active_id = str(item["id"])
            else:
                status = "pending"

            task_documents.append(
                {
                    "id": item["id"],
                    "title": item["title"],
                    "status": status,
                    "dependencies": [],
                    "source": {
                        "task_ref": f"tasks.md#{item['id']}",
                        "requirements": [],
                        "design_refs": [],
                    },
                    "done_when": [
                        "OpenSpec task result is implemented and verification evidence is recorded"
                    ],
                    "steps": [],
                    "working_files": [],
                    "artifacts": [],
                    "verification": (
                        ["Imported as complete from the authoritative tasks.md checkbox"]
                        if checked
                        else []
                    ),
                    "blockers": [],
                    "last_observation": (
                        "Task was already checked in tasks.md" if checked else ""
                    ),
                    "result_summary": (
                        "Imported as already complete from tasks.md" if checked else ""
                    ),
                }
            )

        all_complete = active_id is None
        state = {
            "schema_version": 2,
            "mode": "openspec",
            "state_revision": 0,
            "implementation_skill": args.implementation_skill,
            "goal": args.goal or f"Complete OpenSpec change {args.change}",
            "current_status": "complete" if all_complete else "planned",
            "openspec": {
                "change": args.change,
                "schema": "spec-driven",
                "phase": "apply",
                "change_path": str(change_dir.relative_to(root)),
                "tasks_path": str(tasks_file.relative_to(root)),
            },
            "constraints": [
                "Treat OpenSpec change artifacts as authoritative",
                "Do not mark a task complete without verification evidence",
            ],
            "task_order": [str(item["id"]) for item in tasks],
            "active_task_id": active_id,
            "next_action": (
                f"Refine and start OpenSpec task {active_id}"
                if active_id
                else "No pending OpenSpec tasks"
            ),
            "decisions": [],
            "artifacts": [],
            "blockers": [],
            "last_observation": "Execution state initialized from OpenSpec tasks.md",
            "checkpoint": {
                "can_reset": True,
                "reason": "Initial state is persisted and can be resumed",
            },
        }

        for task in task_documents:
            atomic_write_json(
                target_state.parent / "tasks" / f"{task['id']}.json", task
            )
        atomic_write_json(target_state, state)
    except (OSError, StateError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    print(target_state)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
