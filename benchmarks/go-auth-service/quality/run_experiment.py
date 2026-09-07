#!/usr/bin/env python3
"""Run the committed standalone-only quality experiment in fixed Latin order."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from runner_state import atomic_save_json, mark_stale_interrupted, validate_run_state

VARIANT_CONFIG = {
    "control-skill-previous": ("01-skill-standalone", "control", "v2"),
    "candidate-skill-current": ("01-skill-standalone", "candidate", "v3"),
    "ai-only": ("02-ai-only", "none", "none"),
}


def read_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise RuntimeError(f"expected JSON object: {path}")
    return value


def run_snapshot(
    metrics_path: Path,
    runner_variant: str,
    *,
    stale_ttl: int,
    now: datetime | None = None,
) -> tuple[str, str] | None:
    if not metrics_path.is_file():
        return None
    metrics = read_json(metrics_path)
    run = metrics.get("runs", {}).get(runner_variant, {})
    if not isinstance(run, dict) or not run:
        return None
    if mark_stale_interrupted(
        run,
        now=now or datetime.now(timezone.utc),
        ttl_seconds=stale_ttl,
    ):
        atomic_save_json(metrics_path, metrics)
    validate_run_state(run)
    return run["execution_status"], run["quality_outcome"]


def wait_existing(
    metrics_path: Path,
    runner_variant: str,
    *,
    stale_ttl: int,
    poll_seconds: float,
) -> tuple[str, str]:
    while True:
        snapshot = run_snapshot(
            metrics_path,
            runner_variant,
            stale_ttl=stale_ttl,
        )
        if snapshot is None:
            raise RuntimeError("existing run disappeared")
        execution, _ = snapshot
        if execution in {"complete", "interrupted", "infra_error"}:
            return snapshot
        time.sleep(poll_seconds)


def write_validity_report(experiment: Path, manifest: dict[str, Any]) -> Path:
    entries: list[dict[str, Any]] = []
    for repeat, order in enumerate(manifest["run_orders"], start=1):
        for variant_id in order:
            runner_variant, _, _ = VARIANT_CONFIG[variant_id]
            short = {
                "control-skill-previous": "control",
                "candidate-skill-current": "candidate",
                "ai-only": "ai",
            }[variant_id]
            run_id = f"repeat-{repeat:02d}-{short}"
            metrics_path = experiment / "runs" / run_id / "metrics.json"
            snapshot = run_snapshot(metrics_path, runner_variant, stale_ttl=60) if metrics_path.is_file() else None
            entries.append(
                {
                    "run_id": run_id,
                    "variant": variant_id,
                    "execution_status": snapshot[0] if snapshot else "missing",
                    "quality_outcome": snapshot[1] if snapshot else "not_evaluated",
                    "comparison_eligible": bool(snapshot and snapshot[0] == "complete"),
                }
            )
    report = {
        "protocol_valid": manifest.get("protocol_version") == 5,
        "infrastructure_interruptions": [
            item["run_id"] for item in entries
            if item["execution_status"] in {"interrupted", "infra_error", "missing"}
        ],
        "product_quality": {
            item["run_id"]: item["quality_outcome"]
            for item in entries
            if item["execution_status"] == "complete"
        },
        "comparison_runs": [item["run_id"] for item in entries if item["comparison_eligible"]],
        "runs": entries,
    }
    path = experiment / "validity-report.json"
    atomic_save_json(path, report)
    return path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--experiment", type=Path, required=True)
    parser.add_argument("--timeout", type=int, default=1200)
    parser.add_argument("--stale-ttl", type=int, default=60)
    parser.add_argument("--poll-seconds", type=float, default=5.0)
    args = parser.parse_args()
    experiment = args.experiment.resolve()
    manifest = read_json(experiment / "manifest.json")
    if manifest.get("source") != "standalone":
        raise RuntimeError("only standalone experiments are allowed")
    if manifest.get("protocol_version") != 5:
        raise RuntimeError("experiment protocol_version must equal 5")
    if manifest.get("model") != "gpt-5.6-luna" or manifest.get("reasoning_effort") != "medium":
        raise RuntimeError("experiment model profile is not the fixed control profile")

    repo = Path(__file__).resolve().parents[3]
    runner = repo / "benchmarks" / "go-auth-service" / "quality" / "run_benchmark.py"
    tasks = experiment / "tasks.json"
    output_root = experiment / "runs"
    behavior = repo / "benchmarks" / "go-auth-service" / "quality" / "verify_behavior.py"
    schemas = {
        "v2": repo / "benchmarks" / "go-auth-service" / "quality" / "worker-result-v2.schema.json",
        "v3": repo / "benchmarks" / "go-auth-service" / "quality" / "worker-result.schema.json",
    }
    skills = {
        "control": experiment / "inputs" / "control" / "skills" / "execution-state",
        "candidate": experiment / "inputs" / "candidate" / "skills" / "execution-state",
    }
    for label, skill in skills.items():
        if not (skill / "SKILL.md").is_file() or not (skill / "scripts" / "statectl.py").is_file():
            raise RuntimeError(f"missing {label} skill snapshot: {skill}")
    execution_failures: list[str] = []
    quality_failures: list[str] = []

    for repeat, order in enumerate(manifest["run_orders"], start=1):
        for variant_id in order:
            runner_variant, skill_kind, schema_kind = VARIANT_CONFIG[variant_id]
            short = {"control-skill-previous": "control", "candidate-skill-current": "candidate", "ai-only": "ai"}[variant_id]
            run_id = f"repeat-{repeat:02d}-{short}"
            metrics_path = output_root / run_id / "metrics.json"
            snapshot = run_snapshot(
                metrics_path,
                runner_variant,
                stale_ttl=args.stale_ttl,
            )
            if snapshot and snapshot[0] == "complete":
                print(f"skip complete {run_id} quality={snapshot[1]}", flush=True)
                if snapshot[1] == "fail":
                    quality_failures.append(run_id)
                continue
            if snapshot and snapshot[0] in {"preparing", "running"}:
                print(f"wait existing {run_id}", flush=True)
                snapshot = wait_existing(
                    metrics_path,
                    runner_variant,
                    stale_ttl=args.stale_ttl,
                    poll_seconds=args.poll_seconds,
                )
                if snapshot[0] == "complete":
                    if snapshot[1] == "fail":
                        quality_failures.append(run_id)
                    continue
            if snapshot and snapshot[0] == "infra_error":
                print(f"record infrastructure failure {run_id}; continue experiment", flush=True)
                execution_failures.append(run_id)
                continue
            command = [
                sys.executable, str(runner), "--variant", runner_variant,
                "--tasks-file", str(tasks), "--output-root", str(output_root),
                "--behavior-verifier", str(behavior), "--run-id", run_id,
                "--timeout", str(args.timeout),
            ]
            if skill_kind != "none":
                command.extend(["--skill-path", str(skills[skill_kind])])
                command.extend(["--worker-schema", str(schemas[schema_kind])])
            if snapshot and snapshot[0] == "interrupted":
                command.append("--retry-infrastructure-failure")
            print(f"start {run_id}: {variant_id}", flush=True)
            completed = subprocess.run(command, cwd=repo, check=False)
            final = run_snapshot(
                metrics_path,
                runner_variant,
                stale_ttl=args.stale_ttl,
            )
            if completed.returncode != 0 or final is None or final[0] != "complete":
                print(f"record execution failure {run_id} (exit {completed.returncode}); continue experiment", flush=True)
                execution_failures.append(run_id)
            elif final[1] == "fail":
                quality_failures.append(run_id)
    print(experiment, flush=True)
    if quality_failures:
        print(f"quality failures: {', '.join(quality_failures)}", flush=True)
    if execution_failures:
        print(f"execution failures: {', '.join(execution_failures)}", flush=True)
        write_validity_report(experiment, manifest)
        return 1
    write_validity_report(experiment, manifest)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
