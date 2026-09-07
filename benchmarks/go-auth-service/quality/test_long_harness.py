from __future__ import annotations

import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


QUALITY = Path(__file__).parent


def load_module(name: str, filename: str):
    spec = importlib.util.spec_from_file_location(name, QUALITY / filename)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


verifier = load_module("verify_static", "verify_static.py")
runner = load_module("quality_run_benchmark", "run_benchmark.py")


class LongHarnessTests(unittest.TestCase):
    def test_task_preflight_rejects_empty_paths_without_discovery_policy(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "tasks.json"
            tasks = [
                {
                    "id": f"x-{index}", "title": "Task", "done_when": ["done"],
                    "reads": [], "writes": [],
                }
                for index in range(32)
            ]
            path.write_text(
                json.dumps({"schema_version": 2, "tasks": tasks}),
                encoding="utf-8",
            )
            with self.assertRaisesRegex(RuntimeError, "discovery_required"):
                runner.load_tasks(path)

    def test_context_relevance_records_precision_and_coverage(self) -> None:
        report = runner.context_relevance(
            "3.2",
            [
                {"path": "internal/auth/token.go", "areas": ["access-auth"]},
                {"path": "internal/org/repo.go", "areas": ["organizations"]},
            ],
        )
        self.assertEqual(0.5, report["precision"])
        self.assertEqual(1.0, report["coverage"])
        for task_id in ("4.3", "6.4", "7.1", "8.1"):
            self.assertIn(task_id, runner.CONTEXT_RELEVANCE_AREAS)

    def test_controller_handoff_drives_session_reuse_and_reset(self) -> None:
        self.assertEqual(("thread-1", "continue"), runner.apply_handoff("continue", "thread-1"))
        self.assertEqual((None, "reset"), runner.apply_handoff("reset", "thread-1"))
        self.assertEqual((None, "checkpoint"), runner.apply_handoff("checkpoint", "thread-1"))
        self.assertTrue(runner.handoff_matches("continue", "thread-1", "thread-1"))
        self.assertFalse(runner.handoff_matches("continue", "thread-1", "thread-2"))
        self.assertTrue(runner.handoff_matches("reset", "thread-1", "thread-2"))
        self.assertFalse(runner.handoff_matches("reset", "thread-1", "thread-1"))

    def test_token_metrics_separate_cached_and_uncached_input(self) -> None:
        line = json.dumps(
            {
                "type": "turn.completed",
                "usage": {
                    "input_tokens": 100,
                    "cached_input_tokens": 70,
                    "output_tokens": 20,
                    "reasoning_output_tokens": 5,
                },
            }
        )
        usage, _, _, _ = runner.parse_jsonl(line)
        self.assertEqual(30, usage["uncached_input_tokens"])
        self.assertEqual(120, usage["total_tokens"])

    def test_architecture_review_rejects_malformed_json_and_includes_future_plan(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            project = root / "project"
            state_file = project / ".execution-state" / "state" / "state.json"
            state_file.parent.mkdir(parents=True)
            state_file.write_text(
                json.dumps({"revision": 4, "quality": {"review_reasons": ["periodic"]}}),
                encoding="utf-8",
            )
            prompt_template = root / "review.md"
            prompt_template.write_text(
                "{{STATE_PATH}}\n{{REVIEW_REASONS}}\n{{FUTURE_TASKS}}",
                encoding="utf-8",
            )
            schema = root / "schema.json"
            schema.write_text("{}", encoding="utf-8")
            metrics = {"runs": {"candidate": {"execution_status": "running", "quality_outcome": "not_evaluated"}}}
            journal = runner.RunJournal(metrics, root / "metrics.json", "candidate", root / "journal.jsonl")
            captured: dict[str, str] = {}

            def fake_run(command, prompt, *_args, **_kwargs):
                captured["prompt"] = prompt
                captured["command"] = " ".join(command)
                output = Path(command[command.index("-o") + 1])
                output.write_text("not-json", encoding="utf-8")
                return {"exit_code": 0, "execution_classification": "complete"}

            with mock.patch.object(runner, "run_codex", side_effect=fake_run), mock.patch.object(
                runner,
                "run_statectl",
                side_effect=AssertionError("malformed result must not reach controller"),
            ):
                result = runner.run_architecture_review(
                    project=project,
                    state_id="state",
                    statectl=root / "statectl.py",
                    prompt_template=prompt_template,
                    output_schema=schema,
                    future_tasks=[
                        {"id": "8.3", "title": "Metrics", "done_when": ["Expose /metrics"]}
                    ],
                    raw=root,
                    review_number=1,
                    timeout=10,
                    journal=journal,
                )
            self.assertEqual("failed", result["status"])
            self.assertIn("malformed", result["error"])
            self.assertIn("Expose /metrics", captured["prompt"])
            self.assertIn(str(schema), captured["command"])

    def test_invite_model_ast_accepts_struct_variants_and_alias(self) -> None:
        sources = {
            "Invite": "package sample\ntype Invite struct{}\n",
            "InviteToken": "package sample\ntype InviteToken struct{}\n",
            "OrganizationInvite": "package sample\ntype OrganizationInvite struct{}\n",
            "alias": "package sample\ntype OrganizationInvite struct{}\ntype Invite = OrganizationInvite\n",
        }
        for name, source in sources.items():
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                project = Path(temporary)
                (project / "model.go").write_text(source, encoding="utf-8")
                facts, error = verifier.collect_ast_facts(project, 20)
                self.assertIsNone(error)
                markers = verifier.evaluate_markers(source, ["6.3"], facts)
                self.assertTrue(markers["6.3"]["invite model"], markers)

        negative = {"types": [{"name": "InvitationText", "alias": False}]}
        markers = verifier.evaluate_markers("type InvitationText string", ["6.3"], negative)
        self.assertFalse(markers["6.3"]["invite model"])

    def test_ast_role_wiring_accepts_three_independent_shapes(self) -> None:
        fixtures = (
            {
                "types": [],
                "functions": ["assignRole", "revokeRole"],
                "methods": [],
                "strings": ["/v1/organizations/{id}/roles/{role}"],
                "selectors": [],
            },
            {
                "types": [],
                "functions": [],
                "methods": ["assignRole", "removeRole"],
                "strings": ["/roles/"],
                "selectors": [],
            },
            {
                "types": [],
                "functions": [],
                "methods": [],
                "strings": ["/roles/"],
                "selectors": ["http.MethodPut", "http.MethodDelete"],
            },
        )
        for facts in fixtures:
            markers = verifier.evaluate_markers("", ["7.2"], facts)["7.2"]
            self.assertTrue(all(markers.values()), markers)

    def test_repair_prompt_exposes_contracts_not_internal_marker_names(self) -> None:
        text = runner.repair_suffix(
            {
                "markers": {"6.3": {"invite model": False}},
                "go_test": {"command": ["go", "test"], "exit_code": 0},
                "go_vet": {"command": ["go", "vet"], "exit_code": 0},
                "gofmt": {"command": ["gofmt"], "exit_code": 0, "stdout": ""},
            },
            "",
            {"done_when": ["invites can be persisted"]},
        )
        self.assertIn("invites can be persisted", text)
        self.assertNotIn("Missing markers", text)
        self.assertNotIn("invite model", text)

    def test_model_subprocess_streams_heartbeat_and_classifies_outcomes(self) -> None:
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
            metrics_path = root / "metrics.json"
            journal = runner.RunJournal(
                metrics, metrics_path, "candidate", root / "orchestration.jsonl"
            )
            command = [
                sys.executable,
                "-c",
                "import sys,time; sys.stdin.read(); time.sleep(.12); print('{\"type\":\"turn.completed\",\"usage\":{}}')",
            ]
            result = runner.run_codex(
                command,
                "work",
                root,
                2,
                root / "attempt.jsonl",
                root / "attempt.stderr.log",
                journal,
                heartbeat_seconds=.02,
            )
            self.assertEqual("complete", result["execution_classification"])
            saved = json.loads(metrics_path.read_text(encoding="utf-8"))["runs"]["candidate"]
            self.assertGreater(saved["last_attempt"]["heartbeat_at"], saved["last_attempt"]["started_at"])
            self.assertNotIn("active_attempt", saved)

            timeout_result = runner.run_codex(
                [sys.executable, "-c", "import sys,time; sys.stdin.read(); time.sleep(2)"],
                "work",
                root,
                .08,
                root / "timeout.jsonl",
                root / "timeout.stderr.log",
                journal,
                heartbeat_seconds=.02,
            )
            self.assertEqual("interrupted", timeout_result["execution_classification"])

            failed_result = runner.run_codex(
                [sys.executable, "-c", "import sys; sys.stdin.read(); raise SystemExit(7)"],
                "work",
                root,
                2,
                root / "failed.jsonl",
                root / "failed.stderr.log",
                journal,
                heartbeat_seconds=.02,
            )
            self.assertEqual("infra_error", failed_result["execution_classification"])

            spawn_result = runner.run_codex(
                [str(root / "missing-worker")],
                "work",
                root,
                2,
                root / "spawn.jsonl",
                root / "spawn.stderr.log",
                journal,
                heartbeat_seconds=.02,
            )
            self.assertEqual("infra_error", spawn_result["execution_classification"])
            events = [
                json.loads(line)["event"]
                for line in (root / "orchestration.jsonl").read_text(encoding="utf-8").splitlines()
            ]
            self.assertIn("subprocess_spawn_failed", events)

    def test_runner_exposes_only_standalone_variants(self) -> None:
        self.assertEqual(
            {"01-skill-standalone", "02-ai-only"},
            set(runner.VARIANTS),
        )
        self.assertTrue(
            all(variant["source"] == "standalone" for variant in runner.VARIANTS.values())
        )

    def test_worker_result_protocol_tracks_request_protocol(self) -> None:
        for request_version, result_version in (("v2", "v2"), ("v3", "v3")):
            packet = {
                "protocol": f"execution-state.worker/{request_version}",
                "run_id": "run-1",
                "based_on_revision": 4,
                "task": {"id": "1.1"},
            }
            result = {
                "protocol": f"execution-state.result/{result_version}",
                "run_id": "run-1",
                "based_on_revision": 4,
                "task_id": "1.1",
                "status": "complete",
            }
            matches, message = runner.worker_result_matches(result, packet)
            self.assertTrue(matches, message)

        matches, message = runner.worker_result_matches(
            {
                "protocol": "execution-state.result/v1",
                "run_id": "run-1",
                "based_on_revision": 4,
                "task_id": "1.1",
                "status": "complete",
            },
            {
                "protocol": "execution-state.worker/v1",
                "run_id": "run-1",
                "based_on_revision": 4,
                "task": {"id": "1.1"},
            },
        )
        self.assertFalse(matches)
        self.assertIn("unsupported", message)

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
