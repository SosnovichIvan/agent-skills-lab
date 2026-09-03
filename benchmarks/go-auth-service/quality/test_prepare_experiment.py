from __future__ import annotations

import importlib.util
import json
import sys
import subprocess
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("prepare_experiment.py")
SPEC = importlib.util.spec_from_file_location("prepare_experiment", MODULE_PATH)
assert SPEC and SPEC.loader
prepare_experiment = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = prepare_experiment
SPEC.loader.exec_module(prepare_experiment)


class PrepareExperimentTests(unittest.TestCase):
    def test_catalog_has_quality_contracts_and_integration_bridge(self) -> None:
        tasks = [
            {"id": "8.3", "title": "Operations", "done_when": ["request ID works"]},
            {"id": "8.4", "title": "Final wiring", "done_when": ["service works"]},
        ]
        catalog = prepare_experiment.enrich_tasks(tasks)
        self.assertEqual(2, catalog["schema_version"])
        self.assertTrue(catalog["tasks"][0]["requires_bridge"])
        self.assertEqual("integration", catalog["tasks"][1]["kind"])
        self.assertEqual(
            ["task-8.3-external-gate"],
            catalog["tasks"][0]["regression_checks"],
        )

    def test_checked_in_profile_contains_no_sdd_variants(self) -> None:
        profile = json.loads(Path(__file__).with_name("experiment.json").read_text(encoding="utf-8"))
        self.assertEqual("standalone", profile["source"])
        self.assertEqual(3, profile["repeats"])
        self.assertEqual("gpt-5.6-luna", profile["model"])
        self.assertTrue(all("sdd" not in item["id"].lower() for item in profile["variants"]))
        self.assertTrue(all("openspec" not in item["id"].lower() for item in profile["variants"]))

    def test_prepare_requires_committed_candidate_sha(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            with self.assertRaisesRegex(ValueError, "committed Git SHA"):
                prepare_experiment.prepare(Path(temporary), "run-1", "WORKTREE")

    def test_prepare_writes_immutable_manifest_and_catalog(self) -> None:
        repo_root = Path(__file__).resolve().parents[3]
        candidate = subprocess.run(
            ["git", "rev-parse", "HEAD"], cwd=repo_root, text=True,
            capture_output=True, check=True,
        ).stdout.strip()
        with tempfile.TemporaryDirectory() as temporary:
            output = prepare_experiment.prepare(
                Path(temporary), "run-1", candidate
            )
            manifest = json.loads((output / "manifest.json").read_text(encoding="utf-8"))
            catalog = json.loads((output / "tasks.json").read_text(encoding="utf-8"))
            self.assertEqual("prepared", manifest["status"])
            self.assertEqual(candidate, manifest["candidate_ref"])
            self.assertEqual(32, len(catalog["tasks"]))
            with self.assertRaisesRegex(ValueError, "refusing to overwrite"):
                prepare_experiment.prepare(Path(temporary), "run-1", candidate)

    def test_prepare_rejects_control_as_candidate(self) -> None:
        profile = json.loads(Path(__file__).with_name("experiment.json").read_text(encoding="utf-8"))
        with tempfile.TemporaryDirectory() as temporary:
            with self.assertRaisesRegex(ValueError, "differ from control_ref"):
                prepare_experiment.prepare(
                    Path(temporary), "run-1", profile["control_ref"]
                )


if __name__ == "__main__":
    unittest.main()
