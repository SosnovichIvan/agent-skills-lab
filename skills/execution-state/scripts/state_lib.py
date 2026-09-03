#!/usr/bin/env python3
"""Shared helpers for the execution-state skill."""

from __future__ import annotations

import json
import os
import re
import tempfile
from pathlib import Path
from typing import Any


STATE_STATUSES = {"planned", "in_progress", "blocked", "complete"}
STATE_MODES = {"standalone", "openspec"}
TASK_STATUSES = {"pending", "ready", "in_progress", "blocked", "complete", "cancelled"}
STEP_STATUSES = {"pending", "ready", "in_progress", "blocked", "complete", "skipped"}
MAX_STATE_BYTES = 64 * 1024
MAX_TASK_BYTES = 32 * 1024

TASK_LINE = re.compile(
    r"^\s*[-*]\s+\[([ xX])\]\s+"
    r"([A-Za-z0-9]+(?:[._-][A-Za-z0-9]+)*)[.)]?\s+(.+?)\s*$"
)


class StateError(ValueError):
    """Raised when execution state is invalid or cannot be updated safely."""


def load_json(path: Path) -> dict[str, Any]:
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise StateError(f"File not found: {path}") from exc
    except json.JSONDecodeError as exc:
        raise StateError(f"Invalid JSON in {path}: {exc}") from exc
    if not isinstance(data, dict):
        raise StateError(f"Expected a JSON object in {path}")
    return data


def atomic_write_json(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(data, ensure_ascii=False, indent=2) + "\n"
    with tempfile.NamedTemporaryFile(
        mode="w", encoding="utf-8", dir=path.parent, delete=False
    ) as handle:
        handle.write(payload)
        temp_name = handle.name
    os.replace(temp_name, path)


def state_path(project_root: Path, change: str) -> Path:
    return project_root / ".execution-state" / change / "state.json"


def task_file_name(task_id: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", task_id):
        raise StateError(f"Unsafe task id: {task_id!r}")
    return f"{task_id}.json"


def task_path_for(state_file: Path, task_id: str) -> Path:
    return state_file.parent / "tasks" / task_file_name(task_id)


def parse_openspec_tasks(tasks_file: Path) -> list[dict[str, Any]]:
    try:
        lines = tasks_file.read_text(encoding="utf-8").splitlines()
    except FileNotFoundError as exc:
        raise StateError(f"OpenSpec tasks file not found: {tasks_file}") from exc

    result: list[dict[str, Any]] = []
    seen: set[str] = set()
    for line_number, line in enumerate(lines, start=1):
        match = TASK_LINE.match(line)
        if not match:
            continue
        checked, task_id, title = match.groups()
        if task_id in seen:
            raise StateError(
                f"Duplicate task id {task_id!r} in {tasks_file}:{line_number}"
            )
        seen.add(task_id)
        result.append(
            {
                "id": task_id,
                "title": title,
                "checked": checked.lower() == "x",
                "line": line_number,
            }
        )
    if not result:
        raise StateError(
            "No executable tasks found. Use checkbox lines with stable ids, "
            "for example: '- [ ] 1.1 Implement feature'."
        )
    return result


def _is_nonempty_string(value: Any) -> bool:
    return isinstance(value, str) and bool(value.strip())


def _is_string_list(value: Any) -> bool:
    return isinstance(value, list) and all(_is_nonempty_string(item) for item in value)


def validate_task(task: dict[str, Any], expected_id: str | None = None) -> list[str]:
    errors: list[str] = []
    task_id = task.get("id")
    if not _is_nonempty_string(task_id):
        errors.append("task.id must be a non-empty string")
    elif expected_id is not None and task_id != expected_id:
        errors.append(f"task.id {task_id!r} does not match expected id {expected_id!r}")

    if not _is_nonempty_string(task.get("title")):
        errors.append("task.title must be a non-empty string")
    if task.get("status") not in TASK_STATUSES:
        errors.append(f"task.status must be one of {sorted(TASK_STATUSES)}")
    if not _is_string_list(task.get("dependencies", [])) and task.get("dependencies") != []:
        errors.append("task.dependencies must be a list of non-empty strings")

    source = task.get("source")
    if not isinstance(source, dict):
        errors.append("task.source must be an object")
    else:
        if not _is_nonempty_string(source.get("task_ref")):
            errors.append("task.source.task_ref must be a non-empty string")
        for field in ("requirements", "design_refs"):
            if not _is_string_list(source.get(field, [])) and source.get(field) != []:
                errors.append(f"task.source.{field} must be a list of strings")

    done_when = task.get("done_when")
    if not _is_string_list(done_when) or not done_when:
        errors.append("task.done_when must contain at least one observable criterion")

    steps = task.get("steps", [])
    if not isinstance(steps, list):
        errors.append("task.steps must be a list")
    else:
        seen_steps: set[str] = set()
        for index, step in enumerate(steps):
            if not isinstance(step, dict):
                errors.append(f"task.steps[{index}] must be an object")
                continue
            step_id = step.get("id")
            if not _is_nonempty_string(step_id):
                errors.append(f"task.steps[{index}].id must be a non-empty string")
            elif step_id in seen_steps:
                errors.append(f"duplicate step id {step_id!r}")
            else:
                seen_steps.add(step_id)
            if not _is_nonempty_string(step.get("text")):
                errors.append(f"task.steps[{index}].text must be a non-empty string")
            if step.get("status") not in STEP_STATUSES:
                errors.append(
                    f"task.steps[{index}].status must be one of {sorted(STEP_STATUSES)}"
                )

    for field in ("working_files", "verification", "blockers"):
        value = task.get(field, [])
        if not _is_string_list(value) and value != []:
            errors.append(f"task.{field} must be a list of strings")

    artifacts = task.get("artifacts", [])
    if not isinstance(artifacts, list):
        errors.append("task.artifacts must be a list")
    else:
        for index, artifact in enumerate(artifacts):
            if not isinstance(artifact, dict) or not _is_nonempty_string(
                artifact.get("path")
            ):
                errors.append(f"task.artifacts[{index}] must contain a non-empty path")

    if task.get("status") == "complete":
        if not task.get("verification"):
            errors.append("complete task must contain verification evidence")
        if not _is_nonempty_string(task.get("result_summary")):
            errors.append("complete task must contain result_summary")
    return errors


def validate_state(
    state: dict[str, Any],
    state_file: Path,
    task_overrides: dict[str, dict[str, Any]] | None = None,
) -> list[str]:
    errors: list[str] = []
    overrides = task_overrides or {}

    if state.get("schema_version") != 2:
        errors.append("schema_version must equal 2")
    mode = state.get("mode")
    if mode not in STATE_MODES:
        errors.append(f"mode must be one of {sorted(STATE_MODES)}")
    implementation_skill = state.get("implementation_skill")
    if not _is_nonempty_string(implementation_skill) or not implementation_skill.startswith("$"):
        errors.append("implementation_skill must be an explicit $skill-name invocation")
    revision = state.get("state_revision")
    if not isinstance(revision, int) or isinstance(revision, bool) or revision < 0:
        errors.append("state_revision must be a non-negative integer")
    if not _is_nonempty_string(state.get("goal")):
        errors.append("goal must be a non-empty string")
    if state.get("current_status") not in STATE_STATUSES:
        errors.append(f"current_status must be one of {sorted(STATE_STATUSES)}")
    if not _is_string_list(state.get("constraints", [])) and state.get("constraints") != []:
        errors.append("constraints must be a list of non-empty strings")

    openspec = state.get("openspec")
    if mode == "openspec" and not isinstance(openspec, dict):
        errors.append("openspec mode requires an openspec object")
    elif mode == "openspec":
        for field in ("change", "schema", "phase", "change_path", "tasks_path"):
            if not _is_nonempty_string(openspec.get(field)):
                errors.append(f"openspec.{field} must be a non-empty string")
    elif mode == "standalone" and openspec is not None:
        errors.append("standalone mode must not contain an openspec object")

    task_order = state.get("task_order")
    if not isinstance(task_order, list) or not all(
        _is_nonempty_string(item) for item in task_order
    ):
        errors.append("task_order must be a list of non-empty task ids")
        task_order = []
    elif len(task_order) != len(set(task_order)):
        errors.append("task_order contains duplicate ids")

    active_task_id = state.get("active_task_id")
    if active_task_id is not None and active_task_id not in task_order:
        errors.append("active_task_id must be null or present in task_order")
    if state.get("current_status") != "complete" and not _is_nonempty_string(
        state.get("next_action")
    ):
        errors.append("next_action must be non-empty while state is not complete")

    checkpoint = state.get("checkpoint")
    if not isinstance(checkpoint, dict):
        errors.append("checkpoint must be an object")
    else:
        if not isinstance(checkpoint.get("can_reset"), bool):
            errors.append("checkpoint.can_reset must be boolean")
        if not _is_nonempty_string(checkpoint.get("reason")):
            errors.append("checkpoint.reason must be a non-empty string")

    loaded_tasks: dict[str, dict[str, Any]] = {}
    for task_id in task_order:
        task_path = task_path_for(state_file, task_id)
        if task_id in overrides:
            task = overrides[task_id]
        else:
            try:
                if task_path.stat().st_size > MAX_TASK_BYTES:
                    errors.append(f"task {task_id} exceeds {MAX_TASK_BYTES} bytes")
                task = load_json(task_path)
            except (OSError, StateError) as exc:
                errors.append(str(exc))
                continue
        loaded_tasks[task_id] = task
        errors.extend(f"task {task_id}: {error}" for error in validate_task(task, task_id))

    for task_id, task in loaded_tasks.items():
        for dependency in task.get("dependencies", []):
            if dependency not in task_order:
                errors.append(f"task {task_id}: unknown dependency {dependency!r}")
        if task.get("status") in {"ready", "in_progress"}:
            incomplete = [
                dependency
                for dependency in task.get("dependencies", [])
                if loaded_tasks.get(dependency, {}).get("status") != "complete"
            ]
            if incomplete:
                errors.append(
                    f"task {task_id}: dependencies are not complete: {', '.join(incomplete)}"
                )

    if active_task_id in loaded_tasks and loaded_tasks[active_task_id].get("status") not in {
        "ready",
        "in_progress",
        "blocked",
    }:
        errors.append("active task must have ready, in_progress, or blocked status")

    if state.get("current_status") == "complete":
        unfinished = [
            task_id
            for task_id, task in loaded_tasks.items()
            if task.get("status") not in {"complete", "cancelled"}
        ]
        if unfinished:
            errors.append(f"complete state has unfinished tasks: {', '.join(unfinished)}")
        if active_task_id is not None:
            errors.append("complete state must have active_task_id null")

    return errors


def resolve_state_file(
    project_root: Path, change: str | None, explicit_state: str | None
) -> Path:
    if explicit_state:
        path = Path(explicit_state)
        return path if path.is_absolute() else project_root / path
    if change:
        return state_path(project_root, change)
    raise StateError("Provide either --change or --state")
