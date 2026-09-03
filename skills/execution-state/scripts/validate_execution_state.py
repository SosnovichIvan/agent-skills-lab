#!/usr/bin/env python3
"""Validate global execution state and all referenced task files."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from state_lib import (
    MAX_STATE_BYTES,
    StateError,
    load_json,
    resolve_state_file,
    validate_state,
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Validate execution state v2")
    parser.add_argument("--change", help="OpenSpec change id")
    parser.add_argument("--state", help="Explicit path to state.json")
    parser.add_argument("--project-root", default=".", help="Project root")
    parser.add_argument("--json", action="store_true", help="Emit JSON result")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = Path(args.project_root).resolve()
    try:
        target = resolve_state_file(root, args.change, args.state)
        errors: list[str] = []
        if target.stat().st_size > MAX_STATE_BYTES:
            errors.append(f"state.json exceeds {MAX_STATE_BYTES} bytes")
        state = load_json(target)
        errors.extend(validate_state(state, target))
    except (OSError, StateError) as exc:
        errors = [str(exc)]
        target = Path(args.state or args.change or "<unknown>")

    if args.json:
        print(
            json.dumps(
                {"valid": not errors, "state": str(target), "errors": errors},
                ensure_ascii=False,
                indent=2,
            )
        )
    elif errors:
        print(f"INVALID: {target}", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
    else:
        print(f"VALID: {target}")
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
