from __future__ import annotations

import importlib.util
import sys
import unittest
from pathlib import Path


LONG_SESSION = Path(__file__).parents[1] / "long-session"


def load_module(name: str, filename: str):
    spec = importlib.util.spec_from_file_location(name, LONG_SESSION / filename)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


verifier = load_module("verify_long_project", "verify_long_project.py")
runner = load_module("quality_run_long_benchmark", "run_long_benchmark.py")


class LongHarnessTests(unittest.TestCase):
    def test_utf8_truncation_uses_bytes_and_preserves_valid_text(self) -> None:
        value = "Итог: " + "проверка " * 300
        truncated = runner.truncate_utf8(value, 1000)
        self.assertLessEqual(len(truncated.encode("utf-8")), 1000)
        self.assertTrue(truncated.startswith("Итог:"))

    def test_role_gate_accepts_inline_method_dispatch(self) -> None:
        source = '''
        if strings.Contains(r.URL.Path, "/roles/") {
            if r.Method == http.MethodPut { assign() }
            if r.Method == http.MethodDelete { revoke() }
        }
        '''
        markers = verifier.evaluate_markers(source, ["7.2"])["7.2"]
        self.assertTrue(all(markers.values()), markers)

    def test_role_gate_accepts_go_122_mux_routes(self) -> None:
        source = '''
        mux.Handle("PUT /v1/organizations/{orgID}/members/{userID}/roles/{roleID}", handler)
        mux.Handle("DELETE /v1/organizations/{orgID}/members/{userID}/roles/{roleID}", handler)
        '''
        markers = verifier.evaluate_markers(source, ["7.2"])["7.2"]
        self.assertTrue(all(markers.values()), markers)

    def test_role_gate_rejects_route_without_delete(self) -> None:
        source = '''
        if strings.Contains(r.URL.Path, "/roles/") && r.Method == http.MethodPut { assign() }
        '''
        markers = verifier.evaluate_markers(source, ["7.2"])["7.2"]
        self.assertTrue(markers["role assignment handler"])
        self.assertFalse(markers["role revocation handler"])


if __name__ == "__main__":
    unittest.main()
