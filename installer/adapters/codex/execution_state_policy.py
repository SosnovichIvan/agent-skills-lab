#!/usr/bin/env python3
"""Codex hook guard for an installed execution-state policy."""

from __future__ import annotations

import json
from pathlib import Path
import shlex
import sys


PROJECT_ROOT = Path(__file__).resolve().parents[2]
POLICY_PATH = PROJECT_ROOT / ".agent-skills-lab" / "policy.json"
CONTEXT_PATH = PROJECT_ROOT / ".agent-skills-lab" / "codex-strict-context.md"
RUNTIME_ROOT = PROJECT_ROOT / ".agent-skills-lab" / "runtime"


def read_input() -> dict:
    try:
        return json.load(sys.stdin)
    except (json.JSONDecodeError, OSError):
        return {}


def read_policy() -> dict:
    try:
        value = json.loads(POLICY_PATH.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}
    return value.get("execution_state", {})


def receipt_path(turn_id: str) -> Path:
    safe = "".join(char for char in turn_id if char.isalnum() or char in "-_")
    return RUNTIME_ROOT / f"{safe or 'unknown'}.json"


def command_text(payload: dict) -> str:
    tool_input = payload.get("tool_input")
    if not isinstance(tool_input, dict):
        return ""
    value = tool_input.get("command", tool_input.get("cmd", ""))
    return value if isinstance(value, str) else ""


def is_route_command(command: str, expected_script: Path) -> bool:
    if any(token in command for token in ("&&", "||", ";", "|", "&", ">", "<", "\n", "`", "$(")):
        return False
    try:
        parts = shlex.split(command)
    except ValueError:
        return False
    if len(parts) < 3:
        return False
    try:
        route_at = parts.index("route")
    except ValueError:
        return False
    if route_at < 2:
        return False
    executable = Path(parts[0]).name.lower()
    if executable not in {"python", "python3", "py"} and not executable.startswith("python3."):
        return False
    try:
        supplied = Path(parts[route_at - 1]).expanduser().resolve()
    except OSError:
        return False
    return supplied == expected_script


def route_succeeded(payload: dict) -> bool:
    response = payload.get("tool_response")
    if not isinstance(response, dict):
        return False
    if response.get("isError") is True:
        return False
    exit_code = response.get("exit_code")
    if exit_code is not None:
        return exit_code == 0
    output = response.get("output")
    if isinstance(output, str):
        try:
            return json.loads(output).get("ok") is True
        except json.JSONDecodeError:
            return False
    return response.get("success") is True


def write_json(value: dict) -> None:
    sys.stdout.write(json.dumps(value, ensure_ascii=False))


def main() -> int:
    payload = read_input()
    event = payload.get("hook_event_name") or payload.get("hookEventName")
    turn_id = str(payload.get("turn_id") or "unknown")
    policy = read_policy()
    if policy.get("scope") != "all_tasks" or policy.get("enforcement") != "strict":
        return 0

    skill_path = PROJECT_ROOT / policy.get("skill_path", ".agents/skills/execution-state/SKILL.md")
    statectl = skill_path.parent / "scripts" / "statectl.py"
    receipt = receipt_path(turn_id)

    if event == "UserPromptSubmit":
        RUNTIME_ROOT.mkdir(parents=True, exist_ok=True)
        receipt.write_text(json.dumps({"status": "pending", "turn_id": turn_id}) + "\n", encoding="utf-8")
        try:
            additional_context = CONTEXT_PATH.read_text(encoding="utf-8").strip()
        except OSError:
            additional_context = "Execution-state routing is required before other tool calls."
        write_json(
            {
                "hookSpecificOutput": {
                    "hookEventName": "UserPromptSubmit",
                    "additionalContext": additional_context,
                }
            }
        )
        return 0

    command = command_text(payload)
    route_command = is_route_command(command, statectl)
    status = ""
    try:
        status = json.loads(receipt.read_text(encoding="utf-8")).get("status", "")
    except (OSError, json.JSONDecodeError):
        pass

    if event == "PreToolUse" and status != "routed" and not route_command:
        write_json(
            {
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": "Route this task through execution-state before other tool calls.",
                }
            }
        )
        return 0

    if event == "PostToolUse" and route_command and route_succeeded(payload):
        RUNTIME_ROOT.mkdir(parents=True, exist_ok=True)
        receipt.write_text(json.dumps({"status": "routed", "turn_id": turn_id}) + "\n", encoding="utf-8")
    if event == "Stop" and status != "routed" and not payload.get("stop_hook_active"):
        write_json(
            {
                "decision": "block",
                "reason": "Route the task through execution-state before completing the turn."
            }
        )
    elif event == "Stop":
        receipt.unlink(missing_ok=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
