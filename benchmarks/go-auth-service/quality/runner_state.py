#!/usr/bin/env python3
"""Durable lifecycle helpers for the quality benchmark runner."""

from __future__ import annotations

import json
import os
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


EXECUTION_STATUSES = {
    "preparing",
    "running",
    "complete",
    "interrupted",
    "infra_error",
}
QUALITY_OUTCOMES = {"pass", "fail", "not_evaluated"}
TERMINAL_EXECUTION_STATUSES = {"complete", "interrupted", "infra_error"}


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat()


def parse_time(value: str) -> datetime:
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise ValueError("timestamp must include a timezone")
    return parsed.astimezone(timezone.utc)


def validate_run_state(run: dict[str, Any]) -> None:
    execution = run.get("execution_status")
    quality = run.get("quality_outcome")
    if execution not in EXECUTION_STATUSES:
        raise ValueError(f"invalid execution_status: {execution!r}")
    if quality not in QUALITY_OUTCOMES:
        raise ValueError(f"invalid quality_outcome: {quality!r}")
    if execution in {"preparing", "running", "interrupted", "infra_error"} and quality != "not_evaluated":
        raise ValueError(f"{execution} execution requires quality_outcome=not_evaluated")


def classify_process(returncode: int, *, timed_out: bool = False) -> str:
    if timed_out or returncode < 0:
        return "interrupted"
    if returncode == 0:
        return "complete"
    return "infra_error"


def is_stale(run: dict[str, Any], *, now: datetime, ttl_seconds: int) -> bool:
    if ttl_seconds < 1:
        raise ValueError("ttl_seconds must be positive")
    if run.get("execution_status") not in {"preparing", "running"}:
        return False
    active = run.get("active_attempt")
    timestamp = active.get("heartbeat_at") if isinstance(active, dict) else run.get("started_at")
    if not isinstance(timestamp, str):
        return True
    return (now.astimezone(timezone.utc) - parse_time(timestamp)).total_seconds() > ttl_seconds


def mark_stale_interrupted(run: dict[str, Any], *, now: datetime, ttl_seconds: int) -> bool:
    if not is_stale(run, now=now, ttl_seconds=ttl_seconds):
        return False
    run["execution_status"] = "interrupted"
    run["quality_outcome"] = "not_evaluated"
    run["interruption"] = {
        "kind": "stale_heartbeat",
        "detected_at": now.astimezone(timezone.utc).isoformat(),
        "ttl_seconds": ttl_seconds,
    }
    validate_run_state(run)
    return True


def prepare_interrupted_resume(run: dict[str, Any]) -> int:
    """Return the next task index without repeating accepted chunks."""
    if run.get("execution_status") != "interrupted":
        raise ValueError("only interrupted runs can be resumed")
    chunks = run.get("chunks")
    if not isinstance(chunks, list):
        raise ValueError("run chunks must be a list")
    active_attempt = run.pop("active_attempt", None)
    if isinstance(active_attempt, dict):
        run.setdefault("interrupted_attempts", []).append(active_attempt)
    completed = 0
    for chunk in chunks:
        if not isinstance(chunk, dict):
            raise ValueError("chunk must be an object")
        if chunk.get("status") == "complete":
            completed += 1
            continue
        break
    if any(chunk.get("status") == "complete" for chunk in chunks[completed + 1 :]):
        raise ValueError("completed chunks must form a durable prefix")
    partial = chunks[completed:]
    if len(partial) > 1:
        raise ValueError("run contains more than one partial chunk")
    if partial:
        attempts = partial[0].get("attempts") or []
        retryable = bool(attempts) and all(
            attempt.get("execution_classification") in {"interrupted", "infra_error"}
            and attempt.get("worker_result") is None
            for attempt in attempts
        )
        if not retryable:
            raise ValueError("partial chunk is not an infrastructure interruption")
        run.setdefault("infrastructure_interruptions", []).append(partial[0])
        del chunks[completed:]
    run["execution_status"] = "running"
    run["quality_outcome"] = "not_evaluated"
    run.pop("interruption", None)
    validate_run_state(run)
    return completed


def atomic_save_json(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    temporary.replace(path)


class RunJournal:
    """Persist one run's active attempt and append-only orchestration events."""

    def __init__(
        self,
        metrics: dict[str, Any],
        metrics_path: Path,
        run_name: str,
        log_path: Path,
    ) -> None:
        self.metrics = metrics
        self.metrics_path = metrics_path
        self.run_name = run_name
        self.log_path = log_path

    @property
    def run(self) -> dict[str, Any]:
        value = self.metrics["runs"][self.run_name]
        if not isinstance(value, dict):
            raise ValueError("run record must be an object")
        return value

    def _log(self, event: dict[str, Any]) -> None:
        self.log_path.parent.mkdir(parents=True, exist_ok=True)
        with self.log_path.open("a", encoding="utf-8") as stream:
            stream.write(json.dumps(event, ensure_ascii=False, separators=(",", ":")) + "\n")
            stream.flush()
            os.fsync(stream.fileno())

    def started(
        self,
        *,
        attempt_id: str,
        pid: int,
        command: list[str],
        stdout_path: Path,
        stderr_path: Path,
    ) -> None:
        now = utc_now()
        self._log(
            {
                "event": "subprocess_started",
                "run": self.run_name,
                "at": now,
                "attempt_id": attempt_id,
                "pid": pid,
                "stdout": str(stdout_path),
                "stderr": str(stderr_path),
            }
        )
        self.run["execution_status"] = "running"
        self.run["quality_outcome"] = "not_evaluated"
        self.run["active_attempt"] = {
            "id": attempt_id,
            "pid": pid,
            "runner_pid": os.getpid(),
            "started_at": now,
            "heartbeat_at": now,
            "command": command,
        }
        validate_run_state(self.run)
        atomic_save_json(self.metrics_path, self.metrics)

    def heartbeat(self) -> None:
        active = self.run.get("active_attempt")
        if not isinstance(active, dict):
            raise ValueError("cannot heartbeat without active_attempt")
        active["heartbeat_at"] = utc_now()
        atomic_save_json(self.metrics_path, self.metrics)

    def spawn_failed(
        self,
        *,
        attempt_id: str,
        command: list[str],
        stdout_path: Path,
        stderr_path: Path,
        error: str,
    ) -> None:
        now = utc_now()
        self._log(
            {
                "event": "subprocess_spawn_failed",
                "run": self.run_name,
                "at": now,
                "attempt_id": attempt_id,
                "stdout": str(stdout_path),
                "stderr": str(stderr_path),
                "error": error,
            }
        )
        self.run["execution_status"] = "infra_error"
        self.run["quality_outcome"] = "not_evaluated"
        self.run["last_attempt"] = {
            "id": attempt_id,
            "command": command,
            "finished_at": now,
            "classification": "infra_error",
            "spawn_error": error,
        }
        self.run.pop("active_attempt", None)
        validate_run_state(self.run)
        atomic_save_json(self.metrics_path, self.metrics)

    def finished(self, *, classification: str, returncode: int, timed_out: bool) -> None:
        now = utc_now()
        active = self.run.get("active_attempt")
        attempt_id = active.get("id") if isinstance(active, dict) else None
        self._log(
            {
                "event": "subprocess_finished",
                "run": self.run_name,
                "at": now,
                "attempt_id": attempt_id,
                "classification": classification,
                "returncode": returncode,
                "timed_out": timed_out,
            }
        )
        if isinstance(active, dict):
            self.run["last_attempt"] = {
                **active,
                "finished_at": now,
                "classification": classification,
                "returncode": returncode,
                "timed_out": timed_out,
            }
        self.run.pop("active_attempt", None)
        atomic_save_json(self.metrics_path, self.metrics)
