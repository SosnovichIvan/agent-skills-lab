#!/usr/bin/env python3
"""Black-box quality gate for generated standalone IAM projects."""

from __future__ import annotations

import argparse
import json
import os
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any


@dataclass
class Response:
    status: int
    headers: dict[str, str]
    body: bytes

    def json(self) -> dict[str, Any]:
        try:
            value = json.loads(self.body)
        except (UnicodeDecodeError, json.JSONDecodeError):
            return {}
        return value if isinstance(value, dict) else {}


def _request(
    base_url: str,
    method: str,
    path: str,
    body: dict[str, Any] | bytes | None = None,
    *,
    token: str | None = None,
    headers: dict[str, str] | None = None,
    timeout: float = 5,
) -> Response:
    payload = None
    request_headers = {"Accept": "application/json", **(headers or {})}
    if isinstance(body, dict):
        payload = json.dumps(body, separators=(",", ":")).encode()
        request_headers["Content-Type"] = "application/json"
    elif isinstance(body, bytes):
        payload = body
        request_headers["Content-Type"] = "application/json"
    if token:
        request_headers["Authorization"] = f"Bearer {token}"
    request = urllib.request.Request(
        base_url + path,
        data=payload,
        headers=request_headers,
        method=method,
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return Response(response.status, dict(response.headers.items()), response.read())
    except urllib.error.HTTPError as error:
        return Response(error.code, dict(error.headers.items()), error.read())


def _data(response: Response) -> Any:
    return response.json().get("data")


def _field(value: Any, *names: str) -> Any:
    if not isinstance(value, dict):
        return None
    for name in names:
        if name in value:
            return value[name]
    return None


def _free_port() -> int:
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return int(listener.getsockname()[1])


def _check(name: str, passed: bool, detail: str) -> dict[str, Any]:
    return {"name": name, "passed": bool(passed), "detail": detail[:500]}


def exercise(base_url: str) -> list[dict[str, Any]]:
    suffix = uuid.uuid4().hex[:12]
    owner_email = f"owner-{suffix}@example.test"
    member_email = f"member-{suffix}@example.test"
    password = "correct-horse-battery-staple"
    checks: list[dict[str, Any]] = []

    invalid = _request(
        base_url,
        "POST",
        "/v1/login",
        b'{"email":',
    )
    error_id = _field(invalid.json().get("error"), "request_id")
    header_id = next(
        (value for key, value in invalid.headers.items() if key.lower() == "x-request-id"),
        "",
    )
    checks.append(
        _check(
            "request_id_consistency",
            invalid.status >= 400 and bool(header_id) and header_id == error_id,
            f"status={invalid.status}, header={header_id!r}, body={error_id!r}",
        )
    )

    register_body = {"email": owner_email, "password": password}
    idem_headers = {"Idempotency-Key": f"register-{suffix}"}
    first = _request(base_url, "POST", "/v1/register", register_body, headers=idem_headers)
    same = _request(base_url, "POST", "/v1/register", register_body, headers=idem_headers)
    changed = _request(
        base_url,
        "POST",
        "/v1/register",
        {"email": f"changed-{suffix}@example.test", "password": password},
        headers=idem_headers,
    )
    checks.append(
        _check(
            "idempotency_same_request_replay",
            first.status == 201 and same.status == first.status and same.body == first.body,
            f"first={first.status}, same={same.status}, bodies_equal={same.body == first.body}",
        )
    )
    checks.append(
        _check(
            "idempotency_changed_body_conflict",
            changed.status == 409 and changed.body != first.body,
            f"first={first.status}, changed={changed.status}, replayed={changed.body == first.body}",
        )
    )

    member = _request(
        base_url,
        "POST",
        "/v1/register",
        {"email": member_email, "password": password},
    )
    owner_login = _request(base_url, "POST", "/v1/login", register_body)
    member_login = _request(
        base_url,
        "POST",
        "/v1/login",
        {"email": member_email, "password": password},
    )
    owner_token = _field(_data(owner_login), "access_token")
    member_token = _field(_data(member_login), "access_token")
    member_id = _field(_data(member), "id")
    prerequisites = (
        member.status == 201
        and owner_login.status == 200
        and member_login.status == 200
        and isinstance(owner_token, str)
        and isinstance(member_token, str)
        and isinstance(member_id, str)
    )

    organization = _request(
        base_url,
        "POST",
        "/v1/organizations",
        {"name": f"Quality {suffix}"},
        token=owner_token if isinstance(owner_token, str) else None,
    )
    organization_id = _field(_data(organization), "id")
    role_status = 0
    assignment_status = 0
    if prerequisites and organization.status == 201 and isinstance(organization_id, str):
        invite = _request(
            base_url,
            "POST",
            f"/v1/organizations/{organization_id}/invites",
            {"email": member_email, "role": "viewer"},
            token=owner_token,
        )
        invite_data = _data(invite)
        invite_token = _field(invite_data, "invite_token", "token")
        if isinstance(invite_token, str):
            _request(
                base_url,
                "POST",
                f"/v1/invites/{invite_token}/accept",
                {},
                token=member_token,
            )
        role = _request(
            base_url,
            "POST",
            f"/v1/organizations/{organization_id}/roles",
            {"name": f"auditor-{suffix}", "permissions": ["org.read"]},
            token=owner_token,
        )
        role_status = role.status
        role_id = _field(_data(role), "id")
        if isinstance(role_id, str) and isinstance(member_id, str):
            assignment = _request(
                base_url,
                "PUT",
                f"/v1/organizations/{organization_id}/members/{member_id}/roles/{role_id}",
                {},
                token=owner_token,
            )
            assignment_status = assignment.status
    checks.append(
        _check(
            "custom_role_creation",
            role_status == 201,
            f"prerequisites={prerequisites}, organization={organization.status}, role={role_status}",
        )
    )
    checks.append(
        _check(
            "custom_role_assignment",
            assignment_status in {200, 204},
            f"role={role_status}, assignment={assignment_status}",
        )
    )
    return checks


def select_checks(
    checks: list[dict[str, Any]],
    selected: set[str] | None,
) -> list[dict[str, Any]]:
    if selected is None:
        return checks
    return [check for check in checks if check.get("name") in selected]


def _wait_ready(base_url: str, process: subprocess.Popen[bytes], timeout: float) -> None:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f"service exited before readiness with code {process.returncode}")
        try:
            # Readiness here means that the generated process is accepting HTTP
            # requests.  A health endpoint is introduced by a later benchmark
            # chunk, so requiring /healthz=200 would reject a live, valid
            # intermediate implementation before that chunk is reached.
            _request(base_url, "GET", "/healthz", timeout=0.5)
            return
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(0.1)
    raise RuntimeError("service readiness timeout")


def verify(
    project: Path,
    timeout: float,
    selected_checks: set[str] | None = None,
) -> dict[str, Any]:
    project = project.resolve()
    started = time.monotonic()
    go_files = sorted(project.rglob("*.go"))
    report: dict[str, Any] = {
        "project": str(project),
        "checks": [],
        "code_metrics": {
            "go_files": len(go_files),
            "go_lines": sum(len(path.read_text(encoding="utf-8").splitlines()) for path in go_files),
            "max_file_lines": max(
                (len(path.read_text(encoding="utf-8").splitlines()) for path in go_files),
                default=0,
            ),
        },
    }
    with tempfile.TemporaryDirectory(prefix="iam-quality-") as temporary:
        binary = Path(temporary) / "iamd"
        environment = os.environ.copy()
        environment["GOCACHE"] = str(Path(temporary) / "gocache")
        build = subprocess.run(
            ["go", "build", "-o", str(binary), "./cmd/iamd"],
            cwd=project,
            env=environment,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
        report["build"] = {
            "exit_code": build.returncode,
            "stderr": build.stderr[-2000:],
        }
        if build.returncode != 0:
            report["passed"] = False
            report["duration_seconds"] = round(time.monotonic() - started, 3)
            return report
        port = _free_port()
        base_url = f"http://127.0.0.1:{port}"
        environment.update(
            {
                "IAM_ADDRESS": f"127.0.0.1:{port}",
                "IAM_ADDR": f"127.0.0.1:{port}",
                "IAM_HMAC_SECRET": "quality-verifier-secret-at-least-32-bytes",
            }
        )
        process = subprocess.Popen(
            [str(binary)],
            cwd=project,
            env=environment,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        )
        try:
            _wait_ready(base_url, process, min(timeout, 15))
            report["checks"] = select_checks(exercise(base_url), selected_checks)
        except Exception as error:  # report infrastructure/service failures as data
            report["runtime_error"] = str(error)
        finally:
            process.terminate()
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=3)
    report["passed"] = bool(report["checks"]) and all(
        check["passed"] for check in report["checks"]
    )
    report["duration_seconds"] = round(time.monotonic() - started, 3)
    return report


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--project", type=Path, required=True)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--timeout", type=float, default=60)
    parser.add_argument("--check", action="append", default=[])
    args = parser.parse_args()
    report = verify(args.project, args.timeout, set(args.check) or None)
    payload = json.dumps(report, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(payload, encoding="utf-8")
    print(payload, end="")
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
