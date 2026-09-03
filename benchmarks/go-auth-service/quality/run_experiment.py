#!/usr/bin/env python3
"""Run the committed standalone-only quality experiment in fixed Latin order."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from pathlib import Path
from typing import Any


VARIANT_CONFIG = {
    "control-skill-v3": ("01-skill-standalone", "control", "v1"),
    "candidate-skill-v4": ("01-skill-standalone", "candidate", "v2"),
    "ai-only": ("02-ai-only", "none", "v2"),
}


def read_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise RuntimeError(f"expected JSON object: {path}")
    return value


def run_status(metrics_path: Path, runner_variant: str) -> str | None:
    if not metrics_path.is_file():
        return None
    metrics = read_json(metrics_path)
    run = metrics.get("runs", {}).get(runner_variant, {})
    return run.get("status") if isinstance(run, dict) else None


def wait_existing(metrics_path: Path, runner_variant: str) -> None:
    while True:
        status = run_status(metrics_path, runner_variant)
        if status == "complete":
            return
        if status == "failed":
            raise RuntimeError(f"existing run failed: {metrics_path.parent.name}")
        if status not in {"preparing", "running"}:
            raise RuntimeError(f"cannot resume existing run in status {status!r}")
        time.sleep(10)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--experiment", type=Path, required=True)
    parser.add_argument("--timeout", type=int, default=1200)
    args = parser.parse_args()
    experiment = args.experiment.resolve()
    manifest = read_json(experiment / "manifest.json")
    if manifest.get("source") != "standalone":
        raise RuntimeError("only standalone experiments are allowed")
    if manifest.get("model") != "gpt-5.6-luna" or manifest.get("reasoning_effort") != "medium":
        raise RuntimeError("experiment model profile is not the fixed control profile")

    repo = Path(__file__).resolve().parents[3]
    runner = repo / "benchmarks" / "go-auth-service" / "long-session" / "run_long_benchmark.py"
    tasks = experiment / "tasks.json"
    output_root = experiment / "runs"
    behavior = repo / "benchmarks" / "go-auth-service" / "quality" / "verify_behavior.py"
    schemas = {
        "v1": repo / "benchmarks" / "go-auth-service" / "long-session" / "worker-result.schema.json",
        "v2": repo / "benchmarks" / "go-auth-service" / "quality" / "worker-result.schema.json",
    }
    skills = {
        "control": experiment / "inputs" / "skills" / "execution-state",
        "candidate": repo / "skills" / "execution-state",
    }

    for repeat, order in enumerate(manifest["run_orders"], start=1):
        for variant_id in order:
            runner_variant, skill_kind, schema_kind = VARIANT_CONFIG[variant_id]
            short = {"control-skill-v3": "control", "candidate-skill-v4": "candidate", "ai-only": "ai"}[variant_id]
            run_id = f"repeat-{repeat:02d}-{short}"
            metrics_path = output_root / run_id / "metrics.json"
            status = run_status(metrics_path, runner_variant)
            if status == "complete":
                print(f"skip complete {run_id}", flush=True)
                continue
            if status in {"preparing", "running"}:
                print(f"wait existing {run_id}", flush=True)
                wait_existing(metrics_path, runner_variant)
                continue
            if status == "failed":
                raise RuntimeError(f"run requires inspection before restart: {run_id}")
            command = [
                sys.executable, str(runner), "--variant", runner_variant,
                "--tasks-file", str(tasks), "--output-root", str(output_root),
                "--behavior-verifier", str(behavior), "--run-id", run_id,
                "--timeout", str(args.timeout),
            ]
            if skill_kind != "none":
                command.extend(["--skill-path", str(skills[skill_kind])])
                command.extend(["--worker-schema", str(schemas[schema_kind])])
            print(f"start {run_id}: {variant_id}", flush=True)
            completed = subprocess.run(command, cwd=repo, check=False)
            if completed.returncode != 0:
                raise RuntimeError(f"run failed: {run_id} (exit {completed.returncode})")
    print(experiment, flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
