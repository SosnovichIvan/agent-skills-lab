#!/usr/bin/env python3
"""Apply a revision-checked JSON patch envelope to execution state."""

from __future__ import annotations

import argparse
import copy
import json
import sys
from pathlib import Path
from typing import Any

from state_lib import (
    StateError,
    atomic_write_json,
    load_json,
    resolve_state_file,
    task_path_for,
    validate_state,
)


IMMUTABLE_STATE_PATHS = {
    ("schema_version",),
    ("mode",),
    ("state_revision",),
    ("goal",),
    ("constraints",),
    ("implementation_skill",),
    ("openspec", "change"),
    ("openspec", "change_path"),
    ("openspec", "tasks_path"),
}
IMMUTABLE_TASK_PATHS = {("id",), ("source", "task_ref")}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Apply add/replace/remove operations with optimistic revision checks"
    )
    parser.add_argument("patch", help="Path to patch envelope JSON")
    parser.add_argument("--change", help="OpenSpec change id")
    parser.add_argument("--state", help="Explicit path to state.json")
    parser.add_argument("--project-root", default=".", help="Project root")
    return parser.parse_args()


def decode_pointer(pointer: str) -> tuple[str, ...]:
    if not isinstance(pointer, str) or not pointer.startswith("/"):
        raise StateError(f"JSON Pointer must start with '/': {pointer!r}")
    if pointer == "/":
        return ("",)
    return tuple(part.replace("~1", "/").replace("~0", "~") for part in pointer[1:].split("/"))


def is_immutable(path: tuple[str, ...], immutable: set[tuple[str, ...]]) -> bool:
    return any(path[: len(prefix)] == prefix for prefix in immutable)


def list_index(token: str, length: int, allow_end: bool) -> int:
    if token == "-" and allow_end:
        return length
    try:
        index = int(token)
    except ValueError as exc:
        raise StateError(f"Invalid list index {token!r}") from exc
    upper = length if allow_end else length - 1
    if index < 0 or index > upper:
        raise StateError(f"List index {index} out of range")
    return index


def apply_operation(document: Any, operation: dict[str, Any]) -> None:
    op = operation.get("op")
    if op not in {"add", "replace", "remove"}:
        raise StateError(f"Unsupported patch operation: {op!r}")
    path = decode_pointer(operation.get("path"))
    if not path:
        raise StateError("Replacing the document root is not supported")

    parent = document
    for token in path[:-1]:
        if isinstance(parent, dict):
            if token not in parent:
                raise StateError(f"Path does not exist: {operation['path']}")
            parent = parent[token]
        elif isinstance(parent, list):
            parent = parent[list_index(token, len(parent), allow_end=False)]
        else:
            raise StateError(f"Path is not traversable: {operation['path']}")

    token = path[-1]
    if isinstance(parent, dict):
        if op in {"replace", "remove"} and token not in parent:
            raise StateError(f"Path does not exist: {operation['path']}")
        if op == "remove":
            del parent[token]
        else:
            if "value" not in operation:
                raise StateError(f"Operation {op!r} requires value")
            parent[token] = copy.deepcopy(operation["value"])
    elif isinstance(parent, list):
        index = list_index(token, len(parent), allow_end=op == "add")
        if op == "remove":
            parent.pop(index)
        elif op == "replace":
            if "value" not in operation:
                raise StateError("replace operation requires value")
            parent[index] = copy.deepcopy(operation["value"])
        else:
            if "value" not in operation:
                raise StateError("add operation requires value")
            parent.insert(index, copy.deepcopy(operation["value"]))
    else:
        raise StateError(f"Patch parent is not a container: {operation['path']}")


def apply_operations(
    document: dict[str, Any],
    operations: Any,
    immutable: set[tuple[str, ...]],
) -> dict[str, Any]:
    if not isinstance(operations, list):
        raise StateError("Patch operations must be a list")
    updated = copy.deepcopy(document)
    for operation in operations:
        if not isinstance(operation, dict):
            raise StateError("Each patch operation must be an object")
        path = decode_pointer(operation.get("path"))
        if is_immutable(path, immutable):
            raise StateError(f"Immutable path cannot be patched: {operation['path']}")
        apply_operation(updated, operation)
    return updated


def main() -> int:
    args = parse_args()
    root = Path(args.project_root).resolve()
    try:
        target = resolve_state_file(root, args.change, args.state)
        state = load_json(target)
        envelope = load_json(Path(args.patch))

        expected = envelope.get("expected_revision")
        current = state.get("state_revision")
        if expected != current:
            raise StateError(
                f"Revision conflict: patch expects {expected!r}, current is {current!r}"
            )

        updated_state = apply_operations(
            state, envelope.get("state_patch", []), IMMUTABLE_STATE_PATHS
        )
        updated_tasks: dict[str, dict[str, Any]] = {}
        task_patches = envelope.get("task_patches", [])
        if not isinstance(task_patches, list):
            raise StateError("task_patches must be a list")

        known_ids = set(state.get("task_order", []))
        for entry in task_patches:
            if not isinstance(entry, dict):
                raise StateError("Each task_patches entry must be an object")
            task_id = entry.get("task_id")
            if task_id not in known_ids:
                raise StateError(f"Unknown task id in patch: {task_id!r}")
            if task_id in updated_tasks:
                raise StateError(f"Duplicate patch entry for task {task_id!r}")
            task = load_json(task_path_for(target, task_id))
            updated_tasks[task_id] = apply_operations(
                task, entry.get("patch", []), IMMUTABLE_TASK_PATHS
            )

        updated_state["state_revision"] = current + 1
        errors = validate_state(updated_state, target, task_overrides=updated_tasks)
        if errors:
            raise StateError("Patch would create invalid state:\n- " + "\n- ".join(errors))

        for task_id, task in updated_tasks.items():
            atomic_write_json(task_path_for(target, task_id), task)
        atomic_write_json(target, updated_state)
    except (OSError, StateError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    print(f"Applied revision {updated_state['state_revision']} to {target}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
