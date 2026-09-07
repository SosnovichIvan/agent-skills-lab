from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


QUALITY = Path(__file__).parent


class ResumeSmokeTests(unittest.TestCase):
    def test_interrupted_fake_worker_resumes_without_model_usage(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "smoke"
            completed = subprocess.run(
                [sys.executable, str(QUALITY / "resume_smoke.py"), "--output", str(output)],
                text=True,
                capture_output=True,
                timeout=10,
                check=False,
            )
            self.assertEqual(0, completed.returncode, completed.stderr)
            report = json.loads(completed.stdout)
            self.assertTrue(report["passed"])
            self.assertEqual(0, report["model_requests"])
            self.assertEqual(1, report["resume_index"])
            self.assertEqual(["fake-1", "fake-2"], report["completed_tasks"])
            self.assertEqual(1, report["interruption_count"])
            events = [
                json.loads(line)
                for line in (output / "orchestration.jsonl").read_text(encoding="utf-8").splitlines()
            ]
            self.assertEqual(2, sum(item["event"] == "subprocess_started" for item in events))
            self.assertEqual(2, sum(item["event"] == "subprocess_finished" for item in events))
