#!/usr/bin/env python3
"""Run the four isolated execution-state v3 benchmark variants with Codex."""

from __future__ import annotations

import argparse
import atexit
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


PROTOCOL_VERSION = 3
BENCHMARK_MODEL = "gpt-5.6-luna"
BENCHMARK_REASONING_EFFORT = "medium"
VARIANTS = {
    "01-skill-standalone": {
        "prompt": "01-skill-standalone.md",
        "sdd": False,
        "skill": True,
        "expected_profile": "passthrough",
    },
    "02-ai-only": {
        "prompt": "02-ai-only.md",
        "sdd": False,
        "skill": False,
        "expected_profile": None,
    },
    "03-sdd-skill": {
        "prompt": "03-sdd-skill.md",
        "sdd": True,
        "skill": True,
        "expected_profile": "passthrough",
    },
    "04-sdd-only": {
        "prompt": "04-sdd-only.md",
        "sdd": True,
        "skill": False,
        "expected_profile": None,
    },
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--variant", action="append", choices=VARIANTS)
    parser.add_argument("--timeout", type=int, default=1800)
    parser.add_argument(
        "--run-id",
        default=datetime.now(timezone.utc).strftime("v3-%Y%m%dT%H%M%SZ"),
        help="New or resumable result directory name under results/runs/.",
    )
    return parser.parse_args()


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat()


def safe_run_id(value: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,95}", value):
        raise ValueError("run-id must contain only letters, digits, dot, underscore or dash")
    return value


def parse_usage(
    stdout: str,
) -> tuple[dict[str, int], dict[str, int], dict[str, int]]:
    totals = {
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
    completed = 0
    for line in stdout.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        event_type = str(event.get("type", "unknown"))
        event_counts[event_type] = event_counts.get(event_type, 0) + 1
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
        if event_type != "turn.completed":
            continue
        completed += 1
        usage = event.get("usage", {})
        for key in totals:
            totals[key] += int(usage.get(key, 0))
    if completed == 0:
        raise RuntimeError("Codex JSONL did not contain a turn.completed usage event")
    totals["total_tokens"] = totals["input_tokens"] + totals["output_tokens"]
    return totals, event_counts, item_stats


def save_metrics(path: Path, metrics: dict[str, Any]) -> None:
    temporary = path.with_suffix(".json.tmp")
    temporary.write_text(
        json.dumps(metrics, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    temporary.replace(path)


def _delta(skill_value: float, baseline_value: float) -> dict[str, float]:
    return {
        "absolute": round(skill_value - baseline_value, 3),
        "percent": round((skill_value - baseline_value) / baseline_value * 100, 3),
    }


def derive_comparisons(metrics: dict[str, Any]) -> dict[str, Any]:
    pairs = {
        "standalone": ("01-skill-standalone", "02-ai-only"),
        "openspec": ("03-sdd-skill", "04-sdd-only"),
    }
    result: dict[str, Any] = {}
    for pair_name, (skill_name, baseline_name) in pairs.items():
        skill = metrics["runs"].get(skill_name)
        baseline = metrics["runs"].get(baseline_name)
        if not skill or not baseline:
            continue
        try:
            result[pair_name] = {
                "skill_variant": skill_name,
                "baseline_variant": baseline_name,
                "agent_duration_seconds": _delta(
                    skill["agent_duration_seconds"],
                    baseline["agent_duration_seconds"],
                ),
                "total_tokens": _delta(
                    skill["usage"]["total_tokens"],
                    baseline["usage"]["total_tokens"],
                ),
                "input_tokens": _delta(
                    skill["usage"]["input_tokens"],
                    baseline["usage"]["input_tokens"],
                ),
                "cached_input_tokens": _delta(
                    skill["usage"]["cached_input_tokens"],
                    baseline["usage"]["cached_input_tokens"],
                ),
                "output_tokens": _delta(
                    skill["usage"]["output_tokens"],
                    baseline["usage"]["output_tokens"],
                ),
                "command_executions": _delta(
                    skill["item_stats"]["command_executions"],
                    baseline["item_stats"]["command_executions"],
                ),
                "command_output_chars": _delta(
                    skill["item_stats"]["command_output_chars"],
                    baseline["item_stats"]["command_output_chars"],
                ),
            }
        except (KeyError, TypeError, ZeroDivisionError):
            continue
    return result


def command_version(command: str, *version_args: str) -> str:
    arguments = version_args or ("--version",)
    completed = subprocess.run(
        [command, *arguments],
        text=True,
        capture_output=True,
        timeout=10,
        check=False,
    )
    output = " ".join((completed.stdout + " " + completed.stderr).split())
    return output[:512]


def verify_project(project: Path, cache_dir: Path, timeout: int = 120) -> dict[str, Any]:
    started = time.monotonic()
    environment = os.environ.copy()
    environment["GOCACHE"] = str(cache_dir.resolve())
    cache_dir.mkdir(parents=True, exist_ok=True)
    go_test = subprocess.run(
        ["go", "test", "./..."],
        cwd=project,
        env=environment,
        text=True,
        capture_output=True,
        timeout=timeout,
        check=False,
    )
    go_vet = subprocess.run(
        ["go", "vet", "./..."],
        cwd=project,
        env=environment,
        text=True,
        capture_output=True,
        timeout=timeout,
        check=False,
    )
    go_files = sorted(path for path in project.rglob("*.go") if path.is_file())
    test_files = [path for path in go_files if path.name.endswith("_test.go")]
    if go_files:
        gofmt = subprocess.run(
            ["gofmt", "-l", *[str(path) for path in go_files]],
            text=True,
            capture_output=True,
            timeout=timeout,
            check=False,
        )
        unformatted = [line for line in gofmt.stdout.splitlines() if line.strip()]
        gofmt_exit_code = gofmt.returncode
    else:
        gofmt_exit_code = 1
        unformatted = []
    source = "\n".join(path.read_text(encoding="utf-8") for path in go_files)
    expected_markers = {
        "register": "/register" in source,
        "login": "/login" in source,
        "me": "/me" in source,
        "hmac_sha256": "hmac.New(sha256.New" in source,
        "constant_time": "subtle.ConstantTimeCompare" in source,
        "graceful_shutdown": ".Shutdown(" in source,
    }
    sdd_tasks = project / "openspec" / "changes" / "auth-service" / "tasks.md"
    checkbox = None
    if sdd_tasks.is_file():
        task_text = sdd_tasks.read_text(encoding="utf-8")
        checkbox = {
            "complete": len(re.findall(r"^\s*[-*]\s+\[[xX]\]", task_text, re.MULTILINE)),
            "pending": len(re.findall(r"^\s*[-*]\s+\[ \]", task_text, re.MULTILINE)),
        }
    openspec_complete = checkbox is None or checkbox["pending"] == 0
    go_mod_present = (project / "go.mod").is_file()
    go_sum = project / "go.sum"
    standard_library_only = not go_sum.exists() or not go_sum.read_text(
        encoding="utf-8"
    ).strip()
    passed = (
        go_test.returncode == 0
        and go_vet.returncode == 0
        and gofmt_exit_code == 0
        and go_mod_present
        and bool(go_files)
        and not test_files
        and not unformatted
        and all(expected_markers.values())
        and openspec_complete
        and standard_library_only
    )
    return {
        "passed": passed,
        "duration_seconds": round(time.monotonic() - started, 3),
        "go_test": {
            "exit_code": go_test.returncode,
            "stdout": go_test.stdout[-2000:],
            "stderr": go_test.stderr[-2000:],
        },
        "go_vet": {
            "exit_code": go_vet.returncode,
            "stdout": go_vet.stdout[-2000:],
            "stderr": go_vet.stderr[-2000:],
        },
        "go_files": len(go_files),
        "go_mod_present": go_mod_present,
        "standard_library_only": standard_library_only,
        "gofmt_exit_code": gofmt_exit_code,
        "test_files": [str(path.relative_to(project)) for path in test_files],
        "unformatted": [str(Path(path).relative_to(project)) for path in unformatted],
        "expected_markers": expected_markers,
        "openspec_tasks": checkbox,
    }


def main() -> int:
    args = parse_args()
    try:
        run_id = safe_run_id(args.run_id)
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    benchmark_root = Path(__file__).resolve().parents[1]
    repo_root = benchmark_root.parents[1]
    skill_path = repo_root / "skills" / "execution-state"
    prompts_dir = benchmark_root / "prompts" / "v3"
    run_root = benchmark_root / "results" / "runs" / run_id
    projects_dir = run_root / "projects"
    raw_dir = run_root / "raw"
    cache_dir = Path(tempfile.mkdtemp(prefix=f"{run_id}-go-build-"))
    atexit.register(shutil.rmtree, cache_dir, ignore_errors=True)
    projects_dir.mkdir(parents=True, exist_ok=True)
    raw_dir.mkdir(parents=True, exist_ok=True)
    metrics_path = run_root / "metrics.json"

    if metrics_path.exists():
        metrics = json.loads(metrics_path.read_text(encoding="utf-8"))
        identity = (
            metrics.get("protocol_version"),
            metrics.get("model"),
            metrics.get("reasoning_effort"),
        )
        expected = (
            PROTOCOL_VERSION,
            BENCHMARK_MODEL,
            BENCHMARK_REASONING_EFFORT,
        )
        if identity != expected:
            print(f"error: run-id already belongs to {identity}, expected {expected}", file=sys.stderr)
            return 2
    else:
        metrics = {
            "benchmark": "go-auth-service",
            "protocol_version": PROTOCOL_VERSION,
            "run_id": run_id,
            "model": BENCHMARK_MODEL,
            "reasoning_effort": BENCHMARK_REASONING_EFFORT,
            "started_at": utc_now(),
            "codex_version": command_version("codex"),
            "python_version": sys.version.split()[0],
            "go_version": command_version("go", "version"),
            "method": "one fresh codex exec per variant; skill variants route one semantic chunk to passthrough",
            "runs": {},
        }
        save_metrics(metrics_path, metrics)

    selected = args.variant or list(VARIANTS)
    any_failed = False
    base_prompt = (benchmark_root / "prompts" / "base-task.md").read_text(
        encoding="utf-8"
    )
    for name in selected:
        previous = metrics["runs"].get(name)
        if previous and previous.get("status") == "complete":
            print(f"skipping completed {name}", flush=True)
            continue
        project = projects_dir / name
        if project.exists():
            print(
                f"error: incomplete project already exists; use a new run-id: {project}",
                file=sys.stderr,
            )
            return 2
        project.mkdir(parents=True)
        variant = VARIANTS[name]
        if variant["sdd"]:
            shutil.copytree(benchmark_root / "fixtures" / "openspec", project / "openspec")

        variant_prompt = (prompts_dir / variant["prompt"]).read_text(encoding="utf-8")
        prompt = variant_prompt.replace("{SKILL_PATH}", str(skill_path.resolve()))
        prompt += "\n\n" + base_prompt
        prompt_path = raw_dir / f"{name}.prompt.md"
        prompt_path.write_text(prompt, encoding="utf-8")
        stdout_path = raw_dir / f"{name}.jsonl"
        stderr_path = raw_dir / f"{name}.stderr.log"
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
            "--color",
            "never",
            "-m",
            BENCHMARK_MODEL,
            "-c",
            f'model_reasoning_effort="{BENCHMARK_REASONING_EFFORT}"',
            "-C",
            str(project),
            "-o",
            str(last_message),
            "-",
        ]
        metrics["runs"][name] = {
            "status": "running",
            "started_at": utc_now(),
            "project": str(project.relative_to(repo_root)),
            "skill": variant["skill"],
            "sdd": variant["sdd"],
            "expected_profile": variant["expected_profile"],
        }
        save_metrics(metrics_path, metrics)
        print(f"running {name}...", flush=True)
        started = time.monotonic()
        try:
            completed = subprocess.run(
                command,
                input=prompt,
                text=True,
                capture_output=True,
                timeout=args.timeout,
                check=False,
            )
            agent_duration = time.monotonic() - started
        except subprocess.TimeoutExpired as exc:
            stdout = exc.stdout if isinstance(exc.stdout, str) else ""
            stderr = exc.stderr if isinstance(exc.stderr, str) else ""
            stdout_path.write_text(stdout, encoding="utf-8")
            stderr_path.write_text(stderr, encoding="utf-8")
            metrics["runs"][name].update(
                {
                    "status": "timeout",
                    "agent_duration_seconds": round(time.monotonic() - started, 3),
                }
            )
            save_metrics(metrics_path, metrics)
            print(f"timeout: {name}", file=sys.stderr)
            any_failed = True
            continue

        stdout_path.write_text(completed.stdout, encoding="utf-8")
        stderr_path.write_text(completed.stderr, encoding="utf-8")
        try:
            usage, event_counts, item_stats = parse_usage(completed.stdout)
        except RuntimeError as exc:
            usage = {"error": str(exc)}
            event_counts = {}
            item_stats = {}
        verification = verify_project(project, cache_dir)
        files = sorted(
            str(path.relative_to(project))
            for path in project.rglob("*")
            if path.is_file()
        )
        state_created = (project / ".execution-state").exists()
        status = (
            "complete"
            if completed.returncode == 0 and verification["passed"]
            else "failed"
        )
        metrics["runs"][name].update(
            {
                "status": status,
                "finished_at": utc_now(),
                "exit_code": completed.returncode,
                "agent_duration_seconds": round(agent_duration, 3),
                "total_duration_seconds": round(
                    agent_duration + verification["duration_seconds"], 3
                ),
                "usage": usage,
                "event_counts": event_counts,
                "item_stats": item_stats,
                "verification": verification,
                "execution_state_created": state_created,
                "profile_observed": (
                    "stateful" if state_created else "passthrough-or-no-skill"
                ),
                "files_created": len(files),
            }
        )
        save_metrics(metrics_path, metrics)
        print(
            f"completed {name}: status={status}, exit={completed.returncode}, "
            f"seconds={agent_duration:.3f}, usage={usage}",
            flush=True,
        )
        if status != "complete":
            any_failed = True

    metrics["finished_at"] = utc_now()
    metrics["comparisons"] = derive_comparisons(metrics)
    save_metrics(metrics_path, metrics)
    print(metrics_path)
    return 1 if any_failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
