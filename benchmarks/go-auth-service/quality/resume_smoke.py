#!/usr/bin/env python3
"""Exercise an interrupted benchmark resume with local fake workers only."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from run_benchmark import run_codex
from runner_state import RunJournal, atomic_save_json, prepare_interrupted_resume


def read_json(path: Path) -> dict:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise RuntimeError(f"expected JSON object: {path}")
    return value


def fake_command(*, interrupt: bool) -> list[str]:
    if interrupt:
        program = "import sys,time; sys.stdin.read(); time.sleep(5)"
    else:
        program = (
            "import sys; sys.stdin.read(); "
            "print('{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":3,\"output_tokens\":2}}')"
        )
    return [sys.executable, "-c", program]


def run_smoke(output: Path) -> dict:
    output.mkdir(parents=True, exist_ok=True)
    metrics_path = output / "metrics.json"
    log_path = output / "orchestration.jsonl"
    accepted = {"task_id": "fake-1", "status": "complete", "attempts": []}
    metrics = {
        "model_requests": 0,
        "runs": {
            "candidate": {
                "execution_status": "running",
                "quality_outcome": "not_evaluated",
                "chunks": [accepted],
            }
        },
    }
    atomic_save_json(metrics_path, metrics)
    journal = RunJournal(metrics, metrics_path, "candidate", log_path)
    interrupted = run_codex(
        fake_command(interrupt=True),
        "fake task two",
        output,
        0.08,
        output / "interrupted.jsonl",
        output / "interrupted.stderr.log",
        journal,
        heartbeat_seconds=0.02,
    )
    if interrupted["execution_classification"] != "interrupted":
        raise RuntimeError("fake worker was not classified as interrupted")
    run = metrics["runs"]["candidate"]
    run["chunks"].append(
        {
            "task_id": "fake-2",
            "status": "failed",
            "attempts": [{**interrupted, "worker_result": None}],
        }
    )
    run["execution_status"] = "interrupted"
    atomic_save_json(metrics_path, metrics)

    restarted = read_json(metrics_path)
    restarted_run = restarted["runs"]["candidate"]
    resume_index = prepare_interrupted_resume(restarted_run)
    if resume_index != 1 or [item["task_id"] for item in restarted_run["chunks"]] != ["fake-1"]:
        raise RuntimeError("resume did not preserve exactly the completed prefix")
    atomic_save_json(metrics_path, restarted)
    resumed_journal = RunJournal(restarted, metrics_path, "candidate", log_path)
    completed = run_codex(
        fake_command(interrupt=False),
        "fake task two retry",
        output,
        2,
        output / "resumed.jsonl",
        output / "resumed.stderr.log",
        resumed_journal,
        heartbeat_seconds=0.02,
    )
    if completed["execution_classification"] != "complete":
        raise RuntimeError("resumed fake worker did not complete")
    restarted_run["chunks"].append(
        {"task_id": "fake-2", "status": "complete", "attempts": [completed]}
    )
    restarted_run["execution_status"] = "complete"
    restarted_run["quality_outcome"] = "pass"
    atomic_save_json(metrics_path, restarted)
    report = {
        "passed": True,
        "model_requests": 0,
        "resume_index": resume_index,
        "completed_tasks": [item["task_id"] for item in restarted_run["chunks"]],
        "interruption_count": len(restarted_run.get("infrastructure_interruptions", [])),
        "metrics": str(metrics_path),
        "orchestration_log": str(log_path),
    }
    atomic_save_json(output / "resume-smoke-report.json", report)
    return report


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    report = run_smoke(args.output.resolve())
    print(json.dumps(report, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
