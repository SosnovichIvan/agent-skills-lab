#!/usr/bin/env python3
"""Run the fixed 32-chunk IAM long-session benchmark."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from verify_long_project import verify


PROTOCOL_VERSION = 1
BENCHMARK_MODEL = "gpt-5.6-luna"
BENCHMARK_REASONING_EFFORT = "medium"
MAX_ATTEMPTS_PER_TASK = 2
VARIANTS = {
    "01-skill-standalone": {
        "skill": True,
        "sdd": False,
        "source": "standalone",
        "context_mode": "fresh-worker-per-task",
    },
    "02-ai-only": {
        "skill": False,
        "sdd": False,
        "source": "standalone",
        "context_mode": "persistent-resume-session",
    },
    "03-sdd-skill": {
        "skill": True,
        "sdd": True,
        "source": "openspec",
        "context_mode": "fresh-worker-per-task",
    },
    "04-sdd-only": {
        "skill": False,
        "sdd": True,
        "source": "openspec",
        "context_mode": "persistent-resume-session",
    },
}


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat()


def safe_run_id(value: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,95}", value):
        raise ValueError("run-id must contain only letters, digits, dot, underscore or dash")
    return value


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--variant", action="append", choices=VARIANTS)
    parser.add_argument("--timeout", type=int, default=1200)
    parser.add_argument(
        "--restart-incomplete",
        action="store_true",
        help="recreate selected variants whose status is not complete",
    )
    parser.add_argument(
        "--recover-verifier-failure",
        action="store_true",
        help="reverify and accept the last failed chunk without another model turn",
    )
    parser.add_argument(
        "--run-id",
        default=datetime.now(timezone.utc).strftime("long-v1-%Y%m%dT%H%M%SZ"),
    )
    return parser.parse_args()


def read_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def save_json(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(
        json.dumps(value, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    temporary.replace(path)


def load_tasks(path: Path) -> list[dict[str, Any]]:
    payload = read_json(path)
    tasks = payload.get("tasks")
    if payload.get("schema_version") != 1 or not isinstance(tasks, list):
        raise RuntimeError("invalid long-session task catalog")
    if len(tasks) != 32:
        raise RuntimeError(f"long-session benchmark requires 32 tasks, found {len(tasks)}")
    ids = [task.get("id") for task in tasks]
    if len(ids) != len(set(ids)) or not all(isinstance(value, str) for value in ids):
        raise RuntimeError("task IDs must be unique strings")
    for task in tasks:
        if not isinstance(task.get("title"), str) or not task.get("title"):
            raise RuntimeError(f"task {task.get('id')} has no title")
        if not isinstance(task.get("done_when"), list) or not task["done_when"]:
            raise RuntimeError(f"task {task.get('id')} has no done_when")
    return tasks


def catalog_hash(requirements: Path, tasks_path: Path) -> str:
    digest = hashlib.sha256()
    digest.update(requirements.read_bytes())
    digest.update(b"\0")
    digest.update(tasks_path.read_bytes())
    return digest.hexdigest()


def render(path: Path, replacements: dict[str, str]) -> str:
    result = path.read_text(encoding="utf-8")
    for key, value in replacements.items():
        result = result.replace("{{" + key + "}}", value)
    leftovers = sorted(set(re.findall(r"\{\{[A-Z_]+\}\}", result)))
    if leftovers:
        raise RuntimeError(f"unresolved prompt placeholders: {leftovers}")
    return result


def generate_openspec_tasks(tasks: list[dict[str, Any]]) -> str:
    lines = ["# Tasks", ""]
    for task in tasks:
        criteria = "; ".join(task["done_when"])
        lines.append(f"- [ ] {task['id']} {task['title']} — {criteria}")
    return "\n".join(lines) + "\n"


def prepare_project(
    project: Path,
    variant: dict[str, Any],
    requirements: Path,
    openspec_template: Path,
    tasks: list[dict[str, Any]],
) -> None:
    if project.exists():
        raise RuntimeError(f"refusing to overwrite existing project: {project}")
    project.mkdir(parents=True)
    shutil.copy2(requirements, project / "benchmark-requirements.md")
    if variant["sdd"]:
        shutil.copytree(openspec_template, project / "openspec")
        tasks_md = project / "openspec" / "changes" / "iam-service" / "tasks.md"
        tasks_md.write_text(generate_openspec_tasks(tasks), encoding="utf-8")


def command_version(command: str, *arguments: str) -> str:
    completed = subprocess.run(
        [command, *(arguments or ("--version",))],
        text=True,
        capture_output=True,
        timeout=10,
        check=False,
    )
    return " ".join((completed.stdout + " " + completed.stderr).split())[:512]


def parse_jsonl(stdout: str) -> tuple[dict[str, int], dict[str, int], dict[str, int], str | None]:
    usage = {
        "input_tokens": 0,
        "cached_input_tokens": 0,
        "cache_write_input_tokens": 0,
        "output_tokens": 0,
        "reasoning_output_tokens": 0,
    }
    event_counts: dict[str, int] = {}
    item_stats = {
        "agent_messages": 0,
        "command_executions": 0,
        "command_output_chars": 0,
        "file_changes": 0,
    }
    thread_id: str | None = None
    completed_turns = 0
    for line in stdout.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        event_type = str(event.get("type", "unknown"))
        event_counts[event_type] = event_counts.get(event_type, 0) + 1
        if event_type == "thread.started" and isinstance(event.get("thread_id"), str):
            thread_id = event["thread_id"]
        if event_type == "item.completed" and isinstance(event.get("item"), dict):
            item = event["item"]
            item_type = item.get("type")
            if item_type == "agent_message":
                item_stats["agent_messages"] += 1
            elif item_type == "command_execution":
                item_stats["command_executions"] += 1
                output = item.get("aggregated_output", "")
                if isinstance(output, str):
                    item_stats["command_output_chars"] += len(output)
            elif item_type == "file_change":
                item_stats["file_changes"] += 1
        if event_type == "turn.completed":
            completed_turns += 1
            raw_usage = event.get("usage", {})
            for key in usage:
                usage[key] += int(raw_usage.get(key, 0))
    if completed_turns == 0:
        raise RuntimeError("Codex JSONL did not contain turn.completed")
    usage["total_tokens"] = usage["input_tokens"] + usage["output_tokens"]
    return usage, event_counts, item_stats, thread_id


def empty_usage() -> dict[str, int]:
    return {
        "input_tokens": 0,
        "cached_input_tokens": 0,
        "cache_write_input_tokens": 0,
        "output_tokens": 0,
        "reasoning_output_tokens": 0,
        "total_tokens": 0,
    }


def aggregate_variant(run: dict[str, Any]) -> dict[str, Any]:
    total = empty_usage()
    duration = 0.0
    attempts = 0
    bootstrap = run.get("bootstrap")
    if isinstance(bootstrap, dict) and bootstrap.get("status") == "complete":
        duration += float(bootstrap.get("duration_seconds", 0))
        for key in total:
            total[key] += int(bootstrap.get("usage", {}).get(key, 0))
    for chunk in run.get("chunks", []):
        for attempt in chunk.get("attempts", []):
            attempts += 1
            duration += float(attempt.get("duration_seconds", 0))
            for key in total:
                total[key] += int(attempt.get("usage", {}).get(key, 0))
    return {
        "duration_seconds": round(duration, 3),
        "usage": total,
        "tasks_completed": sum(1 for item in run.get("chunks", []) if item.get("status") == "complete"),
        "model_attempts": attempts,
    }


def initial_codex_command(
    project: Path,
    last_message: Path,
    *,
    ephemeral: bool,
    output_schema: Path | None = None,
) -> list[str]:
    command = [
        "codex",
        "-a",
        "never",
        "-s",
        "workspace-write",
        "exec",
    ]
    if ephemeral:
        command.append("--ephemeral")
    command.extend(
        [
            "--ignore-user-config",
            "--ignore-rules",
            "--skip-git-repo-check",
            "--json",
            "--color",
            "never",
            "-m",
            BENCHMARK_MODEL,
            "-c",
            f'model_reasoning_effort="{BENCHMARK_REASONING_EFFORT}"',
            "-C",
            str(project),
        ]
    )
    if output_schema is not None:
        command.extend(["--output-schema", str(output_schema)])
    command.extend(["-o", str(last_message), "-"])
    return command


def resume_codex_command(session_id: str, last_message: Path) -> list[str]:
    return [
        "codex",
        "-a",
        "never",
        "-s",
        "workspace-write",
        "exec",
        "resume",
        "--ignore-user-config",
        "--ignore-rules",
        "--skip-git-repo-check",
        "--json",
        "-m",
        BENCHMARK_MODEL,
        "-c",
        f'model_reasoning_effort="{BENCHMARK_REASONING_EFFORT}"',
        "-o",
        str(last_message),
        session_id,
        "-",
    ]


def run_codex(
    command: list[str],
    prompt: str,
    project: Path,
    timeout: int,
    stdout_path: Path,
    stderr_path: Path,
) -> dict[str, Any]:
    stdout_path.parent.mkdir(parents=True, exist_ok=True)
    started = time.monotonic()
    try:
        completed = subprocess.run(
            command,
            cwd=project,
            input=prompt,
            text=True,
            capture_output=True,
            timeout=timeout,
            check=False,
        )
        duration = time.monotonic() - started
        stdout_path.write_text(completed.stdout, encoding="utf-8")
        stderr_path.write_text(completed.stderr, encoding="utf-8")
        try:
            usage, event_counts, item_stats, thread_id = parse_jsonl(completed.stdout)
            parse_error = None
        except RuntimeError as exc:
            usage = empty_usage()
            event_counts = {}
            item_stats = {}
            thread_id = None
            parse_error = str(exc)
        return {
            "exit_code": completed.returncode,
            "duration_seconds": round(duration, 3),
            "usage": usage,
            "event_counts": event_counts,
            "item_stats": item_stats,
            "thread_id": thread_id,
            "parse_error": parse_error,
        }
    except subprocess.TimeoutExpired as exc:
        duration = time.monotonic() - started
        stdout = exc.stdout if isinstance(exc.stdout, str) else ""
        stderr = exc.stderr if isinstance(exc.stderr, str) else ""
        stdout_path.write_text(stdout, encoding="utf-8")
        stderr_path.write_text(stderr, encoding="utf-8")
        return {
            "exit_code": 124,
            "duration_seconds": round(duration, 3),
            "usage": empty_usage(),
            "event_counts": {},
            "item_stats": {},
            "thread_id": None,
            "parse_error": "timeout",
        }


def run_statectl(statectl: Path, project: Path, arguments: list[str]) -> dict[str, Any]:
    completed = subprocess.run(
        [sys.executable, str(statectl), *arguments],
        cwd=project,
        text=True,
        capture_output=True,
        timeout=30,
        check=False,
    )
    if completed.returncode != 0:
        raise RuntimeError(
            f"statectl {' '.join(arguments)} failed: {completed.stderr or completed.stdout}"
        )
    return json.loads(completed.stdout)


def state_path(project: Path, state_id: str) -> Path:
    return project / ".execution-state" / state_id / "state.json"


def initialize_state(
    statectl: Path,
    project: Path,
    state_id: str,
    variant: dict[str, Any],
    tasks_path: Path,
) -> dict[str, Any]:
    arguments = [
        "init",
        "--id",
        state_id,
        "--project-root",
        str(project),
        "--source",
        variant["source"],
        "--profile",
        "reset",
        "--implementation-ref",
        "base-agent",
        "--goal",
        "Завершить 32 проверяемых semantic chunks multi-tenant IAM service",
        "--constraint",
        "Выполнять ровно одну задачу на worker",
        "--constraint",
        "Использовать только стандартную библиотеку Go",
        "--constraint",
        "Не создавать test files и внешние зависимости",
        "--constraint",
        "После каждого task должны проходить gofmt, go test и go vet",
        "--adapter",
        "codex",
    ]
    if variant["sdd"]:
        arguments.extend(
            [
                "--change",
                "iam-service",
                "--tasks-path",
                "openspec/changes/iam-service/tasks.md",
            ]
        )
    else:
        arguments.extend(["--tasks-file", str(tasks_path)])
    return run_statectl(statectl, project, arguments)


def parse_worker_result(path: Path) -> dict[str, Any]:
    text = path.read_text(encoding="utf-8").strip()
    if text.startswith("```"):
        text = re.sub(r"^```(?:json)?\s*", "", text)
        text = re.sub(r"\s*```$", "", text)
    value = json.loads(text)
    if not isinstance(value, dict):
        raise RuntimeError("worker result must be a JSON object")
    return value


def worker_result_matches(result: dict[str, Any], packet: dict[str, Any]) -> tuple[bool, str]:
    expected = {
        "protocol": "execution-state.result/v1",
        "run_id": packet["run_id"],
        "based_on_revision": packet["based_on_revision"],
        "task_id": packet["task"]["id"],
        "status": "complete",
    }
    differences = [
        f"{key}: expected {value!r}, got {result.get(key)!r}"
        for key, value in expected.items()
        if result.get(key) != value
    ]
    return not differences, "; ".join(differences)


def checkbox_state(project: Path, tasks: list[dict[str, Any]]) -> list[bool]:
    path = project / "openspec" / "changes" / "iam-service" / "tasks.md"
    text = path.read_text(encoding="utf-8")
    result = []
    for task in tasks:
        match = re.search(
            rf"(?m)^\s*-\s+\[([ xX])\]\s+{re.escape(task['id'])}\s+",
            text,
        )
        if match is None:
            raise RuntimeError(f"OpenSpec task disappeared: {task['id']}")
        result.append(match.group(1).lower() == "x")
    return result


def expected_checkbox_state(project: Path, tasks: list[dict[str, Any]], completed: int) -> tuple[bool, str]:
    actual = checkbox_state(project, tasks)
    expected = [index < completed for index in range(len(tasks))]
    if actual == expected:
        return True, ""
    return False, f"expected {sum(expected)} checked tasks, found {sum(actual)}"


def bootstrap_skill(
    project: Path,
    variant: dict[str, Any],
    prompts: Path,
    skill_path: Path,
    raw: Path,
    timeout: int,
) -> dict[str, Any]:
    prompt = render(
        prompts / "skill-bootstrap.md",
        {"SKILL_PATH": str(skill_path), "SOURCE": variant["source"]},
    )
    prompt_path = raw / "bootstrap.prompt.md"
    stdout_path = raw / "bootstrap.jsonl"
    stderr_path = raw / "bootstrap.stderr.log"
    last_message = raw / "bootstrap.last-message.md"
    prompt_path.write_text(prompt, encoding="utf-8")
    result = run_codex(
        initial_codex_command(project, last_message, ephemeral=True),
        prompt,
        project,
        timeout,
        stdout_path,
        stderr_path,
    )
    answer = last_message.read_text(encoding="utf-8") if last_message.is_file() else ""
    result["route_observed"] = "reset" if re.search(r"\breset\b", answer, re.IGNORECASE) else "unknown"
    result["status"] = (
        "complete"
        if result["exit_code"] == 0 and result["route_observed"] == "reset"
        else "failed"
    )
    return result


def repair_suffix(verification: dict[str, Any], protocol_error: str) -> str:
    missing = [
        f"{task_id}:{name}"
        for task_id, groups in verification.get("markers", {}).items()
        for name, passed in groups.items()
        if not passed
    ]
    commands = [verification.get("go_test", {}), verification.get("go_vet", {}), verification.get("gofmt", {})]
    failures = [
        f"{' '.join(value.get('command', []))}: {value.get('stderr') or value.get('stdout')}"
        for value in commands
        if value.get("exit_code") != 0 or (value.get("command", [""])[0] == "gofmt" and value.get("stdout", "").strip())
    ]
    return (
        "\n\nПредыдущая попытка не прошла внешний gate. Исправь только текущую задачу.\n"
        f"Protocol error: {protocol_error or 'none'}\n"
        f"Missing markers: {missing or 'none'}\n"
        f"Command failures: {failures or 'none'}\n"
    )


def run_skill_task(
    *,
    project: Path,
    task: dict[str, Any],
    task_index: int,
    state_id: str,
    statectl: Path,
    prompts: Path,
    output_schema: Path,
    raw: Path,
    timeout: int,
) -> dict[str, Any]:
    state_file = state_path(project, state_id)
    current = read_json(state_file)
    if current.get("active_task", {}).get("id") != task["id"]:
        raise RuntimeError(f"state active task does not match {task['id']}")
    revision = int(current["revision"])
    packet_dir = state_file.parent / "packets"
    packet_dir.mkdir(parents=True, exist_ok=True)
    worker_run_id = f"t{task_index + 1:02d}-{uuid.uuid4().hex[:16]}"
    packet_path = packet_dir / f"{task_index + 1:02d}-{worker_run_id}.json"
    packet_result = run_statectl(
        statectl,
        project,
        [
            "packet",
            "--id",
            state_id,
            "--project-root",
            str(project),
            "--expected-revision",
            str(revision),
            "--run-id",
            worker_run_id,
            "--max-turns",
            "8",
            "--output",
            str(packet_path),
        ],
    )
    packet = read_json(packet_path)
    base_prompt = render(
        prompts / "worker.md",
        {"PACKET_JSON": json.dumps(packet, ensure_ascii=False, indent=2)},
    )
    attempts: list[dict[str, Any]] = []
    repair = ""
    accepted_result: dict[str, Any] | None = None
    final_verification: dict[str, Any] = {}
    for attempt_index in range(MAX_ATTEMPTS_PER_TASK):
        attempt_number = attempt_index + 1
        prefix = raw / f"task-{task_index + 1:02d}-attempt-{attempt_number}"
        prompt = base_prompt + repair
        prompt_path = prefix.with_suffix(".prompt.md")
        stdout_path = prefix.with_suffix(".jsonl")
        stderr_path = prefix.with_suffix(".stderr.log")
        last_message = prefix.with_suffix(".last-message.json")
        prompt_path.write_text(prompt, encoding="utf-8")
        attempt = run_codex(
            initial_codex_command(
                project,
                last_message,
                ephemeral=True,
                output_schema=output_schema,
            ),
            prompt,
            project,
            timeout,
            stdout_path,
            stderr_path,
        )
        protocol_error = ""
        parsed: dict[str, Any] | None = None
        if last_message.is_file():
            try:
                parsed = parse_worker_result(last_message)
                matches, protocol_error = worker_result_matches(parsed, packet)
                if not matches:
                    parsed = None
            except (RuntimeError, json.JSONDecodeError) as exc:
                protocol_error = str(exc)
        else:
            protocol_error = "last message file is missing"
        final_verification = verify(project, task["id"], False, timeout=120)
        attempt["protocol_error"] = protocol_error or None
        attempt["worker_result"] = parsed
        attempt["verification"] = final_verification
        attempts.append(attempt)
        if attempt["exit_code"] == 0 and parsed is not None and final_verification["passed"]:
            accepted_result = parsed
            break
        repair = repair_suffix(final_verification, protocol_error)

    if accepted_result is None:
        return {
            "task_id": task["id"],
            "status": "failed",
            "attempts": attempts,
            "packet_bytes": packet_path.stat().st_size,
            "state_bytes": state_file.stat().st_size,
            "packet_revision": packet_result["revision"],
        }
    completion = run_statectl(
        statectl,
        project,
        [
            "complete",
            "--id",
            state_id,
            "--project-root",
            str(project),
            "--expected-revision",
            str(packet["based_on_revision"]),
            "--run-id",
            worker_run_id,
            "--summary",
            accepted_result["summary"][:1000],
            "--evidence",
            f"external compile/static gate passed for {task['id']}",
        ],
    )
    return {
        "task_id": task["id"],
        "status": "complete",
        "attempts": attempts,
        "packet_bytes": packet_path.stat().st_size,
        "state_bytes": state_file.stat().st_size,
        "packet_revision": packet_result["revision"],
        "completion_revision": completion["revision"],
        "verification": final_verification,
    }


def baseline_prompt(
    prompts: Path,
    task: dict[str, Any],
    task_index: int,
    sdd: bool,
) -> str:
    task_json = json.dumps(task, ensure_ascii=False, indent=2)
    checkbox = (
        f"после успешной проверки отметь только checkbox {task['id']} в OpenSpec tasks.md как [x]"
        if sdd
        else "OpenSpec отсутствует; не создавай task ledger"
    )
    if task_index == 0:
        source = (
            "OpenSpec proposal.md, design.md, specs/iam/spec.md и tasks.md"
            if sdd
            else "только benchmark-requirements.md; OpenSpec не создавай"
        )
        return render(
            prompts / "baseline-first.md",
            {
                "TASK_ID": task["id"],
                "TASK_JSON": task_json,
                "SOURCE_GUIDANCE": source,
                "CHECKBOX_GUIDANCE": checkbox,
            },
        )
    return render(
        prompts / "baseline-next.md",
        {
            "TASK_ID": task["id"],
            "TASK_JSON": task_json,
            "CHECKBOX_GUIDANCE": checkbox,
        },
    )


def run_baseline_task(
    *,
    project: Path,
    task: dict[str, Any],
    task_index: int,
    tasks: list[dict[str, Any]],
    sdd: bool,
    session_id: str | None,
    prompts: Path,
    raw: Path,
    timeout: int,
) -> tuple[dict[str, Any], str | None]:
    base_prompt = baseline_prompt(prompts, task, task_index, sdd)
    attempts: list[dict[str, Any]] = []
    repair = ""
    current_session = session_id
    final_verification: dict[str, Any] = {}
    for attempt_index in range(MAX_ATTEMPTS_PER_TASK):
        attempt_number = attempt_index + 1
        prefix = raw / f"task-{task_index + 1:02d}-attempt-{attempt_number}"
        prompt = base_prompt + repair
        prompt_path = prefix.with_suffix(".prompt.md")
        stdout_path = prefix.with_suffix(".jsonl")
        stderr_path = prefix.with_suffix(".stderr.log")
        last_message = prefix.with_suffix(".last-message.md")
        prompt_path.write_text(prompt, encoding="utf-8")
        if current_session is None:
            command = initial_codex_command(project, last_message, ephemeral=False)
        else:
            command = resume_codex_command(current_session, last_message)
        attempt = run_codex(
            command,
            prompt,
            project,
            timeout,
            stdout_path,
            stderr_path,
        )
        if attempt.get("thread_id"):
            current_session = attempt["thread_id"]
        final_verification = verify(project, task["id"], False, timeout=120)
        checkbox_ok = True
        checkbox_error = ""
        if sdd:
            checkbox_ok, checkbox_error = expected_checkbox_state(
                project, tasks, task_index + 1
            )
        attempt["verification"] = final_verification
        attempt["checkbox_error"] = checkbox_error or None
        attempts.append(attempt)
        if attempt["exit_code"] == 0 and final_verification["passed"] and checkbox_ok:
            return (
                {
                    "task_id": task["id"],
                    "status": "complete",
                    "attempts": attempts,
                    "verification": final_verification,
                },
                current_session,
            )
        repair = repair_suffix(final_verification, checkbox_error)
    return (
        {
            "task_id": task["id"],
            "status": "failed",
            "attempts": attempts,
            "verification": final_verification,
        },
        current_session,
    )


def quartile_usage(chunks: list[dict[str, Any]]) -> list[dict[str, Any]]:
    groups = []
    for start in range(0, 32, 8):
        usage = empty_usage()
        duration = 0.0
        attempts = 0
        for chunk in chunks[start : start + 8]:
            for attempt in chunk.get("attempts", []):
                attempts += 1
                duration += float(attempt.get("duration_seconds", 0))
                for key in usage:
                    usage[key] += int(attempt.get("usage", {}).get(key, 0))
        groups.append(
            {
                "tasks": f"{start + 1}-{start + 8}",
                "duration_seconds": round(duration, 3),
                "usage": usage,
                "model_attempts": attempts,
            }
        )
    return groups


def main() -> int:
    args = parse_args()
    try:
        run_id = safe_run_id(args.run_id)
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    root = Path(__file__).resolve().parent
    benchmark_root = root.parent
    repo_root = benchmark_root.parents[1]
    skill_path = repo_root / "skills" / "execution-state"
    statectl = skill_path / "scripts" / "statectl.py"
    requirements = root / "requirements.md"
    tasks_path = root / "tasks.json"
    prompts = root / "prompts"
    output_schema = root / "worker-result.schema.json"
    openspec_template = root / "openspec-template"
    tasks = load_tasks(tasks_path)
    definition_hash = catalog_hash(requirements, tasks_path)
    run_root = benchmark_root / "results" / "long-runs" / run_id
    projects = run_root / "projects"
    raw_root = run_root / "raw"
    metrics_path = run_root / "metrics.json"
    projects.mkdir(parents=True, exist_ok=True)
    raw_root.mkdir(parents=True, exist_ok=True)

    identity = {
        "protocol_version": PROTOCOL_VERSION,
        "model": BENCHMARK_MODEL,
        "reasoning_effort": BENCHMARK_REASONING_EFFORT,
        "task_count": len(tasks),
        "definition_sha256": definition_hash,
    }
    if metrics_path.exists():
        metrics = read_json(metrics_path)
        for key, value in identity.items():
            if metrics.get(key) != value:
                print(f"error: run identity mismatch for {key}", file=sys.stderr)
                return 2
    else:
        metrics = {
            "benchmark": "multi-tenant-iam-long-session",
            **identity,
            "started_at": utc_now(),
            "codex_version": command_version("codex"),
            "python_version": sys.version.split()[0],
            "go_version": command_version("go", "version"),
            "max_attempts_per_task": MAX_ATTEMPTS_PER_TASK,
            "runs": {},
        }
        save_json(metrics_path, metrics)

    selected = args.variant or list(VARIANTS)
    any_failed = False
    for name in selected:
        variant = VARIANTS[name]
        existing = metrics["runs"].get(name)
        if existing and existing.get("status") == "complete":
            print(f"skipping completed {name}", flush=True)
            continue
        project = projects / name
        raw = raw_root / name
        if existing is not None and args.restart_incomplete:
            print(f"restarting incomplete {name}", flush=True)
            if project.exists():
                shutil.rmtree(project)
            if raw.exists():
                shutil.rmtree(raw)
            del metrics["runs"][name]
            save_json(metrics_path, metrics)
            existing = None
        raw.mkdir(parents=True, exist_ok=True)
        if existing is None:
            prepare_project(project, variant, requirements, openspec_template, tasks)
            run: dict[str, Any] = {
                "status": "preparing",
                "started_at": utc_now(),
                "project": str(project.relative_to(repo_root)),
                **variant,
                "chunks": [],
                "session_id": None,
            }
            metrics["runs"][name] = run
            save_json(metrics_path, metrics)
            if variant["skill"]:
                print(f"bootstrapping {name}...", flush=True)
                bootstrap = bootstrap_skill(
                    project, variant, prompts, skill_path, raw, args.timeout
                )
                run["bootstrap"] = bootstrap
                if bootstrap["status"] != "complete":
                    run["status"] = "failed"
                    run["failure"] = "skill did not select reset during bootstrap"
                    run["totals"] = aggregate_variant(run)
                    save_json(metrics_path, metrics)
                    any_failed = True
                    continue
                state_id = "iam-long-" + ("openspec" if variant["sdd"] else "standalone")
                run["state_id"] = state_id
                run["state_init"] = initialize_state(
                    statectl, project, state_id, variant, tasks_path
                )
            run["status"] = "running"
            save_json(metrics_path, metrics)
        else:
            run = existing
            if run.get("status") not in {"running", "preparing", "failed"}:
                print(f"error: cannot resume status {run.get('status')} for {name}", file=sys.stderr)
                return 2
            if (
                args.recover_verifier_failure
                and run.get("chunks")
                and run["chunks"][-1].get("status") == "failed"
            ):
                failed_chunk = run["chunks"][-1]
                failed_index = len(run["chunks"]) - 1
                failed_task = tasks[failed_index]
                recovered_verification = verify(
                    project, failed_task["id"], False, timeout=120
                )
                if not recovered_verification["passed"]:
                    print(
                        f"error: failed chunk still does not verify for {name}",
                        file=sys.stderr,
                    )
                    return 2
                failed_chunk["verification"] = recovered_verification
                if variant["skill"]:
                    current_state = read_json(state_path(project, run["state_id"]))
                    lease = current_state.get("worker_lease") or {}
                    if lease.get("task_id") != failed_task["id"]:
                        print("error: state lease does not match failed chunk", file=sys.stderr)
                        return 2
                    accepted = next(
                        (
                            attempt.get("worker_result")
                            for attempt in reversed(failed_chunk["attempts"])
                            if attempt.get("worker_result") is not None
                        ),
                        None,
                    )
                    if accepted is None:
                        print("error: failed chunk has no valid worker result", file=sys.stderr)
                        return 2
                    completion = run_statectl(
                        statectl,
                        project,
                        [
                            "complete",
                            "--id",
                            run["state_id"],
                            "--project-root",
                            str(project),
                            "--expected-revision",
                            str(current_state["revision"]),
                            "--run-id",
                            lease["run_id"],
                            "--summary",
                            accepted["summary"][:1000],
                            "--evidence",
                            f"external compile/static gate passed for {failed_task['id']}",
                        ],
                    )
                    failed_chunk["completion_revision"] = completion["revision"]
                    failed_chunk["state_bytes"] = state_path(
                        project, run["state_id"]
                    ).stat().st_size
                if variant["sdd"]:
                    checkbox_ok, checkbox_error = expected_checkbox_state(
                        project, tasks, failed_index + 1
                    )
                    if not checkbox_ok:
                        print(
                            f"error: recovered OpenSpec ledger mismatch: {checkbox_error}",
                            file=sys.stderr,
                        )
                        return 2
                failed_chunk["status"] = "complete"
                failed_chunk["recovery"] = {
                    "kind": "verifier_false_negative",
                    "accepted_attempt": len(failed_chunk["attempts"]),
                    "recovered_at": utc_now(),
                }
                run["status"] = "running"
                run.pop("failure", None)
                run["totals"] = aggregate_variant(run)
                save_json(metrics_path, metrics)
                print(
                    f"recovered verifier-only failure for {name} {failed_task['id']}",
                    flush=True,
                )
            if run.get("chunks") and run["chunks"][-1].get("status") != "complete":
                print(
                    f"error: last chunk is partial for {name}; use a new run-id",
                    file=sys.stderr,
                )
                return 2

        start_index = len(run.get("chunks", []))
        session_id = run.get("session_id")
        print(f"running {name} from task {start_index + 1}/32", flush=True)
        for task_index in range(start_index, len(tasks)):
            task = tasks[task_index]
            print(f"{name} task {task_index + 1:02d}/32 {task['id']}...", flush=True)
            if variant["skill"]:
                chunk = run_skill_task(
                    project=project,
                    task=task,
                    task_index=task_index,
                    state_id=run["state_id"],
                    statectl=statectl,
                    prompts=prompts,
                    output_schema=output_schema,
                    raw=raw,
                    timeout=args.timeout,
                )
            else:
                chunk, session_id = run_baseline_task(
                    project=project,
                    task=task,
                    task_index=task_index,
                    tasks=tasks,
                    sdd=variant["sdd"],
                    session_id=session_id,
                    prompts=prompts,
                    raw=raw,
                    timeout=args.timeout,
                )
                run["session_id"] = session_id
            run["chunks"].append(chunk)
            run["totals"] = aggregate_variant(run)
            save_json(metrics_path, metrics)
            if chunk["status"] != "complete":
                run["status"] = "failed"
                run["failure"] = f"task {task['id']} failed after {MAX_ATTEMPTS_PER_TASK} attempts"
                save_json(metrics_path, metrics)
                any_failed = True
                break
            print(
                f"completed {name} {task['id']}: "
                f"tokens={sum(a['usage']['total_tokens'] for a in chunk['attempts'])}",
                flush=True,
            )
        if run.get("status") == "failed":
            continue
        final_verification = verify(project, tasks[-1]["id"], True, timeout=180)
        run["final_verification"] = final_verification
        if variant["sdd"]:
            checkbox_ok, checkbox_error = expected_checkbox_state(project, tasks, len(tasks))
            run["openspec_complete"] = checkbox_ok
            run["openspec_error"] = checkbox_error or None
        else:
            checkbox_ok = True
        if variant["skill"]:
            final_state = read_json(state_path(project, run["state_id"]))
            run["final_state"] = {
                "status": final_state.get("status"),
                "revision": final_state.get("revision"),
                "bytes": state_path(project, run["state_id"]).stat().st_size,
            }
            state_ok = final_state.get("status") == "complete"
        else:
            state_ok = not (project / ".execution-state").exists()
        run["quartiles"] = quartile_usage(run["chunks"])
        run["totals"] = aggregate_variant(run)
        run["finished_at"] = utc_now()
        run["status"] = (
            "complete"
            if final_verification["passed"] and checkbox_ok and state_ok
            else "failed"
        )
        if run["status"] != "complete":
            any_failed = True
        save_json(metrics_path, metrics)

    metrics["finished_at"] = utc_now()
    save_json(metrics_path, metrics)
    print(metrics_path, flush=True)
    return 1 if any_failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
