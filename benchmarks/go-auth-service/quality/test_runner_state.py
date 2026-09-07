from __future__ import annotations

import json
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest import mock

from runner_state import (
    RunJournal,
    classify_process,
    is_stale,
    mark_stale_interrupted,
    prepare_interrupted_resume,
    validate_run_state,
)


class RunnerStateTests(unittest.TestCase):
    def test_execution_and_quality_dimensions_are_unambiguous(self) -> None:
        validate_run_state({"execution_status": "complete", "quality_outcome": "pass"})
        validate_run_state({"execution_status": "complete", "quality_outcome": "fail"})
        validate_run_state({"execution_status": "infra_error", "quality_outcome": "not_evaluated"})
        with self.assertRaisesRegex(ValueError, "quality_outcome"):
            validate_run_state({"execution_status": "infra_error", "quality_outcome": "fail"})
        with self.assertRaisesRegex(ValueError, "execution_status"):
            validate_run_state({"status": "failed"})

    def test_process_classification_distinguishes_timeout_signal_and_exit(self) -> None:
        self.assertEqual("complete", classify_process(0))
        self.assertEqual("infra_error", classify_process(2))
        self.assertEqual("interrupted", classify_process(-15))
        self.assertEqual("interrupted", classify_process(124, timed_out=True))

    def test_stale_detection_needs_no_sleep(self) -> None:
        now = datetime(2026, 9, 7, 12, tzinfo=timezone.utc)
        run = {
            "execution_status": "running",
            "quality_outcome": "not_evaluated",
            "active_attempt": {"heartbeat_at": (now - timedelta(seconds=61)).isoformat()},
        }
        self.assertTrue(is_stale(run, now=now, ttl_seconds=60))
        self.assertTrue(mark_stale_interrupted(run, now=now, ttl_seconds=60))
        self.assertEqual("interrupted", run["execution_status"])
        self.assertEqual("not_evaluated", run["quality_outcome"])

    def test_journal_persists_active_attempt_heartbeat_and_completion_log(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            metrics = {
                "runs": {
                    "candidate": {
                        "execution_status": "preparing",
                        "quality_outcome": "not_evaluated",
                    }
                }
            }
            path = root / "metrics.json"
            log = root / "orchestration.jsonl"
            journal = RunJournal(metrics, path, "candidate", log)
            journal.started(
                attempt_id="task-01-attempt-1",
                pid=42,
                command=["fake-worker"],
                stdout_path=root / "worker.jsonl",
                stderr_path=root / "worker.stderr.log",
            )
            first = json.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(42, first["runs"]["candidate"]["active_attempt"]["pid"])
            before = first["runs"]["candidate"]["active_attempt"]["heartbeat_at"]
            with mock.patch("runner_state.utc_now", return_value="2026-09-07T12:00:05+00:00"):
                journal.heartbeat()
            after = json.loads(path.read_text(encoding="utf-8"))
            self.assertNotEqual(before, after["runs"]["candidate"]["active_attempt"]["heartbeat_at"])
            journal.finished(classification="interrupted", returncode=-15, timed_out=False)
            final = json.loads(path.read_text(encoding="utf-8"))
            self.assertNotIn("active_attempt", final["runs"]["candidate"])
            events = [json.loads(line) for line in log.read_text(encoding="utf-8").splitlines()]
            self.assertEqual(["subprocess_started", "subprocess_finished"], [item["event"] for item in events])

    def test_resume_preserves_completed_prefix_and_discards_only_interrupted_chunk(self) -> None:
        accepted = {"task_id": "1.1", "status": "complete", "attempts": []}
        interrupted = {
            "task_id": "1.2",
            "status": "failed",
            "attempts": [
                {
                    "execution_classification": "interrupted",
                    "worker_result": None,
                }
            ],
        }
        run = {
            "execution_status": "interrupted",
            "quality_outcome": "not_evaluated",
            "chunks": [accepted, interrupted],
        }
        self.assertEqual(1, prepare_interrupted_resume(run))
        self.assertEqual([accepted], run["chunks"])
        self.assertEqual("1.2", run["infrastructure_interruptions"][0]["task_id"])
        self.assertEqual("running", run["execution_status"])


if __name__ == "__main__":
    unittest.main()
