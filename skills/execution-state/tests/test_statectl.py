from __future__ import annotations

import io
import json
import os
import sys
import tempfile
import threading
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock


SCRIPTS = Path(__file__).resolve().parents[1] / "scripts"
sys.path.insert(0, str(SCRIPTS))

import statectl  # noqa: E402


class StateCtlTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name) / "проект с пробелами"
        self.root.mkdir()

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def run_cli(self, *arguments: str) -> tuple[int, dict]:
        stdout = io.StringIO()
        with redirect_stdout(stdout):
            code = statectl.main(list(arguments))
        output = stdout.getvalue()
        self.assertEqual(1, len(output.splitlines()), output)
        return code, json.loads(output)

    def state_arguments(self, state_id: str = "задача-1") -> tuple[str, ...]:
        return ("--id", state_id, "--project-root", str(self.root))

    def init_standalone(self, *, task_json: list[dict] | None = None) -> Path:
        arguments = [
            "init",
            *self.state_arguments(),
            "--source",
            "standalone",
            "--profile",
            "lite",
            "--adapter",
            "manual",
            "--implementation-ref",
            "/backend-implementation",
            "--goal",
            "Create a verifiable service",
        ]
        if task_json:
            for task in task_json:
                arguments.extend(
                    ["--task-json", json.dumps(task, ensure_ascii=False)]
                )
        else:
            arguments.extend(
                [
                    "--task-title",
                    "Implement auth",
                    "--done-when",
                    "service compiles",
                ]
            )
        code, result = self.run_cli(*arguments)
        self.assertEqual(0, code, result)
        return Path(result["state"])

    def load_state(self, path: Path) -> dict:
        return json.loads(path.read_text(encoding="utf-8"))

    def test_short_route_is_passthrough_and_creates_no_state(self) -> None:
        with mock.patch.object(
            statectl,
            "probe_runtime",
            side_effect=AssertionError("passthrough must not probe a runtime"),
        ):
            code, result = self.run_cli(
                "route",
                "--source",
                "standalone",
                "--expected-turns",
                "3",
                "--adapter",
                "auto",
            )
        self.assertEqual(0, code)
        self.assertEqual("passthrough", result["decision"])
        self.assertFalse(result["creates_state"])
        self.assertFalse((self.root / ".execution-state").exists())

    def test_reset_is_not_selected_without_confirmed_capability(self) -> None:
        code, result = self.run_cli(
            "route",
            "--profile",
            "reset",
            "--expected-turns",
            "20",
            "--adapter",
            "manual",
        )
        self.assertEqual(0, code)
        self.assertEqual("lite", result["decision"])
        code, result = self.run_cli(
            "init",
            *self.state_arguments(),
            "--source",
            "standalone",
            "--profile",
            "reset",
            "--adapter",
            "manual",
            "--implementation-ref",
            "anything",
            "--goal",
            "goal",
            "--task-title",
            "task",
            "--done-when",
            "done",
        )
        self.assertEqual(statectl.ERROR_RUNTIME, code)
        self.assertEqual("unsupported_runtime", result["error"])
        self.assertFalse((self.root / ".execution-state").exists())

    def test_opaque_implementation_ref_and_compact_state(self) -> None:
        path = self.init_standalone()
        state = self.load_state(path)
        self.assertEqual(4, state["schema_version"])
        self.assertEqual("/backend-implementation", state["implementation_ref"])
        self.assertLessEqual(path.stat().st_size, statectl.MAX_STATE_BYTES)
        self.assertEqual(1, len(path.read_text(encoding="utf-8").splitlines()))

    def test_revision_conflict_is_non_mutating(self) -> None:
        path = self.init_standalone()
        before_state = path.read_bytes()
        before_tasks = (path.parent / "tasks.json").read_bytes()
        code, result = self.run_cli(
            "begin",
            *self.state_arguments(),
            "--expected-revision",
            "9",
            "--task-id",
            "задача-1",
        )
        self.assertEqual(statectl.ERROR_CONFLICT, code)
        self.assertEqual("revision_conflict", result["error"])
        self.assertEqual(before_state, path.read_bytes())
        self.assertEqual(before_tasks, (path.parent / "tasks.json").read_bytes())

    def test_completion_without_evidence_is_non_mutating(self) -> None:
        path = self.init_standalone()
        before_state = path.read_bytes()
        before_tasks = (path.parent / "tasks.json").read_bytes()
        code, result = self.run_cli(
            "complete",
            *self.state_arguments(),
            "--expected-revision",
            "0",
            "--summary",
            "implemented",
        )
        self.assertEqual(statectl.ERROR_INVALID, code)
        self.assertIn("evidence", result["message"])
        self.assertEqual(before_state, path.read_bytes())
        self.assertEqual(before_tasks, (path.parent / "tasks.json").read_bytes())

    def test_observation_limit_is_non_mutating(self) -> None:
        path = self.init_standalone()
        before = path.read_bytes()
        code, result = self.run_cli(
            "observe",
            *self.state_arguments(),
            "--expected-revision",
            "0",
            "--observation",
            "Ж" * 1025,
        )
        self.assertEqual(statectl.ERROR_INVALID, code)
        self.assertIn("2048", result["message"])
        self.assertEqual(before, path.read_bytes())

    def test_standalone_packet_excludes_ledger_and_completed_details(self) -> None:
        path = self.init_standalone(
            task_json=[
                {"id": "one", "title": "First", "done_when": ["compiled"]},
                {"id": "two", "title": "Second", "done_when": ["runs"]},
            ]
        )
        code, result = self.run_cli(
            "complete",
            *self.state_arguments(),
            "--expected-revision",
            "0",
            "--summary",
            "first complete",
            "--evidence",
            "go test ./... passed",
        )
        self.assertEqual(0, code, result)
        packet_path = self.root / "handoff packet.json"
        code, result = self.run_cli(
            "packet", *self.state_arguments(), "--output", str(packet_path)
        )
        self.assertEqual(0, code, result)
        packet = json.loads(packet_path.read_text(encoding="utf-8"))
        self.assertNotIn("tasks", packet)
        self.assertNotIn("last_result", packet)
        self.assertNotIn("history", packet)
        self.assertNotIn("active_task", packet)
        self.assertEqual("execution-state.worker/v2", packet["protocol"])
        self.assertEqual(2, packet["based_on_revision"])
        self.assertEqual("two", packet["task"]["id"])
        self.assertEqual("first complete", packet["last_observation"])
        self.assertEqual(
            {"max_turns": 8, "max_result_bytes": 8192}, packet["limits"]
        )
        self.assertLessEqual(packet_path.stat().st_size, statectl.MAX_PACKET_BYTES)
        ledger = json.loads((path.parent / "tasks.json").read_text(encoding="utf-8"))
        self.assertEqual("complete", ledger["tasks"][0]["status"])
        self.assertEqual("in_progress", ledger["tasks"][1]["status"])

    def test_generated_next_action_uses_english_id_not_source_title(self) -> None:
        path = self.init_standalone(task_json=[
            {"id": "one", "title": "Реализовать вход", "done_when": ["Готово"]},
            {"id": "two", "title": "Проверить токен", "done_when": ["Готово"]},
        ])
        state = self.load_state(path)
        self.assertEqual("Реализовать вход", state["active_task"]["title"])
        self.assertEqual("Execute task one", state["next_action"])

        code, result = self.run_cli(
            "complete",
            *self.state_arguments(),
            "--expected-revision",
            "0",
            "--summary",
            "Login implemented",
            "--evidence",
            "Targeted check passed",
        )
        self.assertEqual(0, code, result)
        state = self.load_state(path)
        self.assertEqual("Проверить токен", state["active_task"]["title"])
        self.assertEqual("Execute task two", state["next_action"])

    def test_checkpoint_then_packet_contains_only_active_context(self) -> None:
        self.init_standalone()
        transitions = (
            (
                "observe",
                *self.state_arguments(),
                "--expected-revision",
                "0",
                "--observation",
                "implementation created",
                "--next-action",
                "compile service",
                "--artifact",
                "cmd/server/main.go",
            ),
            (
                "checkpoint",
                *self.state_arguments(),
                "--expected-revision",
                "1",
                "--reason",
                "semantic chunk finished",
            ),
        )
        for transition in transitions:
            code, result = self.run_cli(*transition)
            self.assertEqual(0, code, result)
        packet_path = self.root / "packet.json"
        code, result = self.run_cli(
            "packet", *self.state_arguments(), "--output", str(packet_path)
        )
        self.assertEqual(0, code, result)
        packet = json.loads(packet_path.read_text(encoding="utf-8"))
        self.assertEqual("execution-state.worker/v2", packet["protocol"])
        self.assertTrue(packet["run_id"])
        self.assertEqual(3, packet["based_on_revision"])
        self.assertEqual("задача-1", packet["task"]["id"])
        self.assertEqual("compile service", packet["next_action"])
        self.assertEqual("implementation created", packet["last_observation"])

    def test_runtime_plan_validates_canonical_worker_envelope(self) -> None:
        self.init_standalone()
        packet_path = self.root / "packet.json"
        code, result = self.run_cli(
            "packet",
            *self.state_arguments(),
            "--output",
            str(packet_path),
            "--run-id",
            "run-42",
        )
        self.assertEqual(0, code, result)
        prompt_path = self.root / "worker.md"
        prompt_path.write_text("Use the attached worker packet.", encoding="utf-8")
        code, result = self.run_cli(
            "runtime-plan",
            *self.state_arguments(),
            "--adapter",
            "manual",
            "--prompt",
            str(prompt_path),
            "--packet",
            str(packet_path),
        )
        self.assertEqual(0, code, result)
        self.assertEqual("manual_handoff", result["strategy"])
        packet = json.loads(packet_path.read_text(encoding="utf-8"))
        packet["goal"] = "tampered goal"
        packet_path.write_text(json.dumps(packet), encoding="utf-8")
        code, result = self.run_cli(
            "runtime-plan",
            *self.state_arguments(),
            "--adapter",
            "manual",
            "--prompt",
            str(prompt_path),
            "--packet",
            str(packet_path),
        )
        self.assertEqual(statectl.ERROR_INVALID, code)
        self.assertIn("goal", result["message"])

    def test_packet_cannot_overwrite_state_or_existing_files(self) -> None:
        state_path = self.init_standalone()
        before = state_path.read_bytes()
        code, result = self.run_cli(
            "packet",
            *self.state_arguments(),
            "--output",
            str(state_path),
        )
        self.assertEqual(statectl.ERROR_INVALID, code)
        self.assertIn("overwrite", result["message"])
        self.assertEqual(before, state_path.read_bytes())
        existing = self.root / "important.json"
        existing.write_text("important", encoding="utf-8")
        code, _ = self.run_cli(
            "packet",
            *self.state_arguments(),
            "--output",
            str(existing),
        )
        self.assertEqual(statectl.ERROR_INVALID, code)
        self.assertEqual("important", existing.read_text(encoding="utf-8"))

    def test_worker_lease_prevents_duplicate_packets_and_requires_run_id(self) -> None:
        state_path = self.init_standalone()
        first_packet = self.root / "first-packet.json"
        code, result = self.run_cli(
            "packet",
            *self.state_arguments(),
            "--output",
            str(first_packet),
            "--run-id",
            "worker-1",
            "--expected-revision",
            "0",
        )
        self.assertEqual(0, code, result)
        self.assertEqual(1, result["revision"])
        leased = self.load_state(state_path)
        self.assertEqual("worker-1", leased["worker_lease"]["run_id"])
        second_packet = self.root / "second-packet.json"
        before = state_path.read_bytes()
        code, result = self.run_cli(
            "packet",
            *self.state_arguments(),
            "--output",
            str(second_packet),
        )
        self.assertEqual(statectl.ERROR_CONFLICT, code)
        self.assertEqual("worker_lease_conflict", result["error"])
        self.assertFalse(second_packet.exists())
        self.assertEqual(before, state_path.read_bytes())
        code, result = self.run_cli(
            "observe",
            *self.state_arguments(),
            "--expected-revision",
            "1",
            "--observation",
            "worker returned",
        )
        self.assertEqual(statectl.ERROR_CONFLICT, code)
        code, result = self.run_cli(
            "observe",
            *self.state_arguments(),
            "--expected-revision",
            "1",
            "--run-id",
            "worker-1",
            "--observation",
            "worker returned",
        )
        self.assertEqual(0, code, result)
        self.assertIsNone(self.load_state(state_path)["worker_lease"])

    def test_existing_controller_lock_rejects_transition_without_mutation(self) -> None:
        state_path = self.init_standalone()
        before = state_path.read_bytes()
        lock = state_path.parent / ".statectl.lock"
        lock.mkdir()
        try:
            code, result = self.run_cli(
                "observe",
                *self.state_arguments(),
                "--expected-revision",
                "0",
                "--observation",
                "must not be written",
            )
        finally:
            lock.rmdir()
        self.assertEqual(statectl.ERROR_CONFLICT, code)
        self.assertEqual("state_busy", result["error"])
        self.assertEqual(before, state_path.read_bytes())

    def test_dead_owner_lock_directory_is_reclaimed(self) -> None:
        state_path = self.init_standalone()
        lock = state_path.parent / ".statectl.lock"
        lock.mkdir()
        (lock / "owner.json").write_text(
            json.dumps({"pid": 999999, "token": "dead"}), encoding="utf-8"
        )
        with mock.patch.object(statectl, "_pid_is_alive", return_value=False):
            code, result = self.run_cli("validate", *self.state_arguments())
        self.assertEqual(0, code, result)
        self.assertFalse(lock.exists())

    def test_interrupted_pair_write_is_rolled_back_on_next_command(self) -> None:
        state_path = self.init_standalone()
        ledger_path = state_path.parent / "tasks.json"
        old_state = state_path.read_bytes()
        old_ledger = ledger_path.read_bytes()
        real_atomic_write = statectl._atomic_write_bytes

        def crash_before_state(target: Path, payload: bytes) -> None:
            if target == state_path and payload != old_state:
                raise SystemExit("simulated process crash")
            real_atomic_write(target, payload)

        arguments = statectl.build_parser().parse_args(
            [
                "complete",
                *self.state_arguments(),
                "--expected-revision",
                "0",
                "--summary",
                "implemented",
                "--evidence",
                "verification passed",
            ]
        )
        with mock.patch.object(
            statectl, "_atomic_write_bytes", side_effect=crash_before_state
        ), self.assertRaises(SystemExit):
            arguments.handler(arguments)

        self.assertNotEqual(old_ledger, ledger_path.read_bytes())
        self.assertEqual(old_state, state_path.read_bytes())
        self.assertTrue((state_path.parent / statectl.TRANSACTION_MARKER).exists())

        code, result = self.run_cli("validate", *self.state_arguments())
        self.assertEqual(0, code, result)
        self.assertEqual(old_state, state_path.read_bytes())
        self.assertEqual(old_ledger, ledger_path.read_bytes())
        self.assertFalse((state_path.parent / statectl.TRANSACTION_MARKER).exists())
        self.assertFalse((state_path.parent / statectl.TRANSACTION_BACKUP).exists())

    def test_committed_pair_with_leftover_journal_is_kept(self) -> None:
        state_path = self.init_standalone()
        arguments = statectl.build_parser().parse_args(
            [
                "complete",
                *self.state_arguments(),
                "--expected-revision",
                "0",
                "--summary",
                "implemented",
                "--evidence",
                "verification passed",
            ]
        )
        with mock.patch.object(
            statectl,
            "_remove_transaction_files",
            side_effect=SystemExit("simulated crash after commit"),
        ), self.assertRaises(SystemExit):
            arguments.handler(arguments)
        self.assertEqual(1, self.load_state(state_path)["revision"])
        self.assertTrue((state_path.parent / statectl.TRANSACTION_MARKER).exists())

        code, result = self.run_cli("validate", *self.state_arguments())
        self.assertEqual(0, code, result)
        self.assertEqual(1, result["revision"])
        self.assertFalse((state_path.parent / statectl.TRANSACTION_MARKER).exists())

    def test_atomic_packet_publish_has_one_winner(self) -> None:
        output = self.root / "shared-packet.json"
        barrier = threading.Barrier(2)
        real_link = os.link
        successes: list[bytes] = []
        failures: list[statectl.StateCtlError] = []

        def synchronized_link(source: str, destination: Path) -> None:
            barrier.wait(timeout=5)
            real_link(source, destination)

        def publish(payload: bytes) -> None:
            try:
                statectl._atomic_create_bytes(output, payload)
                successes.append(payload)
            except statectl.StateCtlError as exc:
                failures.append(exc)

        with mock.patch.object(statectl.os, "link", side_effect=synchronized_link):
            threads = [
                threading.Thread(target=publish, args=(payload,))
                for payload in (b"first\n", b"second\n")
            ]
            for thread in threads:
                thread.start()
            for thread in threads:
                thread.join(timeout=10)
        self.assertTrue(all(not thread.is_alive() for thread in threads))
        self.assertEqual(1, len(successes))
        self.assertEqual(1, len(failures))
        self.assertEqual(successes[0], output.read_bytes())

    def test_openspec_lite_uses_tasks_md_as_ledger(self) -> None:
        change_dir = self.root / "openspec" / "changes" / "auth service"
        change_dir.mkdir(parents=True)
        tasks_path = change_dir / "tasks.md"
        tasks_path.write_text(
            "# Tasks\n\n- [ ] 1.1. Реализовать токены\n- [ ] 1.2 Добавить middleware\n",
            encoding="utf-8",
        )
        arguments = (
            "init",
            *self.state_arguments("auth-change"),
            "--source",
            "openspec",
            "--profile",
            "lite",
            "--adapter",
            "manual",
            "--implementation-ref",
            "claude-native-ref",
            "--goal",
            "Implement approved change",
            "--change",
            "auth-change",
            "--tasks-path",
            str(tasks_path.relative_to(self.root)),
        )
        code, result = self.run_cli(*arguments)
        self.assertEqual(0, code, result)
        state_path = Path(result["state"])
        self.assertEqual(["state.json"], sorted(item.name for item in state_path.parent.iterdir()))
        before = tasks_path.read_text(encoding="utf-8")
        code, result = self.run_cli(
            "complete",
            *self.state_arguments("auth-change"),
            "--expected-revision",
            "0",
            "--summary",
            "tokens implemented",
        )
        self.assertEqual(statectl.ERROR_INVALID, code)
        self.assertEqual(before, tasks_path.read_text(encoding="utf-8"))
        code, result = self.run_cli(
            "complete",
            *self.state_arguments("auth-change"),
            "--expected-revision",
            "0",
            "--summary",
            "tokens implemented",
            "--evidence",
            "go test ./... passed",
        )
        self.assertEqual(0, code, result)
        updated = tasks_path.read_text(encoding="utf-8")
        self.assertIn("- [x] 1.1.", updated)
        self.assertIn("- [ ] 1.2", updated)
        self.assertEqual(["state.json"], sorted(item.name for item in state_path.parent.iterdir()))

    def test_openspec_completion_honors_authority_lock(self) -> None:
        change_dir = self.root / "openspec" / "changes" / "locked"
        change_dir.mkdir(parents=True)
        tasks_path = change_dir / "tasks.md"
        tasks_path.write_text("- [ ] 1.1 Implement locked task\n", encoding="utf-8")
        code, result = self.run_cli(
            "init",
            *self.state_arguments("locked"),
            "--source",
            "openspec",
            "--adapter",
            "manual",
            "--implementation-ref",
            "implementation",
            "--goal",
            "goal",
            "--change",
            "locked",
            "--tasks-path",
            str(tasks_path.relative_to(self.root)),
        )
        self.assertEqual(0, code, result)
        state_path = Path(result["state"])
        before_state = state_path.read_bytes()
        before_tasks = tasks_path.read_bytes()
        authority_lock = statectl._authority_lock_path(tasks_path)
        authority_lock.mkdir()
        try:
            code, result = self.run_cli(
                "complete",
                *self.state_arguments("locked"),
                "--expected-revision",
                "0",
                "--summary",
                "implemented",
                "--evidence",
                "verification passed",
            )
        finally:
            authority_lock.rmdir()
        self.assertEqual(statectl.ERROR_CONFLICT, code)
        self.assertEqual("authority_busy", result["error"])
        self.assertEqual(before_state, state_path.read_bytes())
        self.assertEqual(before_tasks, tasks_path.read_bytes())

    def test_openspec_compare_and_swap_preserves_external_edit(self) -> None:
        change_dir = self.root / "openspec" / "changes" / "cas"
        change_dir.mkdir(parents=True)
        tasks_path = change_dir / "tasks.md"
        original = "- [ ] 1.1 Implement CAS task\n"
        tasks_path.write_text(original, encoding="utf-8")
        code, result = self.run_cli(
            "init",
            *self.state_arguments("cas"),
            "--source",
            "openspec",
            "--adapter",
            "manual",
            "--implementation-ref",
            "implementation",
            "--goal",
            "goal",
            "--change",
            "cas",
            "--tasks-path",
            str(tasks_path.relative_to(self.root)),
        )
        self.assertEqual(0, code, result)
        state_path = Path(result["state"])
        before_state = state_path.read_bytes()
        real_atomic_json = statectl._atomic_write_json

        def external_edit_after_journal(target: Path, value: dict) -> None:
            real_atomic_json(target, value)
            if target.name == statectl.TRANSACTION_MARKER:
                tasks_path.write_text(original + "External note\n", encoding="utf-8")

        with mock.patch.object(
            statectl,
            "_atomic_write_json",
            side_effect=external_edit_after_journal,
        ):
            code, result = self.run_cli(
                "complete",
                *self.state_arguments("cas"),
                "--expected-revision",
                "0",
                "--summary",
                "implemented",
                "--evidence",
                "verification passed",
            )
        self.assertEqual(statectl.ERROR_CONFLICT, code)
        self.assertEqual("revision_conflict", result["error"])
        self.assertEqual(original + "External note\n", tasks_path.read_text(encoding="utf-8"))
        self.assertEqual(before_state, state_path.read_bytes())
        self.assertFalse((state_path.parent / statectl.TRANSACTION_MARKER).exists())
        self.assertFalse((state_path.parent / statectl.TRANSACTION_BACKUP).exists())

    def test_openspec_task_definition_drift_is_rejected(self) -> None:
        change_dir = self.root / "openspec" / "changes" / "drift"
        change_dir.mkdir(parents=True)
        tasks_path = change_dir / "tasks.md"
        tasks_path.write_text("- [ ] 1.1 Original task\n", encoding="utf-8")
        code, result = self.run_cli(
            "init",
            *self.state_arguments("drift"),
            "--source",
            "openspec",
            "--adapter",
            "manual",
            "--implementation-ref",
            "implementation",
            "--goal",
            "goal",
            "--change",
            "drift",
            "--tasks-path",
            str(tasks_path.relative_to(self.root)),
        )
        self.assertEqual(0, code, result)
        tasks_path.write_text("- [ ] 1.1 Changed task\n", encoding="utf-8")
        code, result = self.run_cli(
            "validate", *self.state_arguments("drift")
        )
        self.assertEqual(statectl.ERROR_INVALID, code)
        self.assertIn("drifted", result["message"])

    def test_block_is_a_ready_resumable_checkpoint(self) -> None:
        path = self.init_standalone()
        code, result = self.run_cli(
            "block",
            *self.state_arguments(),
            "--expected-revision",
            "0",
            "--reason",
            "external approval required",
        )
        self.assertEqual(0, code, result)
        state = self.load_state(path)
        self.assertEqual("blocked", state["status"])
        self.assertTrue(state["checkpoint"]["ready"])

    def test_declared_regression_checks_are_a_completion_gate(self) -> None:
        path = self.init_standalone(task_json=[{
            "id": "contract",
            "title": "Implement contract",
            "done_when": ["behavior works"],
            "contracts": ["same key and body replays"],
            "regression_checks": ["idempotency-replay"],
        }])
        code, result = self.run_cli(
            "complete",
            *self.state_arguments(),
            "--expected-revision", "0",
            "--summary", "implemented",
            "--evidence", "manual inspection",
        )
        self.assertEqual(statectl.ERROR_INVALID, code)
        self.assertIn("idempotency-replay", result["message"])
        self.assertEqual(0, self.load_state(path)["revision"])
        check = json.dumps({
            "id": "idempotency-replay",
            "status": "passed",
            "summary": "black-box replay check passed",
        })
        code, result = self.run_cli(
            "complete",
            *self.state_arguments(),
            "--expected-revision", "0",
            "--summary", "implemented",
            "--check-json", check,
        )
        self.assertEqual(0, code, result)

    def test_context_map_sends_only_relevant_entries(self) -> None:
        self.init_standalone(task_json=[{
            "id": "auth",
            "title": "Implement auth",
            "done_when": ["auth works"],
            "affected_areas": ["auth"],
            "reads": ["internal/auth/service.go"],
        }])
        for entry in (
            {"path": "internal/auth/service.go", "purpose": "auth orchestration", "areas": ["auth"], "symbols": ["Service"]},
            {"path": "internal/billing/store.go", "purpose": "billing store", "areas": ["billing"], "symbols": ["Store"]},
        ):
            code, result = self.run_cli(
                "context-map-update",
                *self.state_arguments(),
                "--expected-revision", "0",
                "--entry-json", json.dumps(entry),
            )
            self.assertEqual(0, code, result)
        packet_path = self.root / "context-packet.json"
        code, result = self.run_cli(
            "packet", *self.state_arguments(), "--output", str(packet_path)
        )
        self.assertEqual(0, code, result)
        packet = json.loads(packet_path.read_text(encoding="utf-8"))
        self.assertEqual(["internal/auth/service.go"], [item["path"] for item in packet["context"]])

    def test_periodic_architecture_review_blocks_next_packet(self) -> None:
        tasks = [
            {"id": str(index), "title": f"Task {index}", "done_when": ["done"]}
            for index in range(1, 4)
        ]
        arguments = [
            "init", *self.state_arguments(), "--source", "standalone",
            "--profile", "lite", "--adapter", "manual",
            "--implementation-ref", "implementation", "--goal", "goal",
            "--review-interval", "2",
        ]
        for task in tasks:
            arguments.extend(["--task-json", json.dumps(task)])
        code, result = self.run_cli(*arguments)
        self.assertEqual(0, code, result)
        for revision in (0, 1):
            code, result = self.run_cli(
                "complete", *self.state_arguments(),
                "--expected-revision", str(revision),
                "--summary", "done", "--evidence", "check passed",
            )
            self.assertEqual(0, code, result)
        packet_path = self.root / "blocked-packet.json"
        code, result = self.run_cli(
            "packet", *self.state_arguments(), "--output", str(packet_path)
        )
        self.assertEqual(statectl.ERROR_INVALID, code)
        self.assertIn("architecture review", result["message"])
        code, result = self.run_cli(
            "architecture-review", *self.state_arguments(),
            "--expected-revision", "2", "--summary", "boundaries remain valid",
            "--evidence", "dependency graph inspected",
        )
        self.assertEqual(0, code, result)
        code, result = self.run_cli(
            "packet", *self.state_arguments(), "--output", str(packet_path)
        )
        self.assertEqual(0, code, result)

    def test_cross_area_change_requires_integration_chunk(self) -> None:
        path = self.init_standalone(task_json=[
            {
                "id": "cross", "title": "Change auth and audit", "done_when": ["done"],
                "affected_areas": ["auth", "audit"], "requires_bridge": True,
            },
            {"id": "plain", "title": "Continue implementation", "done_when": ["done"]},
        ])
        code, result = self.run_cli(
            "complete", *self.state_arguments(), "--expected-revision", "0",
            "--summary", "done", "--evidence", "checks passed",
        )
        self.assertEqual(0, code, result)
        self.assertTrue(result["review_required"])
        code, result = self.run_cli(
            "architecture-review", *self.state_arguments(), "--expected-revision", "1",
            "--summary", "reviewed", "--evidence", "boundaries inspected",
        )
        self.assertEqual(0, code, result)
        code, result = self.run_cli(
            "packet", *self.state_arguments(), "--output", str(self.root / "bridge.json")
        )
        self.assertEqual(statectl.ERROR_INVALID, code)
        self.assertIn("integration chunk", result["message"])
        self.assertTrue(self.load_state(path)["quality"]["pending_bridge"])

    def test_architecture_review_compacts_multibyte_summary_and_evidence(self) -> None:
        path = self.init_standalone(task_json=[
            {"id": "one", "title": "First", "done_when": ["done"]},
            {"id": "two", "title": "Second", "done_when": ["done"]},
            {"id": "three", "title": "Third", "done_when": ["done"]},
        ])
        for revision in (0, 1):
            code, result = self.run_cli(
                "complete", *self.state_arguments(),
                "--expected-revision", str(revision),
                "--summary", "done", "--evidence", "check passed",
            )
            self.assertEqual(0, code, result)
        summary = "Проверены архитектурные границы. " * 30
        evidence = "Проверены gofmt, go test и go vet. " * 20
        self.assertLessEqual(len(summary.encode("utf-8")), statectl.MAX_REVIEW_INPUT_BYTES)
        self.assertLessEqual(len(evidence.encode("utf-8")), statectl.MAX_REVIEW_INPUT_BYTES)
        code, result = self.run_cli(
            "architecture-review", *self.state_arguments(),
            "--expected-revision", "2", "--summary", summary,
            "--evidence", evidence,
        )
        self.assertEqual(0, code, result)
        last_review = self.load_state(path)["quality"]["last_review"]
        self.assertLessEqual(
            len(last_review.encode("utf-8")),
            statectl.MAX_REVIEW_RECORD_BYTES,
        )
        self.assertIn("; evidence: ", last_review)

    def test_handoff_recommends_reset_when_cohesion_changes_and_runtime_supports_it(self) -> None:
        path = self.init_standalone(task_json=[
            {"id": "auth", "title": "Auth", "done_when": ["done"], "cohesion_key": "auth"},
            {"id": "audit", "title": "Audit", "done_when": ["done"], "cohesion_key": "audit"},
        ])
        state = self.load_state(path)
        state["execution"]["capabilities"]["fresh_context"] = True
        path.write_text(json.dumps(state, ensure_ascii=False), encoding="utf-8")
        code, result = self.run_cli(
            "complete", *self.state_arguments(), "--expected-revision", "0",
            "--summary", "done", "--evidence", "checks passed",
        )
        self.assertEqual(0, code, result)
        self.assertEqual("reset", result["next_handoff"])

    def test_migrate_v3_state_and_ledger_is_explicit_and_revision_bound(self) -> None:
        path = self.init_standalone()
        state = self.load_state(path)
        state["schema_version"] = 3
        state.pop("quality")
        for field in (
            "checks", "kind", "cohesion_key", "affected_areas", "reads",
            "writes", "contracts", "regression_checks", "requires_bridge",
        ):
            state["active_task"].pop(field)
        path.write_text(json.dumps(state, ensure_ascii=False), encoding="utf-8")
        ledger_path = path.parent / "tasks.json"
        ledger = json.loads(ledger_path.read_text(encoding="utf-8"))
        ledger["schema_version"] = 1
        for task in ledger["tasks"]:
            for field in (
                "kind", "cohesion_key", "affected_areas", "reads", "writes",
                "contracts", "regression_checks", "requires_bridge",
            ):
                task.pop(field)
        ledger_path.write_text(json.dumps(ledger, ensure_ascii=False), encoding="utf-8")
        code, result = self.run_cli(
            "migrate", *self.state_arguments(), "--expected-revision", "7"
        )
        self.assertEqual(statectl.ERROR_CONFLICT, code)
        code, result = self.run_cli(
            "migrate", *self.state_arguments(), "--expected-revision", "0"
        )
        self.assertEqual(0, code, result)
        self.assertTrue(result["migrated"])
        migrated = self.load_state(path)
        self.assertEqual(4, migrated["schema_version"])
        self.assertEqual(1, migrated["revision"])
        self.assertIn("quality", migrated)
        self.assertEqual(2, json.loads(ledger_path.read_text(encoding="utf-8"))["schema_version"])


if __name__ == "__main__":
    unittest.main()
