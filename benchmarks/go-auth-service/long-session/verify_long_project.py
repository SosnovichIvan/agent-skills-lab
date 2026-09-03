#!/usr/bin/env python3
"""External compile/static gate for one long-session IAM benchmark chunk."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import tempfile
import time
from pathlib import Path
from typing import Any


TASK_MARKERS: dict[str, list[tuple[str, tuple[str, ...]]]] = {
    "1.1": [
        ("module path", (r"module\s+benchmark\.local/iam",)),
        ("config", (r"type\s+Config\s+struct",)),
        ("server entrypoint", (r"cmd/iamd",)),
    ],
    "1.2": [
        ("clock abstraction", (r"type\s+Clock\s+interface", r"Clock\s+interface")),
        ("crypto random", (r"crypto/rand",)),
        ("typed domain errors", (r"ErrNotFound", r"notFoundError")),
    ],
    "1.3": [
        ("user model", (r"type\s+User\s+struct",)),
        ("public profile", (r"PublicUser", r"PublicProfile", r"UserProfile")),
        ("hidden password json", (r"json:\"-\"",)),
    ],
    "1.4": [
        ("repository lock", (r"sync\.RWMutex", r"sync\.Mutex")),
        ("email index", (r"byEmail", r"emailIndex", r"usersByEmail")),
    ],
    "2.1": [
        ("email normalization", (r"strings\.ToLower",)),
        ("email trimming", (r"strings\.TrimSpace",)),
        ("password minimum", (r"12",)),
    ],
    "2.2": [
        ("HMAC SHA-256 KDF", (r"hmac\.New\(sha256\.New",)),
        ("constant-time compare", (r"subtle\.ConstantTimeCompare", r"hmac\.Equal")),
    ],
    "2.3": [("register route", (r"/v1/register", r"/register"))],
    "2.4": [
        ("strict decoder", (r"DisallowUnknownFields",)),
        ("body limit", (r"MaxBytesReader", r"LimitReader")),
        ("error envelope", (r"request_id", r"RequestID")),
    ],
    "3.1": [
        ("HS256", (r"HS256",)),
        ("JWT base64url", (r"RawURLEncoding",)),
        ("JWT id", (r"jti", r"JTI")),
    ],
    "3.2": [
        ("issuer validation", (r"issuer", r"Issuer")),
        ("expiry validation", (r"exp", r"ExpiresAt")),
        ("signature validation", (r"hmac\.Equal", r"ConstantTimeCompare")),
    ],
    "3.3": [("login route", (r"/v1/login", r"/login"))],
    "3.4": [
        ("me route", (r"/v1/me", r"/me")),
        ("authorization header", (r"Authorization",)),
        ("Bearer", (r"Bearer",)),
    ],
    "4.1": [
        ("session model", (r"type\s+Session\s+struct",)),
        ("opaque token hash", (r"sha256\.Sum256",)),
        ("token family", (r"Family", r"family")),
    ],
    "4.2": [("refresh route", (r"/v1/token/refresh", r"/token/refresh"))],
    "4.3": [
        ("reuse detection", (r"Reuse", r"reuse", r"reused")),
        ("family revocation", (r"RevokeFamily", r"revokeFamily", r"family.*revok")),
    ],
    "4.4": [
        ("logout", (r"/v1/logout", r"/logout")),
        ("logout all", (r"logout-all", r"logout/all")),
        ("sessions", (r"/v1/sessions", r"/sessions")),
    ],
    "5.1": [("password change", (r"password/change", r"change-password"))],
    "5.2": [("password reset", (r"password-reset", r"password/reset"))],
    "5.3": [("email verification", (r"email-verification", r"email/verification"))],
    "5.4": [
        ("failed login counter", (r"Failed.*Login", r"failed.*attempt", r"LoginAttempts")),
        ("lock duration", (r"Lockout", r"LockedUntil", r"lockDuration")),
    ],
    "6.1": [
        ("organization model", (r"type\s+Organization\s+struct",)),
        ("membership model", (r"type\s+Membership\s+struct",)),
    ],
    "6.2": [("organizations route", (r"/v1/organizations", r"/organizations"))],
    "6.3": [
        ("invites route", (r"/invites",)),
        ("invite model", (r"type\s+Invite(?:Token)?\s+struct",)),
    ],
    "6.4": [("ownership transfer", (r"ownership-transfer", r"transfer.*owner"))],
    "7.1": [
        ("owner role", (r"owner",)),
        ("permission vocabulary", (r"org\.read", r"member\.manage")),
        ("custom role", (r"CustomRole", r"type\s+Role\s+struct")),
    ],
    "7.2": [
        ("role route discriminator", (r"parts\[\d+\]\s*==\s*\"roles\"", r"/roles/")),
        ("role assignment handler", (r"func\s+\([^)]*\)\s+assignRole\s*\(",)),
        ("role revocation handler", (r"func\s+\([^)]*\)\s+(?:revoke|remove)Role\s*\(",)),
    ],
    "7.3": [
        ("permission check", (r"Permission", r"permission")),
        ("tenant organization id", (r"OrgID", r"OrganizationID")),
    ],
    "7.4": [("api key route", (r"api-keys", r"api_keys"))],
    "8.1": [
        ("rate limiter", (r"RateLimit", r"rateLimiter", r"rate limit")),
        ("idempotency", (r"Idempotency", r"idempotency")),
    ],
    "8.2": [
        ("audit model", (r"type\s+Audit", r"AuditEvent")),
        ("cursor pagination", (r"cursor", r"Cursor")),
        ("audit hash", (r"PreviousHash", r"PrevHash", r"HashChain")),
    ],
    "8.3": [
        ("request id", (r"X-Request-ID",)),
        ("health", (r"/healthz",)),
        ("readiness", (r"/readyz",)),
        ("metrics", (r"/metrics",)),
    ],
    "8.4": [
        ("graceful shutdown", (r"\.Shutdown\(",)),
        ("read timeout", (r"ReadTimeout",)),
        ("write timeout", (r"WriteTimeout",)),
        ("idle timeout", (r"IdleTimeout",)),
    ],
}


def run(command: list[str], project: Path, environment: dict[str, str], timeout: int) -> dict[str, Any]:
    try:
        completed = subprocess.run(
            command,
            cwd=project,
            env=environment,
            text=True,
            capture_output=True,
            timeout=timeout,
            check=False,
        )
        return {
            "command": command,
            "exit_code": completed.returncode,
            "stdout": completed.stdout[-2000:],
            "stderr": completed.stderr[-2000:],
        }
    except subprocess.TimeoutExpired as exc:
        return {
            "command": command,
            "exit_code": 124,
            "stdout": (exc.stdout or "")[-2000:] if isinstance(exc.stdout, str) else "",
            "stderr": "verification timeout",
        }


def verify(project: Path, task_id: str, final: bool, timeout: int) -> dict[str, Any]:
    started = time.monotonic()
    go_files = sorted(path for path in project.rglob("*.go") if path.is_file())
    test_files = [path for path in go_files if path.name.endswith("_test.go")]
    combined = "\n".join(
        [
            (project / "go.mod").read_text(encoding="utf-8")
            if (project / "go.mod").is_file()
            else "",
            *(
                f"\nFILE:{path.relative_to(project)}\n"
                + path.read_text(encoding="utf-8")
                for path in go_files
            ),
        ]
    )
    marker_ids = list(TASK_MARKERS) if final else [task_id]
    marker_results: dict[str, dict[str, bool]] = {}
    for current_id in marker_ids:
        groups = TASK_MARKERS.get(current_id, [])
        marker_results[current_id] = {
            name: any(re.search(pattern, combined, re.IGNORECASE | re.DOTALL) for pattern in patterns)
            for name, patterns in groups
        }

    with tempfile.TemporaryDirectory(prefix="iam-long-verify-") as cache:
        environment = os.environ.copy()
        environment["GOCACHE"] = cache
        go_test = run(["go", "test", "./..."], project, environment, timeout)
        go_vet = run(["go", "vet", "./..."], project, environment, timeout)
        gofmt = run(["gofmt", "-l", *[str(path) for path in go_files]], project, environment, timeout) if go_files else {
            "command": ["gofmt", "-l"],
            "exit_code": 1,
            "stdout": "",
            "stderr": "no Go files",
        }

    go_mod = project / "go.mod"
    go_mod_text = go_mod.read_text(encoding="utf-8") if go_mod.is_file() else ""
    external_modules = bool(re.search(r"(?m)^\s*require\s+(?:\(|\S)", go_mod_text))
    go_sum = project / "go.sum"
    go_sum_nonempty = go_sum.is_file() and bool(go_sum.read_text(encoding="utf-8").strip())
    markers_passed = all(all(groups.values()) for groups in marker_results.values())
    stubs = []
    if final:
        for path in go_files:
            text = path.read_text(encoding="utf-8")
            if re.search(r"TODO|FIXME|panic\(\s*\"not implemented", text, re.IGNORECASE):
                stubs.append(str(path.relative_to(project)))

    passed = (
        go_test["exit_code"] == 0
        and go_vet["exit_code"] == 0
        and gofmt["exit_code"] == 0
        and not gofmt["stdout"].strip()
        and bool(go_files)
        and go_mod.is_file()
        and not test_files
        and not external_modules
        and not go_sum_nonempty
        and markers_passed
        and not stubs
    )
    return {
        "passed": passed,
        "task_id": task_id,
        "final": final,
        "duration_seconds": round(time.monotonic() - started, 3),
        "go_test": go_test,
        "go_vet": go_vet,
        "gofmt": gofmt,
        "go_files": len(go_files),
        "test_files": [str(path.relative_to(project)) for path in test_files],
        "external_modules": external_modules or go_sum_nonempty,
        "markers": marker_results,
        "stubs": stubs,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", type=Path, required=True)
    parser.add_argument("--task-id", choices=TASK_MARKERS, required=True)
    parser.add_argument("--final", action="store_true")
    parser.add_argument("--timeout", type=int, default=120)
    args = parser.parse_args()
    result = verify(args.project.resolve(), args.task_id, args.final, args.timeout)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
