from __future__ import annotations

import importlib.util
import json
import sys
import unittest
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).with_name("verify_behavior.py")
SPEC = importlib.util.spec_from_file_location("verify_behavior", MODULE_PATH)
assert SPEC and SPEC.loader
verify_behavior = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = verify_behavior
SPEC.loader.exec_module(verify_behavior)


def response(status: int, data=None, *, error_id: str | None = None):
    value = {"data": data} if error_id is None else {
        "error": {"code": "invalid", "message": "invalid", "request_id": error_id}
    }
    headers = {"X-Request-ID": error_id} if error_id else {}
    return verify_behavior.Response(status, headers, json.dumps(value).encode())


class ExerciseTests(unittest.TestCase):
    def test_contract_flow_passes_with_correct_service(self) -> None:
        created = response(201, {"id": "owner-1"})
        scripted = [
            response(400, error_id="generated-1"),
            created,
            created,
            response(409, {"conflict": True}),
            response(201, {"id": "member-1"}),
            response(200, {"access_token": "owner-token"}),
            response(200, {"access_token": "member-token"}),
            response(201, {"id": "org-1"}),
            response(201, {"invite_token": "invite-1"}),
            response(200, {}),
            response(201, {"id": "role-1"}),
            response(200, {}),
        ]
        with mock.patch.object(verify_behavior, "_request", side_effect=scripted):
            checks = verify_behavior.exercise("http://example.test")
        self.assertTrue(all(check["passed"] for check in checks), checks)

    def test_contract_flow_detects_known_long_run_defects(self) -> None:
        first = response(201, {"id": "owner-1"})
        scripted = [
            verify_behavior.Response(
                400,
                {"X-Request-ID": "generated-1"},
                b'{"error":{"request_id":""}}',
            ),
            first,
            response(409, {"conflict": True}),
            first,
            response(201, {"id": "member-1"}),
            response(200, {"access_token": "owner-token"}),
            response(200, {"access_token": "member-token"}),
            response(201, {"id": "org-1"}),
            response(201, {"invite_token": "invite-1"}),
            response(200, {}),
            response(404, None),
        ]
        with mock.patch.object(verify_behavior, "_request", side_effect=scripted):
            checks = verify_behavior.exercise("http://example.test")
        failed = {check["name"] for check in checks if not check["passed"]}
        self.assertEqual(
            {
                "request_id_consistency",
                "idempotency_same_request_replay",
                "idempotency_changed_body_conflict",
                "custom_role_creation",
                "custom_role_assignment",
            },
            failed,
        )


if __name__ == "__main__":
    unittest.main()
