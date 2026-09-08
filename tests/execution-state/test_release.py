from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
CHECKER_PATH = ROOT / "benchmarks/go-auth-service/quality/check_skill_release.py"
SPEC = importlib.util.spec_from_file_location("check_skill_release", CHECKER_PATH)
assert SPEC and SPEC.loader
checker = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(checker)


class ReleaseTests(unittest.TestCase):
    def test_public_release_contracts_are_version_1_0_0(self) -> None:
        release = json.loads(
            (ROOT / "skills/execution-state/release.json").read_text(encoding="utf-8")
        )
        self.assertEqual("1.0.0", release["version"])
        for field in (
            "state_schema",
            "task_ledger_schema",
            "context_map_schema",
            "packet_version",
            "adapter_manifest_schema",
        ):
            self.assertEqual("1.0.0", release[field])
        self.assertEqual(
            "execution-state.worker/1.0.0", release["worker_protocol"]
        )
        self.assertEqual(
            "execution-state.result/1.0.0", release["result_protocol"]
        )
        self.assertEqual(
            "execution-state.review/1.0.0", release["review_protocol"]
        )
        self.assertEqual(
            "execution-state.stdin/1.0.0", release["stdin_protocol"]
        )

    def test_inventory_and_archive_are_deterministic(self) -> None:
        skill = ROOT / "skills/execution-state"
        checker.validate_inventory(skill)
        with tempfile.TemporaryDirectory() as temporary:
            first = Path(temporary) / "first.tar"
            second = Path(temporary) / "second.tar"
            self.assertEqual(
                checker.build_archive(skill, first),
                checker.build_archive(skill, second),
            )

    def test_extra_file_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            skill = Path(temporary)
            for relative in checker.ALLOWED_FILES:
                path = skill / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("fixture", encoding="utf-8")
            (skill / "legacy.py").write_text("legacy", encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "legacy.py"):
                checker.validate_inventory(skill)


if __name__ == "__main__":
    unittest.main()
