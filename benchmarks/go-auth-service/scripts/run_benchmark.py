#!/usr/bin/env python3
"""Run isolated Codex benchmark variants and collect wall time and token usage."""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path


VARIANTS = {
    "01-skill-standalone": {"prompt": "01-skill-standalone.md", "sdd": False},
    "02-ai-only": {"prompt": "02-ai-only.md", "sdd": False},
    "03-sdd-skill": {"prompt": "03-sdd-skill.md", "sdd": True},
    "04-sdd-only": {"prompt": "04-sdd-only.md", "sdd": True},
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--variant", action="append", choices=VARIANTS)
    parser.add_argument("--model", default="gpt-5.6-luna")
    parser.add_argument("--reasoning-effort", default="medium")
    parser.add_argument("--timeout", type=int, default=1800)
    return parser.parse_args()


def parse_usage(stdout: str) -> dict[str, int]:
    totals = {
        "input_tokens": 0,
        "cached_input_tokens": 0,
        "cache_write_input_tokens": 0,
        "output_tokens": 0,
        "reasoning_output_tokens": 0,
    }
    completed = 0
    for line in stdout.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if event.get("type") != "turn.completed":
            continue
        completed += 1
        usage = event.get("usage", {})
        for key in totals:
            totals[key] += int(usage.get(key, 0))
    if completed == 0:
        raise RuntimeError("Codex JSONL did not contain a turn.completed usage event")
    totals["total_tokens"] = totals["input_tokens"] + totals["output_tokens"]
    return totals


def main() -> int:
    args = parse_args()
    benchmark_root = Path(__file__).resolve().parents[1]
    repo_root = benchmark_root.parents[1]
    skill_path = repo_root / "skills" / "execution-state"
    prompts_dir = benchmark_root / "prompts"
    projects_dir = benchmark_root / "projects"
    results_dir = benchmark_root / "results"
    raw_dir = results_dir / "raw"
    projects_dir.mkdir(parents=True, exist_ok=True)
    raw_dir.mkdir(parents=True, exist_ok=True)

    metrics_path = results_dir / "metrics.json"
    if metrics_path.exists():
        metrics = json.loads(metrics_path.read_text(encoding="utf-8"))
    else:
        metrics = {
            "benchmark": "go-auth-service",
            "model": args.model,
            "reasoning_effort": args.reasoning_effort,
            "runs": {},
        }

    selected = args.variant or list(VARIANTS)
    base_prompt = (prompts_dir / "base-task.md").read_text(encoding="utf-8")

    for name in selected:
        project = projects_dir / name
        if project.exists():
            print(f"error: project already exists, refusing non-fresh run: {project}", file=sys.stderr)
            return 2
        project.mkdir(parents=True)
        if VARIANTS[name]["sdd"]:
            shutil.copytree(benchmark_root / "fixtures" / "openspec", project / "openspec")

        variant_prompt = (prompts_dir / VARIANTS[name]["prompt"]).read_text(
            encoding="utf-8"
        )
        prompt = variant_prompt.replace("{SKILL_PATH}", str(skill_path))
        prompt += "\n\n" + base_prompt
        (raw_dir / f"{name}.prompt.md").write_text(prompt, encoding="utf-8")

        last_message = raw_dir / f"{name}.last-message.md"
        command = [
            "codex",
            "-a",
            "never",
            "-s",
            "workspace-write",
            "exec",
            "--ephemeral",
            "--ignore-user-config",
            "--ignore-rules",
            "--skip-git-repo-check",
            "--json",
            "-m",
            args.model,
            "-c",
            f'model_reasoning_effort="{args.reasoning_effort}"',
            "-C",
            str(project),
            "-o",
            str(last_message),
            prompt,
        ]

        print(f"running {name}...", flush=True)
        started_at = datetime.now(timezone.utc)
        started = time.monotonic()
        completed = subprocess.run(
            command,
            text=True,
            capture_output=True,
            timeout=args.timeout,
            check=False,
        )
        elapsed = time.monotonic() - started
        (raw_dir / f"{name}.jsonl").write_text(completed.stdout, encoding="utf-8")
        (raw_dir / f"{name}.stderr.log").write_text(completed.stderr, encoding="utf-8")

        try:
            usage = parse_usage(completed.stdout)
        except RuntimeError as exc:
            usage = {"error": str(exc)}
        files = sorted(
            str(path.relative_to(project))
            for path in project.rglob("*")
            if path.is_file()
        )
        metrics["runs"][name] = {
            "started_at": started_at.isoformat(),
            "duration_seconds": round(elapsed, 3),
            "exit_code": completed.returncode,
            "usage": usage,
            "project": str(project.relative_to(repo_root)),
            "files_created": len(files),
        }
        metrics_path.write_text(
            json.dumps(metrics, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
        print(
            f"completed {name}: exit={completed.returncode}, "
            f"seconds={elapsed:.3f}, usage={usage}",
            flush=True,
        )
        if completed.returncode != 0:
            return completed.returncode
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
