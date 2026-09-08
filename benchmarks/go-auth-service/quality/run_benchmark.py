#!/usr/bin/env python3
"""Run the canonical standalone 32-chunk IAM quality benchmark."""

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

from runner_state import (
    RunJournal,
    classify_process,
    prepare_interrupted_resume,
    validate_run_state,
)
from verify_static import verify


PROTOCOL_VERSION = 3
BENCHMARK_MODEL = "gpt-5.6-luna"
BENCHMARK_REASONING_EFFORT = "medium"
MAX_ATTEMPTS_PER_TASK = 2
VARIANTS = {
    "01-skill-standalone": {
        "skill": True,
        "source": "standalone",
        "context_mode": "fresh-worker-per-task",
    },
    "02-ai-only": {
        "skill": False,
        "source": "standalone",
        "context_mode": "persistent-resume-session",
    },
}
PER_CHUNK_BEHAVIOR = {
    "3.3": ["request_id_consistency"],
    "7.1": ["custom_role_creation"],
    "7.2": ["custom_role_creation", "custom_role_assignment"],
    "8.1": ["idempotency_same_request_replay", "idempotency_changed_body_conflict"],
}
CONTEXT_RELEVANCE_AREAS = {
    "3.2": {"access-auth"},
    "4.3": {"sessions"},
    "6.4": {"organizations"},
    "7.1": {"authorization"},
    "8.1": {"operations"},
}


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat()


def truncate_utf8(value: str, max_bytes: int) -> str:
    """Return valid UTF-8 text whose encoded representation fits max_bytes."""
    encoded = value.encode("utf-8")
    if len(encoded) <= max_bytes:
        return value
    return encoded[:max_bytes].decode("utf-8", errors="ignore").rstrip()


def safe_run_id(value: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,95}", value):
        raise ValueError("run-id must contain only letters, digits, dot, underscore or dash")
    return value


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--variant", action="append", choices=VARIANTS)
    parser.add_argument("--tasks-file", type=Path)
    parser.add_argument("--output-root", type=Path)
    parser.add_argument("--skill-path", type=Path)
    parser.add_argument("--worker-schema", type=Path)
    parser.add_argument("--behavior-verifier", type=Path)
    parser.add_argument("--timeout", type=int, default=1200)
    parser.add_argument("--task-limit", type=int, choices=range(1, 33))
    parser.add_argument("--review-interval", type=int, default=8)
    parser.add_argument(
        "--force-recovery-after",
        help="inject one deterministic architecture blocker after this task ID",
    )
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
        "--retry-infrastructure-failure",
        action="store_true",
        help="retry the last chunk when every attempt failed before reaching the model",
    )
    parser.add_argument(
        "--run-id",
        default=datetime.now(timezone.utc).strftime("quality-%Y%m%dT%H%M%SZ"),
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
    if payload.get("schema_version") != "1.0.0" or not isinstance(tasks, list):
        raise RuntimeError("quality benchmark requires task catalog schema 1.0.0")
    if len(tasks) != 32:
        raise RuntimeError(f"quality benchmark requires 32 tasks, found {len(tasks)}")
    context_policy = payload.get("context_policy", {})
    discovery_required = bool(context_policy.get("discovery_required"))
    discovery_reason = context_policy.get("reason")
    if discovery_required and (not isinstance(discovery_reason, str) or not discovery_reason.strip()):
        raise RuntimeError("discovery_required context policy needs a reason")
    ids = [task.get("id") for task in tasks]
    if len(ids) != len(set(ids)) or not all(isinstance(value, str) for value in ids):
        raise RuntimeError("task IDs must be unique strings")
    for task in tasks:
        if not isinstance(task.get("title"), str) or not task.get("title"):
            raise RuntimeError(f"task {task.get('id')} has no title")
        if not isinstance(task.get("done_when"), list) or not task["done_when"]:
            raise RuntimeError(f"task {task.get('id')} has no done_when")
        if (not task.get("reads") or not task.get("writes")) and not discovery_required:
            raise RuntimeError(
                f"task {task.get('id')} needs reads/writes or discovery_required context policy"
            )
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


def prepare_project(
    project: Path,
    requirements: Path,
) -> None:
    if project.exists():
        raise RuntimeError(f"refusing to overwrite existing project: {project}")
    project.mkdir(parents=True)
    shutil.copy2(requirements, project / "benchmark-requirements.md")


def command_version(command: str, *arguments: str) -> str:
    completed = subprocess.run(
        [command, *(arguments or ("--version",))],
        text=True,
        capture_output=True,
        timeout=10,
        check=False,
    )
    return " ".join((completed.stdout + " " + completed.stderr).split())[:512]


def file_sha256(path: Path | None) -> str | None:
    return hashlib.sha256(path.read_bytes()).hexdigest() if path and path.is_file() else None


def git_commit(repo: Path) -> str:
    completed = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=repo,
        text=True,
        capture_output=True,
        timeout=10,
        check=False,
    )
    return completed.stdout.strip() if completed.returncode == 0 else "unavailable"


def runtime_capabilities(statectl: Path) -> dict[str, bool]:
    completed = subprocess.run(
        [sys.executable, str(statectl), "runtime-probe", "--adapter", "codex"],
        text=True,
        capture_output=True,
        timeout=10,
        check=False,
    )
    try:
        value = json.loads(completed.stdout)
        capabilities = value.get("selected", {}).get("capabilities", {})
        return capabilities if isinstance(capabilities, dict) else {}
    except json.JSONDecodeError:
        return {}


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
    usage["uncached_input_tokens"] = max(
        0, usage["input_tokens"] - usage["cached_input_tokens"]
    )
    return usage, event_counts, item_stats, thread_id


def empty_usage() -> dict[str, int]:
    return {
        "input_tokens": 0,
        "uncached_input_tokens": 0,
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
    handoffs = {"cold_start": 0, "continue": 0, "reset": 0, "checkpoint": 0}
    cold_start_seconds = 0.0
    files_in_context = 0
    packet_bytes = 0
    context_bytes = 0
    repair_tokens = 0
    bootstrap = run.get("bootstrap")
    if isinstance(bootstrap, dict) and bootstrap.get("status") == "complete":
        duration += float(bootstrap.get("duration_seconds", 0))
        for key in total:
            total[key] += int(bootstrap.get("usage", {}).get(key, 0))
    for chunk in run.get("chunks", []):
        chunk_attempts = chunk.get("attempts", [])
        for attempt_index, attempt in enumerate(chunk_attempts):
            attempts += 1
            duration += float(attempt.get("duration_seconds", 0))
            for key in total:
                total[key] += int(attempt.get("usage", {}).get(key, 0))
            if attempt_index > 0:
                repair_tokens += int(attempt.get("usage", {}).get("total_tokens", 0))
        handoff = chunk.get("handoff", {})
        actual = handoff.get("actual")
        if actual in handoffs:
            handoffs[actual] += 1
        cold_start_seconds += float(handoff.get("cold_start_seconds", 0))
        files_in_context += int(chunk.get("context_files", 0))
        packet_bytes += int(chunk.get("packet_bytes", 0))
        context_bytes += int(chunk.get("context_bytes", 0))
    for review in run.get("reviews", []):
        attempts += 1
        duration += float(review.get("duration_seconds", 0))
        for key in total:
            total[key] += int(review.get("usage", {}).get(key, 0))
    for recovery in run.get("recoveries", []):
        for attempt in recovery.get("attempts", []):
            attempts += 1
            duration += float(attempt.get("duration_seconds", 0))
            for key in total:
                total[key] += int(attempt.get("usage", {}).get(key, 0))
    return {
        "duration_seconds": round(duration, 3),
        "usage": total,
        "tasks_completed": sum(1 for item in run.get("chunks", []) if item.get("status") == "complete"),
        "model_attempts": attempts,
        "handoffs": handoffs,
        "cold_start_seconds": round(cold_start_seconds, 3),
        "context_files": files_in_context,
        "packet_bytes": packet_bytes,
        "context_bytes": context_bytes,
        "repair_tokens": repair_tokens,
    }


def apply_handoff(decision: str, session_id: str | None) -> tuple[str | None, str]:
    if decision == "continue":
        return session_id, "continue" if session_id else "cold_start"
    if decision == "reset":
        return None, "reset"
    if decision == "checkpoint":
        return None, "checkpoint"
    raise ValueError(f"unsupported handoff decision: {decision}")


def handoff_matches(decision: str, before: str | None, after: str | None) -> bool:
    if decision == "continue":
        return before is None or after == before
    if decision == "reset":
        return before is None or (after is not None and after != before)
    return decision == "checkpoint"


def context_relevance(task_id: str, context: list[dict[str, Any]]) -> dict[str, Any] | None:
    expected = CONTEXT_RELEVANCE_AREAS.get(task_id)
    if expected is None:
        return None
    relevant = [
        entry for entry in context
        if expected.intersection(set(entry.get("areas", [])))
    ]
    covered = {
        area for entry in relevant for area in entry.get("areas", []) if area in expected
    }
    return {
        "expected_areas": sorted(expected),
        "entries": len(context),
        "relevant_entries": len(relevant),
        "precision": round(len(relevant) / len(context), 3) if context else 1.0,
        "coverage": round(len(covered) / len(expected), 3),
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


def resume_codex_command(
    session_id: str,
    last_message: Path,
    output_schema: Path | None = None,
) -> list[str]:
    command = [
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
    ]
    if output_schema is not None:
        command.extend(["--output-schema", str(output_schema)])
    command.extend(["-o", str(last_message), session_id, "-"])
    return command


def run_codex(
    command: list[str],
    prompt: str,
    project: Path,
    timeout: int,
    stdout_path: Path,
    stderr_path: Path,
    journal: RunJournal | None = None,
    heartbeat_seconds: float = 5.0,
) -> dict[str, Any]:
    stdout_path.parent.mkdir(parents=True, exist_ok=True)
    started = time.monotonic()
    timed_out = False
    with stdout_path.open("w", encoding="utf-8") as stdout_stream, stderr_path.open(
        "w", encoding="utf-8"
    ) as stderr_stream:
        attempt_id = stdout_path.name.removesuffix(".jsonl")
        try:
            process = subprocess.Popen(
                command,
                cwd=project,
                stdin=subprocess.PIPE,
                stdout=stdout_stream,
                stderr=stderr_stream,
                text=True,
            )
        except OSError as error:
            stderr_stream.write(str(error))
            stderr_stream.flush()
            if journal is not None:
                journal.spawn_failed(
                    attempt_id=attempt_id,
                    command=command,
                    stdout_path=stdout_path,
                    stderr_path=stderr_path,
                    error=str(error),
                )
            return {
                "exit_code": 127,
                "execution_classification": "infra_error",
                "duration_seconds": round(time.monotonic() - started, 3),
                "usage": empty_usage(),
                "event_counts": {},
                "item_stats": {},
                "thread_id": None,
                "parse_error": f"spawn failed: {error}",
            }
        if journal is not None:
            journal.started(
                attempt_id=attempt_id,
                pid=process.pid,
                command=command,
                stdout_path=stdout_path,
                stderr_path=stderr_path,
            )
        assert process.stdin is not None
        try:
            process.stdin.write(prompt)
        except BrokenPipeError:
            pass
        finally:
            process.stdin.close()
        while process.poll() is None:
            elapsed = time.monotonic() - started
            if elapsed >= timeout:
                timed_out = True
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
                break
            time.sleep(min(heartbeat_seconds, max(0.01, timeout - elapsed)))
            if journal is not None and process.poll() is None:
                journal.heartbeat()
        returncode = process.returncode if process.returncode is not None else 124
    duration = time.monotonic() - started
    classification = classify_process(returncode, timed_out=timed_out)
    if journal is not None:
        journal.finished(
            classification=classification,
            returncode=returncode,
            timed_out=timed_out,
        )
    stdout = stdout_path.read_text(encoding="utf-8")
    if not timed_out:
        try:
            usage, event_counts, item_stats, thread_id = parse_jsonl(stdout)
            parse_error = None
        except RuntimeError as exc:
            usage = empty_usage()
            event_counts = {}
            item_stats = {}
            thread_id = None
            parse_error = str(exc)
    else:
        usage = empty_usage()
        event_counts = {}
        item_stats = {}
        thread_id = None
        parse_error = "timeout"
    return {
        "exit_code": returncode,
        "execution_classification": classification,
        "duration_seconds": round(duration, 3),
        "usage": usage,
        "event_counts": event_counts,
        "item_stats": item_stats,
        "thread_id": thread_id,
        "parse_error": parse_error,
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


def run_behavior_verifier(
    script: Path,
    project: Path,
    timeout: int,
    checks: list[str] | None = None,
) -> dict[str, Any]:
    command = [sys.executable, str(script), "--project", str(project), "--timeout", str(timeout)]
    for check in checks or []:
        command.extend(["--check", check])
    completed = subprocess.run(
        command,
        cwd=project,
        text=True,
        capture_output=True,
        timeout=timeout + 30,
        check=False,
    )
    try:
        report = json.loads(completed.stdout)
    except json.JSONDecodeError:
        report = {"passed": False, "parse_error": completed.stdout[-1000:]}
    report["exit_code"] = completed.returncode
    if completed.stderr:
        report["stderr"] = completed.stderr[-2000:]
    return report


def state_path(project: Path, state_id: str) -> Path:
    return project / ".execution-state" / state_id / "state.json"


def initialize_state(
    statectl: Path,
    project: Path,
    state_id: str,
    tasks_path: Path,
    quality_runtime: bool,
    review_interval: int,
) -> dict[str, Any]:
    arguments = [
        "init",
        "--id",
        state_id,
        "--project-root",
        str(project),
        "--source",
        "standalone",
        "--profile",
        "reset",
        "--implementation-ref",
        "base-agent",
        "--goal",
        "Завершить 32 проверяемых semantic chunks multi-tenant IAM service",
        "--review-interval",
        str(review_interval),
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
    if quality_runtime:
        arguments[arguments.index("--adapter"):arguments.index("--adapter")] = [
            "--invariant", "Все обязательные HTTP routes должны оставаться достижимыми",
            "--invariant", "Request ID и error envelope должны оставаться согласованными",
            "--invariant", "Idempotency fingerprint обязан учитывать request body",
        ]
    arguments.extend(["--tasks-file", str(tasks_path)])
    return run_statectl(statectl, project, arguments)


def supports_quality_state(statectl: Path) -> bool:
    source = statectl.read_text(encoding="utf-8")
    return 'SCHEMA_VERSION = "1.0.0"' in source


def supports_structured_review(statectl: Path) -> bool:
    return 'add_argument("--review-json"' in statectl.read_text(encoding="utf-8")


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
    request_protocol = packet.get("protocol")
    protocols = {
        "execution-state.worker/1.0.0": "execution-state.result/1.0.0",
    }
    result_protocol = protocols.get(request_protocol)
    if result_protocol is None:
        return False, f"unsupported worker request protocol: {request_protocol!r}"
    expected = {
        "protocol": result_protocol,
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


def bootstrap_skill(
    project: Path,
    variant: dict[str, Any],
    prompts: Path,
    skill_path: Path,
    raw: Path,
    timeout: int,
    journal: RunJournal,
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
        journal,
    )
    answer = last_message.read_text(encoding="utf-8") if last_message.is_file() else ""
    result["route_observed"] = "reset" if re.search(r"\breset\b", answer, re.IGNORECASE) else "unknown"
    result["status"] = (
        "complete"
        if result["exit_code"] == 0 and result["route_observed"] == "reset"
        else "failed"
    )
    return result


def repair_suffix(
    verification: dict[str, Any],
    protocol_error: str,
    task: dict[str, Any],
) -> str:
    commands = [verification.get("go_test", {}), verification.get("go_vet", {}), verification.get("gofmt", {})]
    failures = [
        f"{' '.join(value.get('command', []))}: {value.get('stderr') or value.get('stdout')}"
        for value in commands
        if value.get("exit_code") != 0 or (value.get("command", [""])[0] == "gofmt" and value.get("stdout", "").strip())
    ]
    return (
        "\n\nПредыдущая попытка не прошла внешний gate. Исправь только текущую задачу.\n"
        f"Protocol error: {protocol_error or 'none'}\n"
        f"Unmet user contracts: {task.get('done_when', [])}\n"
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
    session_id: str | None,
    journal: RunJournal,
    behavior_verifier: Path | None,
) -> tuple[dict[str, Any], str | None]:
    state_file = state_path(project, state_id)
    current = read_json(state_file)
    if current.get("active_task", {}).get("id") != task["id"]:
        raise RuntimeError(f"state active task does not match {task['id']}")
    revision = int(current["revision"])
    packet_dir = state_file.parent / "packets"
    packet_dir.mkdir(parents=True, exist_ok=True)
    lease = current.get("worker_lease") or {}
    if lease.get("task_id") == task["id"] and lease.get("run_id"):
        worker_run_id = lease["run_id"]
        packet_path = packet_dir / f"{task_index + 1:02d}-{worker_run_id}.json"
        if not packet_path.is_file():
            raise RuntimeError(f"leased worker packet is missing: {packet_path}")
        packet_revision = revision
    else:
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
        packet_revision = packet_result["revision"]
    packet = read_json(packet_path)
    context_bytes = len(json.dumps(packet.get("context", []), ensure_ascii=False).encode("utf-8"))
    relevance = context_relevance(task["id"], packet.get("context", []))
    base_prompt = render(
        prompts / "worker.md",
        {"PACKET_JSON": json.dumps(packet, ensure_ascii=False, indent=2)},
    )
    attempts: list[dict[str, Any]] = []
    repair = ""
    accepted_result: dict[str, Any] | None = None
    final_verification: dict[str, Any] = {}
    behavior_verification: dict[str, Any] = {"passed": True, "checks": []}
    for attempt_index in range(MAX_ATTEMPTS_PER_TASK):
        attempt_number = attempt_index + 1
        prefix = raw / f"task-{task_index + 1:02d}-attempt-{attempt_number}"
        prompt = base_prompt + repair
        prompt_path = prefix.with_suffix(".prompt.md")
        stdout_path = prefix.with_suffix(".jsonl")
        stderr_path = prefix.with_suffix(".stderr.log")
        last_message = prefix.with_suffix(".last-message.json")
        prompt_path.write_text(prompt, encoding="utf-8")
        command = (
            initial_codex_command(project, last_message, ephemeral=False, output_schema=output_schema)
            if session_id is None
            else resume_codex_command(session_id, last_message, output_schema)
        )
        attempt = run_codex(
            command,
            prompt,
            project,
            timeout,
            stdout_path,
            stderr_path,
            journal,
        )
        if attempt.get("thread_id"):
            session_id = attempt["thread_id"]
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
        if behavior_verifier is not None and task["id"] in PER_CHUNK_BEHAVIOR:
            behavior_verification = run_behavior_verifier(
                behavior_verifier,
                project,
                120,
                PER_CHUNK_BEHAVIOR[task["id"]],
            )
        attempt["protocol_error"] = protocol_error or None
        attempt["worker_result"] = parsed
        attempt["verification"] = final_verification
        attempt["behavior_verification"] = behavior_verification
        attempts.append(attempt)
        if (
            attempt["exit_code"] == 0
            and parsed is not None
            and final_verification["passed"]
            and behavior_verification["passed"]
        ):
            accepted_result = parsed
            break
        repair = repair_suffix(final_verification, protocol_error, task)

    if accepted_result is None:
        return ({
            "task_id": task["id"],
            "status": "failed",
            "attempts": attempts,
            "packet_bytes": packet_path.stat().st_size,
            "context_bytes": context_bytes,
            "context_files": len(packet.get("context", [])),
            "context_relevance": relevance,
            "state_bytes": state_file.stat().st_size,
            "packet_revision": packet_revision,
            "behavior_verification": behavior_verification,
        }, session_id)
    completion_arguments = [
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
        truncate_utf8(accepted_result["summary"], 1000),
    ]
    regression_checks = packet.get("task", {}).get("regression_checks", [])
    if regression_checks:
        for check_id in regression_checks:
            completion_arguments.extend(
                [
                    "--check-json",
                    json.dumps(
                        {
                            "id": check_id,
                            "status": "passed",
                            "summary": f"external compile/static gate passed for {task['id']}",
                        },
                        ensure_ascii=False,
                    ),
                ]
            )
    else:
        completion_arguments.extend(
            ["--evidence", f"external compile/static gate passed for {task['id']}"]
        )
    completion = run_statectl(
        statectl,
        project,
        completion_arguments,
    )
    context_updates = accepted_result.get("context_updates", [])
    for entry in context_updates:
        run_statectl(
            statectl,
            project,
            [
                "context-map-update", "--id", state_id, "--project-root", str(project),
                "--expected-revision", str(completion["revision"]),
                "--entry-json", json.dumps(entry, ensure_ascii=False),
            ],
        )
    return ({
        "task_id": task["id"],
        "status": "complete",
        "attempts": attempts,
        "packet_bytes": packet_path.stat().st_size,
        "context_bytes": context_bytes,
        "context_files": len(packet.get("context", [])),
        "context_relevance": relevance,
        "state_bytes": state_file.stat().st_size,
        "packet_revision": packet_revision,
        "completion_revision": completion["revision"],
        "next_handoff": completion.get("next_handoff"),
        "review_required": completion.get("review_required", False),
        "verification": final_verification,
        "behavior_verification": behavior_verification,
    }, session_id)


def run_architecture_review(
    *,
    project: Path,
    state_id: str,
    statectl: Path,
    prompt_template: Path,
    output_schema: Path,
    future_tasks: list[dict[str, Any]],
    raw: Path,
    review_number: int,
    timeout: int,
    journal: RunJournal,
) -> dict[str, Any]:
    state_file = state_path(project, state_id)
    state = read_json(state_file)
    reasons = state.get("quality", {}).get("review_reasons", [])
    prompt = render(
        prompt_template,
        {
            "STATE_PATH": str(state_file.relative_to(project)),
            "REVIEW_REASONS": json.dumps(reasons, ensure_ascii=False),
            "FUTURE_TASKS": json.dumps(
                [
                    {
                        "id": task["id"],
                        "title": task["title"],
                        "done_when": task["done_when"],
                    }
                    for task in future_tasks
                ],
                ensure_ascii=False,
            ),
        },
    )
    prefix = raw / f"architecture-review-{review_number:02d}"
    prefix.with_suffix(".prompt.md").write_text(prompt, encoding="utf-8")
    last_message = prefix.with_suffix(".last-message.md")
    result = run_codex(
        initial_codex_command(
            project,
            last_message,
            ephemeral=True,
            output_schema=output_schema,
        ),
        prompt,
        project,
        timeout,
        prefix.with_suffix(".jsonl"),
        prefix.with_suffix(".stderr.log"),
        journal,
    )
    answer = last_message.read_text(encoding="utf-8").strip() if last_message.is_file() else ""
    if result["exit_code"] != 0 or not answer:
        result["status"] = "failed"
        result["error"] = "architecture reviewer did not return a usable result"
        return result
    try:
        review_result = json.loads(answer)
    except json.JSONDecodeError as error:
        result["status"] = "failed"
        result["error"] = f"malformed architecture review JSON: {error}"
        return result
    if not isinstance(review_result, dict):
        result["status"] = "failed"
        result["error"] = "architecture review result must be an object"
        return result
    try:
        arguments = [
            "architecture-review", "--id", state_id, "--project-root", str(project),
            "--expected-revision", str(state["revision"]),
        ]
        if supports_structured_review(statectl):
            arguments.extend(["--review-json", json.dumps(review_result, ensure_ascii=False)])
        else:
            arguments.extend(
                [
                    "--summary", truncate_utf8(str(review_result.get("summary", "reviewed")), 900),
                    "--evidence", "independent reviewer ran declared architecture checks",
                ]
            )
        command = run_statectl(statectl, project, arguments)
    except RuntimeError as error:
        result["status"] = "failed"
        result["error"] = str(error)
        return result
    verdict = command.get("verdict", review_result.get("verdict"))
    result["status"] = "complete" if verdict == "passed" else "blocked"
    result["review_result"] = review_result
    result["controller"] = command
    return result


def inject_canary_recovery(
    *, project: Path, state_id: str, statectl: Path
) -> dict[str, Any]:
    state = read_json(state_path(project, state_id))
    review_result = {
        "protocol": "execution-state.review/1.0.0",
        "verdict": "blocked",
        "summary": "Deterministic canary recovery is required before continuation",
        "blockers": [
            {
                "contract": (
                    "Create internal/foundation/canary_recovery.go with package foundation "
                    "and exported constant ExecutionStateCanaryRecovery = true"
                ),
                "evidence": "The canary recovery marker is intentionally absent",
                "affected_files": ["internal/foundation/canary_recovery.go"],
                "regression_check": "canary-recovery-marker",
            }
        ],
        "planned_gaps": [],
        "recommendations": ["Keep the recovery isolated from product wiring"],
        "checks": [
            {
                "id": "canary-recovery-marker",
                "status": "failed",
                "summary": "Intentional canary fault injected",
            }
        ],
    }
    controller = run_statectl(
        statectl,
        project,
        [
            "architecture-review", "--id", state_id, "--project-root", str(project),
            "--expected-revision", str(state["revision"]),
            "--review-json", json.dumps(review_result),
        ],
    )
    return {
        "status": "blocked",
        "injected": True,
        "duration_seconds": 0.0,
        "usage": empty_usage(),
        "review_result": review_result,
        "controller": controller,
    }


def baseline_prompt(
    prompts: Path,
    task: dict[str, Any],
    task_index: int,
) -> str:
    task_json = json.dumps(task, ensure_ascii=False, indent=2)
    if task_index == 0:
        return render(
            prompts / "baseline-first.md",
            {
                "TASK_ID": task["id"],
                "TASK_JSON": task_json,
                "SOURCE_GUIDANCE": "только benchmark-requirements.md; OpenSpec не создавай",
                "CHECKBOX_GUIDANCE": "OpenSpec отсутствует; не создавай task ledger",
            },
        )
    return render(
        prompts / "baseline-next.md",
        {
            "TASK_ID": task["id"],
            "TASK_JSON": task_json,
            "CHECKBOX_GUIDANCE": "OpenSpec отсутствует; не создавай task ledger",
        },
    )


def run_baseline_task(
    *,
    project: Path,
    task: dict[str, Any],
    task_index: int,
    session_id: str | None,
    prompts: Path,
    raw: Path,
    timeout: int,
    journal: RunJournal,
    behavior_verifier: Path | None,
) -> tuple[dict[str, Any], str | None]:
    base_prompt = baseline_prompt(prompts, task, task_index)
    attempts: list[dict[str, Any]] = []
    repair = ""
    current_session = session_id
    final_verification: dict[str, Any] = {}
    behavior_verification: dict[str, Any] = {"passed": True, "checks": []}
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
            journal,
        )
        if attempt.get("thread_id"):
            current_session = attempt["thread_id"]
        final_verification = verify(project, task["id"], False, timeout=120)
        if behavior_verifier is not None and task["id"] in PER_CHUNK_BEHAVIOR:
            behavior_verification = run_behavior_verifier(
                behavior_verifier,
                project,
                120,
                PER_CHUNK_BEHAVIOR[task["id"]],
            )
        attempt["verification"] = final_verification
        attempt["behavior_verification"] = behavior_verification
        attempts.append(attempt)
        if attempt["exit_code"] == 0 and final_verification["passed"] and behavior_verification["passed"]:
            return (
                {
                    "task_id": task["id"],
                    "status": "complete",
                    "attempts": attempts,
                    "verification": final_verification,
                    "behavior_verification": behavior_verification,
                },
                current_session,
            )
        repair = repair_suffix(final_verification, "", task)
    return (
        {
            "task_id": task["id"],
            "status": "failed",
            "attempts": attempts,
            "verification": final_verification,
            "behavior_verification": behavior_verification,
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
    skill_path = (args.skill_path or (repo_root / "skills" / "execution-state")).resolve()
    statectl = skill_path / "scripts" / "statectl.py"
    quality_runtime = supports_quality_state(statectl)
    requirements = root / "requirements.md"
    tasks_path = (args.tasks_file or (root / "tasks.json")).resolve()
    prompts = root / "prompts"
    output_schema = (args.worker_schema or (root / "worker-result.schema.json")).resolve()
    all_tasks = load_tasks(tasks_path)
    tasks = all_tasks[:args.task_limit] if args.task_limit else all_tasks
    definition_hash = catalog_hash(requirements, tasks_path)
    behavior_verifier = args.behavior_verifier.resolve() if args.behavior_verifier else None
    environment_manifest = {
        "model": BENCHMARK_MODEL,
        "reasoning_effort": BENCHMARK_REASONING_EFFORT,
        "codex_version": command_version("codex"),
        "python_version": sys.version.split()[0],
        "go_version": command_version("go", "version"),
        "repository_commit": git_commit(repo_root),
        "task_catalog_sha256": file_sha256(tasks_path),
        "requirements_sha256": file_sha256(requirements),
        "static_verifier_sha256": file_sha256(root / "verify_static.py"),
        "behavior_verifier_sha256": file_sha256(behavior_verifier),
        "statectl_sha256": file_sha256(statectl),
        "worker_schema_sha256": file_sha256(output_schema),
        "runtime_capabilities": runtime_capabilities(statectl),
    }
    output_root = (args.output_root or (benchmark_root / "results" / "quality-runs")).resolve()
    run_root = output_root / run_id
    projects = run_root / "projects"
    raw_root = run_root / "raw"
    metrics_path = run_root / "metrics.json"
    effective_tasks_path = run_root / "effective-tasks.json"
    if not effective_tasks_path.exists():
        source_catalog = read_json(tasks_path)
        save_json(
            effective_tasks_path,
            {
                "schema_version": source_catalog["schema_version"],
                "context_policy": source_catalog.get("context_policy", {}),
                "tasks": tasks,
            },
        )
    projects.mkdir(parents=True, exist_ok=True)
    raw_root.mkdir(parents=True, exist_ok=True)

    identity = {
        "protocol_version": PROTOCOL_VERSION,
        "model": BENCHMARK_MODEL,
        "reasoning_effort": BENCHMARK_REASONING_EFFORT,
        "task_count": len(tasks),
        "definition_sha256": definition_hash,
        "review_interval": args.review_interval,
        "force_recovery_after": args.force_recovery_after,
        "environment_manifest": environment_manifest,
    }
    if metrics_path.exists():
        metrics = read_json(metrics_path)
        for key, value in identity.items():
            if metrics.get(key) != value:
                print(f"error: run identity mismatch for {key}", file=sys.stderr)
                return 2
    else:
        metrics = {
            "benchmark": "standalone-iam-quality",
            **identity,
            "started_at": utc_now(),
            "max_attempts_per_task": MAX_ATTEMPTS_PER_TASK,
            "runs": {},
        }
        save_json(metrics_path, metrics)

    selected = args.variant or list(VARIANTS)
    any_execution_failure = False
    for name in selected:
        variant = VARIANTS[name]
        existing = metrics["runs"].get(name)
        if existing and existing.get("execution_status") == "complete":
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
            prepare_project(project, requirements)
            run: dict[str, Any] = {
                "execution_status": "preparing",
                "quality_outcome": "not_evaluated",
                "started_at": utc_now(),
                "project": str(project.relative_to(repo_root)),
                **variant,
                "chunks": [],
                "reviews": [],
                "recoveries": [],
                "session_id": None,
            }
            metrics["runs"][name] = run
            validate_run_state(run)
            save_json(metrics_path, metrics)
            journal = RunJournal(
                metrics,
                metrics_path,
                name,
                run_root / "orchestration.jsonl",
            )
            if variant["skill"]:
                print(f"bootstrapping {name}...", flush=True)
                bootstrap = bootstrap_skill(
                    project, variant, prompts, skill_path, raw, args.timeout, journal
                )
                run["bootstrap"] = bootstrap
                if bootstrap["status"] != "complete":
                    run["failure"] = "skill did not select reset during bootstrap"
                    run["totals"] = aggregate_variant(run)
                    classification = bootstrap.get("execution_classification")
                    if classification == "complete":
                        run["execution_status"] = "complete"
                        run["quality_outcome"] = "fail"
                    else:
                        run["execution_status"] = classification or "infra_error"
                        run["quality_outcome"] = "not_evaluated"
                        any_execution_failure = True
                    validate_run_state(run)
                    save_json(metrics_path, metrics)
                    continue
                state_id = "iam-quality-standalone"
                run["state_id"] = state_id
                run["state_init"] = initialize_state(
                    statectl,
                    project,
                    state_id,
                    effective_tasks_path,
                    quality_runtime,
                    args.review_interval,
                )
            run["execution_status"] = "running"
            run["quality_outcome"] = "not_evaluated"
            validate_run_state(run)
            save_json(metrics_path, metrics)
        else:
            run = existing
            journal = RunJournal(
                metrics,
                metrics_path,
                name,
                run_root / "orchestration.jsonl",
            )
            if run.get("execution_status") not in {"running", "preparing", "interrupted", "infra_error"}:
                print(f"error: cannot resume status {run.get('execution_status')} for {name}", file=sys.stderr)
                return 2
            if args.retry_infrastructure_failure and run.get("execution_status") == "interrupted":
                try:
                    resume_index = prepare_interrupted_resume(run)
                except ValueError as error:
                    print(f"error: {error}", file=sys.stderr)
                    return 2
                run["totals"] = aggregate_variant(run)
                save_json(metrics_path, metrics)
                print(
                    f"resuming {name} from durable task index {resume_index}",
                    flush=True,
                )
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
                            truncate_utf8(accepted["summary"], 1000),
                            "--evidence",
                            f"external compile/static gate passed for {failed_task['id']}",
                        ],
                    )
                    failed_chunk["completion_revision"] = completion["revision"]
                    failed_chunk["state_bytes"] = state_path(
                        project, run["state_id"]
                    ).stat().st_size
                failed_chunk["status"] = "complete"
                failed_chunk["recovery"] = {
                    "kind": "verifier_false_negative",
                    "accepted_attempt": len(failed_chunk["attempts"]),
                    "recovered_at": utc_now(),
                }
                run["execution_status"] = "running"
                run["quality_outcome"] = "not_evaluated"
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
        print(f"running {name} from task {start_index + 1}/{len(tasks)}", flush=True)
        for task_index in range(start_index, len(tasks)):
            task = tasks[task_index]
            print(f"{name} task {task_index + 1:02d}/{len(tasks)} {task['id']}...", flush=True)
            if variant["skill"]:
                current_state = read_json(state_path(project, run["state_id"]))
                handoff_decision = current_state.get("quality", {}).get("next_handoff", "checkpoint")
                session_before = session_id
                session_id, actual_handoff = apply_handoff(handoff_decision, session_id)
                chunk, session_id = run_skill_task(
                    project=project,
                    task=task,
                    task_index=task_index,
                    state_id=run["state_id"],
                    statectl=statectl,
                    prompts=prompts,
                    output_schema=output_schema,
                    raw=raw,
                    timeout=args.timeout,
                    session_id=session_id,
                    journal=journal,
                    behavior_verifier=args.behavior_verifier.resolve() if args.behavior_verifier else None,
                )
                chunk["handoff"] = {
                    "expected": handoff_decision,
                    "actual": actual_handoff,
                    "session_before": session_before,
                    "session_after": session_id,
                    "matched": handoff_matches(handoff_decision, session_before, session_id),
                    "cold_start_seconds": (
                        float(chunk.get("attempts", [{}])[0].get("duration_seconds", 0))
                        if actual_handoff != "continue" and chunk.get("attempts")
                        else 0.0
                    ),
                }
                if not chunk["handoff"]["matched"]:
                    chunk["status"] = "failed"
                    chunk["handoff_error"] = "runtime session did not match controller handoff"
                run["session_id"] = session_id
            else:
                chunk, session_id = run_baseline_task(
                    project=project,
                    task=task,
                    task_index=task_index,
                    session_id=session_id,
                    prompts=prompts,
                    raw=raw,
                    timeout=args.timeout,
                    journal=journal,
                    behavior_verifier=args.behavior_verifier.resolve() if args.behavior_verifier else None,
                )
                run["session_id"] = session_id
            run["chunks"].append(chunk)
            run["totals"] = aggregate_variant(run)
            save_json(metrics_path, metrics)
            if chunk["status"] != "complete":
                run["failure"] = f"task {task['id']} failed after {MAX_ATTEMPTS_PER_TASK} attempts"
                attempts = chunk.get("attempts") or []
                classification = (
                    attempts[-1].get("execution_classification")
                    if attempts
                    else "infra_error"
                )
                if classification == "complete":
                    run["execution_status"] = "complete"
                    run["quality_outcome"] = "fail"
                else:
                    run["execution_status"] = classification or "infra_error"
                    run["quality_outcome"] = "not_evaluated"
                    any_execution_failure = True
                validate_run_state(run)
                save_json(metrics_path, metrics)
                break
            if variant["skill"] and chunk.get("review_required"):
                print(f"{name} architecture review after {task['id']}...", flush=True)
                force_recovery = (
                    args.force_recovery_after == task["id"]
                    and not run.get("forced_recovery_injected")
                )
                if force_recovery:
                    if not supports_structured_review(statectl):
                        raise RuntimeError("forced recovery requires structured review support")
                    review = inject_canary_recovery(
                        project=project,
                        state_id=run["state_id"],
                        statectl=statectl,
                    )
                    run["forced_recovery_injected"] = True
                else:
                    review = run_architecture_review(
                        project=project,
                        state_id=run["state_id"],
                        statectl=statectl,
                        prompt_template=root / "prompts" / "architecture-review.md",
                        output_schema=root / "architecture-review.schema.json",
                        future_tasks=tasks[task_index + 1:],
                        raw=raw,
                        review_number=len(run.get("reviews", [])) + 1,
                        timeout=args.timeout,
                        journal=journal,
                    )
                run.setdefault("reviews", []).append(review)
                run["totals"] = aggregate_variant(run)
                save_json(metrics_path, metrics)
                if review["status"] == "blocked" and review.get("controller", {}).get("recovery_task"):
                    recovery_task = review["controller"]["recovery_task"]
                    print(f"{name} recovery {recovery_task['id']}...", flush=True)
                    recovery, session_id = run_skill_task(
                        project=project,
                        task=recovery_task,
                        task_index=len(tasks) + len(run.get("recoveries", [])),
                        state_id=run["state_id"],
                        statectl=statectl,
                        prompts=prompts,
                        output_schema=output_schema,
                        raw=raw,
                        timeout=args.timeout,
                        session_id=None,
                        journal=journal,
                        behavior_verifier=None,
                    )
                    recovery["handoff"] = {
                        "expected": "checkpoint",
                        "actual": "checkpoint",
                        "session_before": run.get("session_id"),
                        "session_after": session_id,
                        "matched": True,
                        "cold_start_seconds": (
                            float(recovery.get("attempts", [{}])[0].get("duration_seconds", 0))
                            if recovery.get("attempts") else 0.0
                        ),
                    }
                    run.setdefault("recoveries", []).append(recovery)
                    run["session_id"] = session_id
                    run["totals"] = aggregate_variant(run)
                    save_json(metrics_path, metrics)
                    if recovery["status"] == "complete":
                        review = run_architecture_review(
                            project=project,
                            state_id=run["state_id"],
                            statectl=statectl,
                            prompt_template=root / "prompts" / "architecture-review.md",
                            output_schema=root / "architecture-review.schema.json",
                            future_tasks=tasks[task_index + 1:],
                            raw=raw,
                            review_number=len(run.get("reviews", [])) + 1,
                            timeout=args.timeout,
                            journal=journal,
                        )
                        run["reviews"].append(review)
                        run["totals"] = aggregate_variant(run)
                        save_json(metrics_path, metrics)
                if review["status"] != "complete":
                    run["failure"] = f"architecture review failed after {task['id']}"
                    classification = review.get("execution_classification")
                    if classification == "complete":
                        run["execution_status"] = "complete"
                        run["quality_outcome"] = "fail"
                    else:
                        run["execution_status"] = classification or "infra_error"
                        run["quality_outcome"] = "not_evaluated"
                        any_execution_failure = True
                    validate_run_state(run)
                    save_json(metrics_path, metrics)
                    break
            print(
                f"completed {name} {task['id']}: "
                f"tokens={sum(a['usage']['total_tokens'] for a in chunk['attempts'])}",
                flush=True,
            )
        if run.get("execution_status") != "running":
            continue
        final_verification = verify(
            project,
            tasks[-1]["id"],
            len(tasks) == len(all_tasks),
            timeout=180,
        )
        run["final_verification"] = final_verification
        behavior_ok = True
        if args.behavior_verifier:
            behavior = run_behavior_verifier(args.behavior_verifier.resolve(), project, 180)
            run["behavior_verification"] = behavior
            behavior_ok = bool(behavior.get("passed"))
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
        run["execution_status"] = "complete"
        run["quality_outcome"] = (
            "pass" if final_verification["passed"] and behavior_ok and state_ok else "fail"
        )
        if run["quality_outcome"] == "fail":
            failed_gates = []
            if not final_verification["passed"]:
                failed_gates.append("compile/static")
            if not behavior_ok:
                failed_gates.append("behavior")
            if not state_ok:
                failed_gates.append("execution-state")
            run["failure"] = "final gates failed: " + ", ".join(failed_gates)
        validate_run_state(run)
        save_json(metrics_path, metrics)

    metrics["finished_at"] = utc_now()
    save_json(metrics_path, metrics)
    print(metrics_path, flush=True)
    return 1 if any_execution_failure else 0


if __name__ == "__main__":
    raise SystemExit(main())
