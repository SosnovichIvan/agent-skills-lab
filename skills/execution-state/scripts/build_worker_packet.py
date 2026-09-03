#!/usr/bin/env python3
"""Build a minimal handoff packet for one execution-state task."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from state_lib import (
    StateError,
    load_json,
    resolve_state_file,
    task_path_for,
    validate_state,
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build a minimal worker packet")
    parser.add_argument("--change", help="OpenSpec change id")
    parser.add_argument("--state", help="Explicit path to state.json")
    parser.add_argument("--task-id", help="Task id; defaults to active_task_id")
    parser.add_argument("--project-root", default=".", help="Project root")
    parser.add_argument("--output", help="Write packet to a file instead of stdout")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = Path(args.project_root).resolve()
    try:
        target = resolve_state_file(root, args.change, args.state)
        state = load_json(target)
        errors = validate_state(state, target)
        if errors:
            raise StateError("Cannot build packet from invalid state:\n- " + "\n- ".join(errors))

        task_id = args.task_id or state.get("active_task_id")
        if not task_id:
            raise StateError("No active task; provide --task-id")
        if task_id not in state.get("task_order", []):
            raise StateError(f"Unknown task id: {task_id!r}")
        task = load_json(task_path_for(target, task_id))

        packet = {
            "state_revision": state["state_revision"],
            "mode": state["mode"],
            "goal": state["goal"],
            "implementation_skill": state["implementation_skill"],
            "constraints": state.get("constraints", []),
            "task": {
                key: task.get(key)
                for key in (
                    "id",
                    "title",
                    "status",
                    "dependencies",
                    "source",
                    "done_when",
                    "steps",
                    "working_files",
                )
            },
            "last_observation": task.get("last_observation")
            or state.get("last_observation", ""),
            "response_contract": {
                "required": [
                    "task_id",
                    "based_on_revision",
                    "status",
                    "result_summary",
                    "artifacts",
                    "verification",
                    "last_observation",
                    "blockers",
                ]
            },
        }
        if state["mode"] == "openspec":
            packet["openspec"] = {
                key: state["openspec"][key]
                for key in ("change", "change_path", "tasks_path")
            }
        payload = json.dumps(packet, ensure_ascii=False, indent=2) + "\n"
        if args.output:
            output = Path(args.output)
            output.parent.mkdir(parents=True, exist_ok=True)
            output.write_text(payload, encoding="utf-8")
            print(output)
        else:
            print(payload, end="")
    except (OSError, StateError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
