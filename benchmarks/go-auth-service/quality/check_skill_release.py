#!/usr/bin/env python3
"""Validate and optionally build the minimal execution-state release archive."""

from __future__ import annotations

import argparse
import hashlib
import io
import json
import tarfile
from pathlib import Path


ALLOWED_FILES = {
    "README.md",
    "SKILL.md",
    "release.json",
    "agents/openai.yaml",
    "references/cli-adapters.md",
    "references/microtask-decomposition.md",
    "references/openspec-integration.md",
    "references/runtime-contract.md",
    "references/state-schema.md",
    "references/worker-protocol.md",
    "scripts/adapters/claude.json",
    "scripts/adapters/codex.json",
    "scripts/adapters/gemini.json",
    "scripts/adapters/manual.json",
    "scripts/cli_adapters.py",
    "scripts/statectl.py",
}


def inventory(skill: Path) -> set[str]:
    return {
        path.relative_to(skill).as_posix()
        for path in skill.rglob("*")
        if path.is_file()
    }


def validate_inventory(skill: Path) -> None:
    actual = inventory(skill)
    missing = sorted(ALLOWED_FILES - actual)
    extra = sorted(actual - ALLOWED_FILES)
    if missing or extra:
        raise ValueError(
            "invalid release inventory: "
            + json.dumps({"missing": missing, "extra": extra})
        )


def build_archive(skill: Path, output: Path) -> str:
    validate_inventory(skill)
    output.parent.mkdir(parents=True, exist_ok=True)
    with tarfile.open(output, "w", format=tarfile.PAX_FORMAT) as archive:
        for relative in sorted(ALLOWED_FILES):
            payload = (skill / relative).read_bytes()
            info = tarfile.TarInfo(f"execution-state/{relative}")
            info.size = len(payload)
            info.mode = 0o755 if relative == "scripts/statectl.py" else 0o644
            info.mtime = 0
            info.uid = 0
            info.gid = 0
            info.uname = ""
            info.gname = ""
            archive.addfile(info, io.BytesIO(payload))
    return hashlib.sha256(output.read_bytes()).hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--skill", type=Path, required=True)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    skill = args.skill.resolve()
    try:
        validate_inventory(skill)
        digest = build_archive(skill, args.output.resolve()) if args.output else None
    except (OSError, ValueError) as error:
        print(json.dumps({"ok": False, "error": str(error)}))
        return 2
    print(
        json.dumps(
            {
                "ok": True,
                "files": len(ALLOWED_FILES),
                "archive": str(args.output.resolve()) if args.output else None,
                "sha256": digest,
            }
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
