#!/usr/bin/env python3
"""Create an immutable standalone-only experiment plan and v4 task catalog."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


AREA_BY_GROUP = {
    "1": "foundation",
    "2": "registration-http",
    "3": "access-auth",
    "4": "sessions",
    "5": "account-security",
    "6": "organizations",
    "7": "authorization",
    "8": "operations",
}


def read_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"expected JSON object: {path}")
    return value


def enrich_tasks(tasks: list[dict[str, Any]]) -> dict[str, Any]:
    enriched = []
    for task in tasks:
        task_id = str(task["id"])
        group = task_id.split(".", 1)[0]
        area = AREA_BY_GROUP.get(group, "product")
        integration = task_id == "8.4"
        enriched.append(
            {
                "id": task_id,
                "title": task["title"],
                "done_when": task["done_when"],
                "kind": "integration" if integration else "implementation",
                "cohesion_key": area,
                "affected_areas": [area],
                "reads": [],
                "writes": [],
                "contracts": list(task["done_when"]),
                "regression_checks": [f"task-{task_id}-external-gate"],
                "requires_bridge": task_id == "8.3",
            }
        )
    return {"schema_version": 2, "tasks": enriched}


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def snapshot_skill(repo_root: Path, ref: str, destination: Path, label: str) -> Path:
    destination.mkdir(parents=True, exist_ok=True)
    archive = destination / f"{label}.tar"
    archived = subprocess.run(
        [
            "git",
            "archive",
            "--format=tar",
            "--output",
            str(archive),
            ref,
            "skills/execution-state",
        ],
        cwd=repo_root,
        capture_output=True,
        text=True,
        check=False,
    )
    if archived.returncode != 0:
        raise ValueError(f"cannot archive {label} skill: {archived.stderr.strip()}")
    extracted = destination / label
    extracted.mkdir()
    unpacked = subprocess.run(
        ["tar", "-xf", str(archive), "-C", str(extracted)],
        capture_output=True,
        text=True,
        check=False,
    )
    if unpacked.returncode != 0:
        raise ValueError(f"cannot extract {label} skill: {unpacked.stderr.strip()}")
    skill = extracted / "skills" / "execution-state"
    if not (skill / "SKILL.md").is_file() or not (skill / "scripts" / "statectl.py").is_file():
        raise ValueError(f"invalid {label} skill snapshot")
    return archive


def prepare(root: Path, run_id: str, candidate_ref: str) -> Path:
    quality = Path(__file__).resolve().parent
    benchmark = quality.parent
    profile_path = quality / "experiment.json"
    profile = read_json(profile_path)
    if profile.get("source") != "standalone":
        raise ValueError("quality experiments must be standalone")
    if any("sdd" in variant["id"].lower() or "openspec" in variant["id"].lower() for variant in profile["variants"]):
        raise ValueError("SDD/OpenSpec variants are forbidden in new experiments")
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,95}", run_id):
        raise ValueError("invalid run-id")
    if not re.fullmatch(r"[0-9a-f]{40}", candidate_ref):
        raise ValueError("candidate-ref must be a full committed Git SHA")
    if candidate_ref == profile.get("control_ref"):
        raise ValueError("candidate-ref must differ from control_ref")
    repo_root = quality.parents[2]
    for label, ref in (("control", profile.get("control_ref")), ("candidate", candidate_ref)):
        check = subprocess.run(
            ["git", "cat-file", "-e", f"{ref}^{{commit}}"],
            cwd=repo_root,
            capture_output=True,
            check=False,
        )
        if check.returncode != 0:
            raise ValueError(f"{label} ref is not a committed Git object: {ref}")
    destination = root.resolve() / run_id
    if destination.exists():
        raise ValueError(f"refusing to overwrite experiment: {destination}")
    source_tasks_path = benchmark / "long-session" / "tasks.json"
    source_tasks = read_json(source_tasks_path).get("tasks")
    if not isinstance(source_tasks, list) or len(source_tasks) != 32:
        raise ValueError("expected the fixed 32-task long-session catalog")
    destination.mkdir(parents=True)
    inputs = destination / "inputs"
    control_archive = snapshot_skill(repo_root, profile["control_ref"], inputs, "control")
    candidate_archive = snapshot_skill(repo_root, candidate_ref, inputs, "candidate")
    catalog = enrich_tasks(source_tasks)
    catalog_path = destination / "tasks.json"
    catalog_path.write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    manifest = {
        **profile,
        "candidate_ref": candidate_ref,
        "run_id": run_id,
        "prepared_at": datetime.now(timezone.utc).isoformat(),
        "profile_sha256": sha256(profile_path),
        "task_catalog_sha256": sha256(catalog_path),
        "control_snapshot_sha256": sha256(control_archive),
        "candidate_snapshot_sha256": sha256(candidate_archive),
        "status": "prepared",
    }
    (destination / "manifest.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    return destination


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--candidate-ref", required=True)
    parser.add_argument("--run-id", default=datetime.now(timezone.utc).strftime("quality-v1-%Y%m%dT%H%M%SZ"))
    parser.add_argument("--output-root", type=Path, default=Path("benchmarks/go-auth-service/results/quality-runs"))
    args = parser.parse_args()
    try:
        output = prepare(args.output_root, args.run_id, args.candidate_ref)
    except ValueError as error:
        print(json.dumps({"ok": False, "error": str(error)}))
        return 2
    print(json.dumps({"ok": True, "experiment": str(output)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
