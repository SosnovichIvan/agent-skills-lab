from __future__ import annotations

import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SCRIPTS = Path(__file__).resolve().parents[2] / "skills" / "execution-state" / "scripts"
sys.path.insert(0, str(SCRIPTS))

import cli_adapters  # noqa: E402


class CliAdapterTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name) / "project with spaces Ж"
        self.root.mkdir()
        self.bin_dir = self.root / "fake bin"
        self.bin_dir.mkdir()
        self.prompt = self.root / "worker prompt.md"
        self.packet = self.root / "packet.json"
        self.prompt.write_text("worker instructions", encoding="utf-8")
        self.packet.write_text('{"packet_version":"1.0.0"}', encoding="utf-8")

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def executable(self, name: str) -> Path:
        path = self.bin_dir / name
        path.write_text(
            f'#!/bin/sh\nif [ "$1" = "--version" ]; then echo "{name} 1.0"; exit 0; fi\nexit 99\n',
            encoding="utf-8",
        )
        path.chmod(0o755)
        return path

    def test_unknown_runtime_falls_back_to_manual(self) -> None:
        result = cli_adapters.probe_runtime(
            "unknown-vendor", search_path=str(self.bin_dir)
        )
        self.assertEqual("manual", result["selected"]["id"])
        self.assertIn("unknown runtime", result["reason"])

    def test_auto_is_manual_when_discovery_is_ambiguous(self) -> None:
        self.executable("codex")
        self.executable("claude")
        with mock.patch.object(
            cli_adapters, "_probe_version", return_value=("test 1.0", None)
        ):
            result = cli_adapters.probe_runtime("auto", search_path=str(self.bin_dir))
        self.assertEqual("manual", result["selected"]["id"])
        self.assertEqual(2, len(result["discovered"]))
        self.assertIn("multiple", result["reason"])

    def test_explicit_codex_plan_is_argv_only_and_does_not_execute(self) -> None:
        executable = self.executable("codex")
        marker = self.root / "should-not-exist"
        executable.write_text(
            f'#!/bin/sh\nif [ "$1" = "--version" ]; then echo "codex 1.0"; exit 0; fi\ntouch \'{marker}\'\nexit 99\n',
            encoding="utf-8",
        )
        executable.chmod(0o755)
        plan = cli_adapters.build_runtime_plan(
            requested="codex",
            prompt_path=self.prompt,
            packet_path=self.packet,
            project_root=self.root,
            search_path=str(self.bin_dir),
        )
        self.assertEqual("codex", plan["adapter"])
        self.assertEqual(str(executable.resolve()), plan["argv"][0])
        self.assertEqual(
            [str(self.prompt.resolve()), str(self.packet.resolve())],
            plan["stdin_files"],
        )
        self.assertFalse(plan["executes"])
        self.assertFalse(marker.exists())
        self.assertNotIn("shell", plan)
        self.assertEqual("execution-state.stdin/1.0.0", plan["stdin"]["protocol"])
        self.assertEqual(
            ["worker_prompt", "worker_packet"],
            [part["name"] for part in plan["stdin"]["parts"]],
        )

    def test_failed_version_probe_does_not_confirm_capabilities(self) -> None:
        executable = self.bin_dir / "codex"
        executable.write_text("#!/bin/sh\nexit 2\n", encoding="utf-8")
        executable.chmod(0o755)
        result = cli_adapters.probe_runtime("codex", search_path=str(self.bin_dir))
        self.assertEqual("manual", result["selected"]["id"])
        self.assertFalse(cli_adapters.supports_reset(result))
        self.assertEqual("codex", result["unavailable"][0]["id"])

    def test_environment_adapter_has_wrapper_precedence_only_for_auto(self) -> None:
        self.executable("codex")
        self.executable("claude")
        with mock.patch.dict(os.environ, {"EXECUTION_STATE_ADAPTER": "claude"}):
            automatic = cli_adapters.probe_runtime(
                "auto", search_path=str(self.bin_dir)
            )
            explicit = cli_adapters.probe_runtime(
                "codex", search_path=str(self.bin_dir)
            )
        self.assertEqual("claude", automatic["selected"]["id"])
        self.assertEqual("environment", automatic["selection_source"])
        self.assertEqual("codex", explicit["selected"]["id"])
        self.assertEqual("argument", explicit["selection_source"])

    def test_invalid_environment_adapter_falls_back_without_guessing(self) -> None:
        self.executable("codex")
        with mock.patch.dict(
            os.environ, {"EXECUTION_STATE_ADAPTER": "../../unsafe"}
        ):
            result = cli_adapters.probe_runtime(
                "auto", search_path=str(self.bin_dir)
            )
        self.assertEqual("manual", result["selected"]["id"])
        self.assertIn("not a valid adapter id", result["reason"])

    def test_gemini_fresh_process_does_not_claim_persistence_control(self) -> None:
        self.executable("gemini")
        with mock.patch.object(
            cli_adapters, "_probe_version", return_value=("gemini 1.0", None)
        ):
            result = cli_adapters.probe_runtime(
                "gemini", search_path=str(self.bin_dir)
            )
        capabilities = result["selected"]["capabilities"]
        self.assertTrue(capabilities["fresh_context"])
        self.assertFalse(capabilities["in_place_compaction"])
        self.assertFalse(capabilities["session_persistence_control"])
        with mock.patch.object(
            cli_adapters, "_probe_version", return_value=("gemini 1.0", None)
        ):
            plan = cli_adapters.build_runtime_plan(
                requested="gemini",
                prompt_path=self.prompt,
                packet_path=self.packet,
                project_root=self.root,
                search_path=str(self.bin_dir),
            )
        self.assertEqual("gemini", plan["adapter"])
        self.assertEqual(
            [str((self.bin_dir / "gemini").resolve()), "--output-format", "stream-json"],
            plan["argv"],
        )
        self.assertNotIn("-p", plan["argv"])

    def test_claude_stream_json_plan_is_verbose_and_non_persistent(self) -> None:
        self.executable("claude")
        with mock.patch.object(
            cli_adapters, "_probe_version", return_value=("claude 1.0", None)
        ):
            plan = cli_adapters.build_runtime_plan(
                requested="claude",
                prompt_path=self.prompt,
                packet_path=self.packet,
                project_root=self.root,
                search_path=str(self.bin_dir),
            )
        self.assertEqual("claude", plan["adapter"])
        self.assertIn("-p", plan["argv"])
        self.assertIn("--no-session-persistence", plan["argv"])
        self.assertIn("stream-json", plan["argv"])
        self.assertIn("--verbose", plan["argv"])

    def test_safe_custom_manifest_and_unicode_paths(self) -> None:
        custom_exec = self.executable("custom-agent")
        manifest_path = self.root / "adapter manifest.json"
        manifest_path.write_text(
            json.dumps(
                {
                    "schema_version": "1.0.0",
                    "id": "custom-agent",
                    "executables": [str(custom_exec)],
                    "capabilities": {
                        "fresh_context": True,
                        "in_place_compaction": False,
                        "machine_output": True,
                        "session_persistence_control": False,
                        "usage_metrics": False,
                    },
                    "launch": {
                        "strategy": "fresh_process",
                        "argv": ["{executable}", "run", "{project_root}"],
                        "stdin_files": ["{prompt_path}", "{packet_path}"],
                    },
                },
                ensure_ascii=False,
            ),
            encoding="utf-8",
        )
        plan = cli_adapters.build_runtime_plan(
            requested="custom-agent",
            prompt_path=self.prompt,
            packet_path=self.packet,
            project_root=self.root,
            custom_manifest=manifest_path,
            search_path="",
            trust_custom=True,
        )
        self.assertEqual("custom-agent", plan["adapter"])
        self.assertEqual(str(self.root.resolve()), plan["argv"][-1])
        self.assertEqual(str(custom_exec.resolve()), plan["argv"][0])

    def test_custom_executable_requires_explicit_trust_before_version_probe(self) -> None:
        marker = self.root / "version-probed"
        executable = self.bin_dir / "custom-agent"
        executable.write_text(
            f"#!/bin/sh\ntouch '{marker}'\necho 'custom 1.0'\n",
            encoding="utf-8",
        )
        executable.chmod(0o755)
        manifest_path = self.root / "trusted-only.json"
        manifest_path.write_text(
            json.dumps(
                {
                    "schema_version": "1.0.0",
                    "id": "custom-agent",
                    "executables": [str(executable)],
                    "capabilities": {
                        "fresh_context": True,
                        "in_place_compaction": False,
                        "machine_output": True,
                        "session_persistence_control": False,
                        "usage_metrics": False,
                    },
                    "launch": {
                        "strategy": "fresh_process",
                        "argv": ["{executable}"],
                        "stdin_files": ["{prompt_path}", "{packet_path}"],
                    },
                }
            ),
            encoding="utf-8",
        )
        result = cli_adapters.probe_runtime(
            "custom-agent",
            custom_manifest=manifest_path,
            search_path="",
        )
        self.assertEqual("manual", result["selected"]["id"])
        self.assertFalse(marker.exists())
        trusted = cli_adapters.probe_runtime(
            "custom-agent",
            custom_manifest=manifest_path,
            search_path="",
            trust_custom=True,
        )
        self.assertEqual("custom-agent", trusted["selected"]["id"])
        self.assertTrue(marker.exists())

    def test_custom_manifest_rejects_commands_and_unknown_templates(self) -> None:
        self.executable("custom-agent")
        manifest_path = self.root / "unsafe.json"
        manifest_path.write_text(
            json.dumps(
                {
                    "schema_version": "1.0.0",
                    "id": "custom-agent",
                    "executables": ["custom-agent"],
                    "capabilities": {
                        "fresh_context": True,
                        "in_place_compaction": False,
                        "machine_output": True,
                        "session_persistence_control": False,
                        "usage_metrics": False,
                    },
                    "launch": {
                        "strategy": "fresh_process",
                        "argv": ["{executable}", "{arbitrary_command}"],
                        "stdin_files": [],
                    },
                }
            ),
            encoding="utf-8",
        )
        with self.assertRaises(cli_adapters.AdapterError):
            cli_adapters.load_manifest(manifest_path)

    def test_custom_manifest_rejects_permission_bypass_options(self) -> None:
        for index, dangerous in enumerate(
            (
                "--yolo",
                "--dangerously-skip-permissions",
                "--dangerously-bypass-approvals-and-sandbox",
            )
        ):
            manifest_path = self.root / f"unsafe-permissions-{index}.json"
            manifest_path.write_text(
                json.dumps(
                    {
                        "schema_version": "1.0.0",
                        "id": f"custom-{index}",
                        "executables": ["custom-agent"],
                        "capabilities": {
                            "fresh_context": True,
                            "in_place_compaction": False,
                            "machine_output": True,
                            "session_persistence_control": False,
                            "usage_metrics": False,
                        },
                        "launch": {
                            "strategy": "fresh_process",
                            "argv": ["{executable}", dangerous],
                            "stdin_files": ["{prompt_path}", "{packet_path}"],
                        },
                    }
                ),
                encoding="utf-8",
            )
            with self.subTest(option=dangerous):
                with self.assertRaises(cli_adapters.AdapterError):
                    cli_adapters.load_manifest(manifest_path)

    def test_custom_manifest_requires_whole_element_placeholders(self) -> None:
        manifest_path = self.root / "partial-placeholder.json"
        manifest_path.write_text(
            json.dumps(
                {
                    "schema_version": "1.0.0",
                    "id": "custom-partial",
                    "executables": ["custom-agent"],
                    "capabilities": {
                        "fresh_context": True,
                        "in_place_compaction": False,
                        "machine_output": True,
                        "session_persistence_control": False,
                        "usage_metrics": False,
                    },
                    "launch": {
                        "strategy": "fresh_process",
                        "argv": ["{executable}", "--root={project_root}"],
                        "stdin_files": ["{prompt_path}", "{packet_path}"],
                    },
                }
            ),
            encoding="utf-8",
        )
        with self.assertRaises(cli_adapters.AdapterError):
            cli_adapters.load_manifest(manifest_path)

    def test_custom_manifest_cannot_plan_an_indirect_shell(self) -> None:
        manifest_path = self.root / "shell-adapter.json"
        manifest_path.write_text(
            json.dumps(
                {
                    "schema_version": "1.0.0",
                    "id": "shell-adapter",
                    "executables": ["bash"],
                    "capabilities": {
                        "fresh_context": True,
                        "in_place_compaction": False,
                        "machine_output": True,
                        "session_persistence_control": False,
                        "usage_metrics": False,
                    },
                    "launch": {
                        "strategy": "fresh_process",
                        "argv": ["{executable}", "-c", "payload"],
                        "stdin_files": [],
                    },
                }
            ),
            encoding="utf-8",
        )
        with self.assertRaises(cli_adapters.AdapterError):
            cli_adapters.load_manifest(manifest_path)

    def test_builtin_manifests_are_valid(self) -> None:
        manifests = cli_adapters.builtin_manifests()
        self.assertEqual({"manual", "codex", "claude", "gemini"}, set(manifests))
        for name in ("codex", "claude", "gemini"):
            self.assertTrue(manifests[name].capabilities["fresh_context"])
            self.assertFalse(manifests[name].capabilities["in_place_compaction"])


if __name__ == "__main__":
    unittest.main()
