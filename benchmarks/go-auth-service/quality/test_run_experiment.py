from __future__ import annotations

import importlib.util
import json
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).with_name("run_experiment.py")
SPEC = importlib.util.spec_from_file_location("quality_run_experiment", MODULE_PATH)
assert SPEC and SPEC.loader
run_experiment = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = run_experiment
SPEC.loader.exec_module(run_experiment)


class RunExperimentTests(unittest.TestCase):
    def test_validity_report_excludes_incomplete_run_from_comparison(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            experiment = Path(temporary)
            manifest = {
                "protocol_version": 5,
                "run_orders": [["control-skill-previous", "candidate-skill-current"]],
            }
            complete = experiment / "runs" / "repeat-01-control" / "metrics.json"
            complete.parent.mkdir(parents=True)
            complete.write_text(
                json.dumps({"runs": {"01-skill-standalone": {
                    "execution_status": "complete", "quality_outcome": "pass"
                }}}),
                encoding="utf-8",
            )
            path = run_experiment.write_validity_report(experiment, manifest)
            report = json.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(["repeat-01-control"], report["comparison_runs"])
            self.assertIn("repeat-01-candidate", report["infrastructure_interruptions"])

    def test_product_failure_does_not_stop_later_variants(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            experiment = Path(temporary)
            (experiment / "inputs/control/skills/execution-state/scripts").mkdir(parents=True)
            (experiment / "inputs/candidate/skills/execution-state/scripts").mkdir(parents=True)
            for label in ("control", "candidate"):
                skill = experiment / f"inputs/{label}/skills/execution-state"
                (skill / "SKILL.md").write_text("skill", encoding="utf-8")
                (skill / "scripts/statectl.py").write_text("controller", encoding="utf-8")
            manifest = {
                "protocol_version": 5,
                "source": "standalone",
                "model": "gpt-5.6-luna",
                "reasoning_effort": "medium",
                "run_orders": [["control-skill-previous", "candidate-skill-current", "ai-only"]],
            }
            (experiment / "manifest.json").write_text(json.dumps(manifest), encoding="utf-8")
            (experiment / "tasks.json").write_text("{}", encoding="utf-8")
            calls: list[str] = []

            def fake_run(command, **kwargs):
                run_id = command[command.index("--run-id") + 1]
                variant = command[command.index("--variant") + 1]
                calls.append(run_id)
                metrics = experiment / "runs" / run_id / "metrics.json"
                metrics.parent.mkdir(parents=True, exist_ok=True)
                metrics.write_text(
                    json.dumps(
                        {
                            "runs": {
                                variant: {
                                    "execution_status": "complete",
                                    "quality_outcome": "fail" if len(calls) == 1 else "pass",
                                }
                            }
                        }
                    ),
                    encoding="utf-8",
                )
                return type("Completed", (), {"returncode": 0})()

            argv = ["run_experiment.py", "--experiment", str(experiment)]
            with mock.patch.object(sys, "argv", argv), mock.patch.object(
                run_experiment.subprocess, "run", side_effect=fake_run
            ):
                self.assertEqual(0, run_experiment.main())
            self.assertEqual(
                ["repeat-01-control", "repeat-01-candidate", "repeat-01-ai"],
                calls,
            )

    def test_snapshot_marks_stale_run_interrupted(self) -> None:
        now = datetime(2026, 9, 7, 12, tzinfo=timezone.utc)
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "metrics.json"
            path.write_text(
                json.dumps(
                    {
                        "runs": {
                            "candidate": {
                                "execution_status": "running",
                                "quality_outcome": "not_evaluated",
                                "active_attempt": {
                                    "heartbeat_at": (now - timedelta(seconds=90)).isoformat()
                                },
                            }
                        }
                    }
                ),
                encoding="utf-8",
            )
            snapshot = run_experiment.run_snapshot(
                path, "candidate", stale_ttl=60, now=now
            )
            self.assertEqual(("interrupted", "not_evaluated"), snapshot)
            saved = json.loads(path.read_text(encoding="utf-8"))["runs"]["candidate"]
            self.assertEqual("stale_heartbeat", saved["interruption"]["kind"])

    def test_snapshot_keeps_product_failure_separate(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "metrics.json"
            path.write_text(
                json.dumps(
                    {
                        "runs": {
                            "candidate": {
                                "execution_status": "complete",
                                "quality_outcome": "fail",
                            }
                        }
                    }
                ),
                encoding="utf-8",
            )
            self.assertEqual(
                ("complete", "fail"),
                run_experiment.run_snapshot(path, "candidate", stale_ttl=60),
            )


if __name__ == "__main__":
    unittest.main()
