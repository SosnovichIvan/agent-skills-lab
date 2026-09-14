import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("skill_installer", ROOT / "install.py")
installer = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(installer)


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.catalog = installer.load_catalog()
        self.by_id = {skill["id"]: skill for skill in self.catalog["skills"]}

    def test_catalog_references_exist(self):
        self.assertEqual(2, self.catalog["schema_version"])
        self.assertIn("codex", self.catalog["agent_adapters"])
        self.assertIn("frontend-engineering", self.by_id)
        self.assertIn("backend-engineering", self.by_id)
        self.assertIn("ui-ux-design", self.by_id)
        frontend = self.by_id["frontend-engineering"]
        source = ROOT / frontend["source"] / "references"
        actual = {path.relative_to(ROOT / frontend["source"]).as_posix() for path in source.rglob("*.md")}
        declared = installer.reference_paths(frontend, {item["id"] for item in frontend["options"]})
        self.assertEqual(actual, declared)

    def test_catalog_user_skills_have_no_hard_skill_dependencies(self):
        for skill in self.catalog["skills"]:
            self.assertNotIn(
                "requires_skills",
                skill,
                f"{skill['id']} must remain independently installable",
            )

    def test_nextjs_resolves_react_dependency(self):
        skill = self.by_id["frontend-engineering"]
        resolved = installer.resolve_options(skill, {"typescript", "nextjs", "structure-fsd"})
        self.assertEqual(
            [item["id"] for item in resolved],
            ["typescript", "react", "nextjs", "structure-fsd"],
        )
        installer.validate_selection_groups(skill, resolved)

    def test_modular_install_copies_only_selected_references(self):
        skill = self.by_id["frontend-engineering"]
        options = installer.resolve_options(skill, {"typescript", "nextjs", "structure-flexible"})
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            skills_root = project / ".agents" / "skills"
            installer.install_skill(skill, options, skills_root)
            installed = skills_root / skill["id"]

            self.assertTrue((installed / "references/common/architecture.md").is_file())
            self.assertTrue((installed / "references/common/code-conventions.md").is_file())
            self.assertTrue((installed / "references/options/typescript.md").is_file())
            self.assertTrue((installed / "references/options/react.md").is_file())
            self.assertTrue((installed / "references/options/nextjs.md").is_file())
            self.assertTrue((installed / "references/options/structure-flexible.md").is_file())
            self.assertFalse((installed / "references/options/angular.md").exists())
            self.assertFalse((installed / "references/options/structure-fsd.md").exists())
            skill_text = (installed / "SKILL.md").read_text(encoding="utf-8")
            self.assertIn("references/options/nextjs.md", skill_text)
            self.assertNotIn("references/options/angular.md", skill_text)

    def test_frontend_requires_exactly_one_structure(self):
        skill = self.by_id["frontend-engineering"]
        without_structure = installer.resolve_options(skill, {"typescript", "react"})
        with self.assertRaises(installer.InstallError):
            installer.validate_selection_groups(skill, without_structure)
        with_two = installer.resolve_options(
            skill, {"typescript", "react", "structure-fsd", "structure-flexible"}
        )
        with self.assertRaises(installer.InstallError):
            installer.validate_selection_groups(skill, with_two)

    def test_custom_structure_is_copied_as_parent_reference(self):
        skill = self.by_id["frontend-engineering"]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            custom = root / "my-structure.txt"
            custom.write_text(
                "# Project structure\n\nUse modules by business capability.\n",
                encoding="utf-8",
            )
            options = installer.resolve_options(skill, {"typescript", "react"})
            options.append(installer.custom_structure_option(str(custom)))
            installer.validate_selection_groups(skill, options)
            skills_root = root / "skills"
            installer.install_skill(skill, options, skills_root)
            installed_reference = (
                skills_root
                / "frontend-engineering/references/options/custom-structure.md"
            )
            self.assertEqual(
                installed_reference.read_text(encoding="utf-8"),
                custom.read_text(encoding="utf-8"),
            )
            self.assertFalse((skills_root / "custom-structure.md").exists())

    def test_agent_block_is_updated_without_duplication(self):
        skill = self.by_id["backend-engineering"]
        options = installer.resolve_options(skill, {"go"})
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            skills_root = project / ".agents" / "skills"
            agent_file = project / "AGENTS.md"
            installer.install_skill(skill, options, skills_root)
            manifest = installer.update_manifest(skills_root, [(skill, options)])
            block = installer.render_agent_block(manifest, skills_root, agent_file)
            installer.update_agent_file(agent_file, block)
            installer.update_agent_file(agent_file, block)

            content = agent_file.read_text(encoding="utf-8")
            self.assertEqual(content.count(installer.AGENT_START), 1)
            self.assertIn("Profile: Go", content)
            saved = json.loads((skills_root / installer.MANIFEST_NAME).read_text(encoding="utf-8"))
            self.assertEqual(saved["skills"]["backend-engineering"]["options"], ["go"])

    def test_bundled_skill_keeps_its_references_and_scripts(self):
        skill = self.by_id["execution-state"]
        with tempfile.TemporaryDirectory() as temporary:
            skills_root = Path(temporary) / "skills"
            installer.install_skill(skill, [], skills_root)
            installed = skills_root / "execution-state"
            self.assertTrue((installed / "references/state-schema.md").is_file())
            self.assertTrue((installed / "scripts/statectl.py").is_file())

    def test_ui_ux_design_installs_and_runs_independently(self):
        skill = self.by_id["ui-ux-design"]
        self.assertNotIn("requires_skills", skill)
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            result = installer.main(
                [
                    "--skill", "ui-ux-design",
                    "--agent", "codex",
                    "--project-dir", str(project),
                    "--yes",
                ]
            )
            self.assertEqual(0, result)
            skills_root = project / ".agents/skills"
            installed = skills_root / "ui-ux-design"
            self.assertTrue((installed / "data/ux-guidelines.csv").is_file())
            self.assertTrue((installed / "references/quick-reference.md").is_file())
            self.assertTrue((installed / "LICENSE.upstream").is_file())
            self.assertFalse((skills_root / "frontend-engineering").exists())
            self.assertFalse((skills_root / "execution-state").exists())

            manifest = json.loads(
                (skills_root / installer.MANIFEST_NAME).read_text(encoding="utf-8")
            )
            self.assertEqual({"ui-ux-design"}, set(manifest["skills"]))
            self.assertEqual({}, manifest["policies"])

            search = subprocess.run(
                [
                    sys.executable,
                    str(installed / "scripts/search.py"),
                    "keyboard focus modal",
                    "--domain", "ux",
                    "--json",
                ],
                text=True,
                capture_output=True,
                check=True,
            )
            payload = json.loads(search.stdout)
            self.assertEqual("ux", payload["domain"])
            self.assertGreater(payload["count"], 0)

    def test_independent_skills_can_be_installed_together(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            result = installer.main(
                [
                    "--skill", "ui-ux-design",
                    "--skill", "frontend-engineering",
                    "--option", "frontend-engineering:typescript",
                    "--option", "frontend-engineering:react",
                    "--option", "frontend-engineering:structure-fsd",
                    "--agent", "codex",
                    "--project-dir", str(project),
                    "--yes",
                ]
            )
            self.assertEqual(0, result)
            skills_root = project / ".agents/skills"
            manifest = json.loads(
                (skills_root / installer.MANIFEST_NAME).read_text(encoding="utf-8")
            )
            self.assertEqual(
                {"ui-ux-design", "frontend-engineering"},
                set(manifest["skills"]),
            )
            self.assertEqual([], manifest["skills"]["ui-ux-design"]["required_by"])
            self.assertEqual([], manifest["skills"]["frontend-engineering"]["required_by"])
            instructions = (project / "AGENTS.md").read_text(encoding="utf-8")
            self.assertIn("ui-ux-design/SKILL.md", instructions)
            self.assertIn("frontend-engineering/SKILL.md", instructions)

    def test_skill_dependencies_are_resolved_before_parent(self):
        catalog = {
            "skills": [
                {"id": "execution-state"},
                {"id": "frontend", "requires_skills": [{"id": "execution-state"}]},
            ]
        }
        resolved, required_by = installer.resolve_skill_dependencies(catalog, ["frontend"])
        self.assertEqual(["execution-state", "frontend"], [skill["id"] for skill in resolved])
        self.assertEqual(["frontend"], required_by["execution-state"])

    def test_circular_skill_dependency_is_rejected(self):
        catalog = {
            "skills": [
                {"id": "one", "requires_skills": ["two"]},
                {"id": "two", "requires_skills": ["one"]},
            ]
        }
        with self.assertRaises(installer.InstallError):
            installer.resolve_skill_dependencies(catalog, ["one"])

    def test_reinstall_backup_is_outside_discoverable_skills_root(self):
        skill = self.by_id["execution-state"]
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            skills_root = project / ".agents" / "skills"
            installer.install_skill(skill, [], skills_root)
            backup = installer.install_skill(skill, [], skills_root)
            second_backup = installer.install_skill(skill, [], skills_root)
            self.assertIsNotNone(backup)
            assert backup is not None
            assert second_backup is not None
            self.assertTrue(backup.is_relative_to(project / ".agents" / ".agent-skills-lab" / "backups"))
            self.assertTrue(second_backup.is_relative_to(project / ".agents" / ".agent-skills-lab" / "backups"))
            duplicate_skills = [path for path in skills_root.glob("*/SKILL.md") if path.parent.name != "execution-state"]
            self.assertEqual([], duplicate_skills)

    def test_execution_state_install_writes_all_tasks_policy(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            result = installer.main(
                [
                    "--skill", "execution-state",
                    "--agent", "codex",
                    "--project-dir", str(project),
                    "--yes",
                ]
            )
            self.assertEqual(0, result)
            instructions = (project / "AGENTS.md").read_text(encoding="utf-8")
            self.assertIn("Scope: `all_tasks`", instructions)
            self.assertIn("A `passthrough` decision is valid", instructions)
            manifest = json.loads(
                (project / ".agents/skills/.agent-skills-lab.json").read_text(encoding="utf-8")
            )
            self.assertEqual(2, manifest["schema_version"])
            self.assertEqual("1.0.0", manifest["installer_version"])
            self.assertEqual("1.0.0", manifest["skills"]["execution-state"]["version"])
            self.assertEqual(64, len(manifest["skills"]["execution-state"]["content_sha256"]))

    def test_required_execution_state_defaults_to_dependent_tasks(self):
        catalog = json.loads(json.dumps(self.catalog))
        backend = next(skill for skill in catalog["skills"] if skill["id"] == "backend-engineering")
        backend["requires_skills"] = [{"id": "execution-state", "activation": "required"}]
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            with mock.patch.object(installer, "load_catalog", return_value=catalog):
                result = installer.main(
                    [
                        "--skill", "backend-engineering",
                        "--option", "backend-engineering:go",
                        "--agent", "codex",
                        "--project-dir", str(project),
                        "--yes",
                    ]
                )
            self.assertEqual(0, result)
            manifest = json.loads(
                (project / ".agents/skills/.agent-skills-lab.json").read_text(encoding="utf-8")
            )
            self.assertEqual("dependent_tasks", manifest["policies"]["execution-state"]["scope"])
            self.assertEqual(
                ["backend-engineering"],
                manifest["skills"]["execution-state"]["required_by"],
            )

    def test_atomic_install_rolls_back_skill_and_files(self):
        skill = self.by_id["execution-state"]
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            skills_root = project / ".agents" / "skills"
            target = skills_root / "execution-state"
            target.mkdir(parents=True)
            (target / "old.txt").write_text("old", encoding="utf-8")
            agent_file = project / "AGENTS.md"
            agent_file.write_text("old agent\n", encoding="utf-8")
            manifest = installer.build_manifest(
                skills_root,
                [(skill, [])],
                agent_adapter="codex",
            )
            with mock.patch.object(installer, "write_atomic", side_effect=OSError("write failed")):
                with self.assertRaises(OSError):
                    installer.apply_installation(
                        project_dir=project,
                        skills_root=skills_root,
                        agent_file=agent_file,
                        installed=[(skill, [])],
                        manifest=manifest,
                        agent_content="new agent\n",
                        strict_codex=False,
                    )
            self.assertEqual("old", (target / "old.txt").read_text(encoding="utf-8"))
            self.assertEqual("old agent\n", agent_file.read_text(encoding="utf-8"))

    def test_strict_codex_hooks_block_until_route_succeeds(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            result = installer.main(
                [
                    "--skill", "execution-state",
                    "--agent", "codex",
                    "--execution-state-policy", "all_tasks",
                    "--enforcement", "strict",
                    "--project-dir", str(project),
                    "--yes",
                ]
            )
            self.assertEqual(0, result)
            hook = project / ".codex/hooks/execution_state_policy.py"
            statectl = project / ".agents/skills/execution-state/scripts/statectl.py"

            def invoke(payload):
                return subprocess.run(
                    [sys.executable, str(hook)],
                    input=json.dumps(payload),
                    text=True,
                    capture_output=True,
                    check=True,
                ).stdout

            invoke({"hook_event_name": "UserPromptSubmit", "turn_id": "turn-1", "prompt": "task"})
            denied = json.loads(
                invoke(
                    {
                        "hook_event_name": "PreToolUse",
                        "turn_id": "turn-1",
                        "tool_input": {"command": "git status"},
                    }
                )
            )
            self.assertEqual("deny", denied["hookSpecificOutput"]["permissionDecision"])
            route_command = f'{sys.executable} "{statectl}" route --expected-turns 1'
            self.assertEqual(
                "",
                invoke(
                    {
                        "hook_event_name": "PreToolUse",
                        "turn_id": "turn-1",
                        "tool_input": {"command": route_command},
                    }
                ),
            )
            chained = route_command + " && git status"
            chained_denial = json.loads(
                invoke(
                    {
                        "hook_event_name": "PreToolUse",
                        "turn_id": "turn-2",
                        "tool_input": {"command": chained},
                    }
                )
            )
            self.assertEqual(
                "deny", chained_denial["hookSpecificOutput"]["permissionDecision"]
            )
            invoke(
                {
                    "hook_event_name": "PostToolUse",
                    "turn_id": "turn-1",
                    "tool_input": {"command": route_command},
                    "tool_response": {"exit_code": 0},
                }
            )
            self.assertEqual(
                "",
                invoke(
                    {
                        "hook_event_name": "PreToolUse",
                        "turn_id": "turn-1",
                        "tool_input": {"command": "git status"},
                    }
                ),
            )

    def test_strict_enforcement_rejects_unsupported_adapter(self):
        with tempfile.TemporaryDirectory() as temporary:
            result = installer.main(
                [
                    "--skill", "execution-state",
                    "--agent", "claude",
                    "--enforcement", "strict",
                    "--project-dir", temporary,
                    "--yes",
                ]
            )
            self.assertEqual(2, result)

    def test_codex_hook_render_is_idempotent_and_preserves_other_hooks(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            hooks_file = root / "hooks.json"
            hooks_file.write_text(
                json.dumps(
                    {
                        "hooks": {
                            "PreToolUse": [
                                {
                                    "matcher": "^Bash$",
                                    "hooks": [{"type": "command", "command": "python3 other.py"}],
                                }
                            ]
                        }
                    }
                ),
                encoding="utf-8",
            )
            hook_script = root / "execution_state_policy.py"
            first = installer.render_codex_hooks(hooks_file, hook_script)
            hooks_file.write_text(first, encoding="utf-8")
            second = json.loads(installer.render_codex_hooks(hooks_file, hook_script))
            groups = second["hooks"]["PreToolUse"]
            commands = [handler["command"] for group in groups for handler in group["hooks"]]
            self.assertEqual(1, sum("execution_state_policy.py" in item for item in commands))
            self.assertIn("python3 other.py", commands)

            hooks_file.write_text(json.dumps(second), encoding="utf-8")
            disabled = json.loads(
                installer.render_codex_hooks(hooks_file, hook_script, enabled=False)
            )
            remaining = [
                handler["command"]
                for group in disabled["hooks"]["PreToolUse"]
                for handler in group["hooks"]
            ]
            self.assertEqual(["python3 other.py"], remaining)

    def test_reinstall_in_instruction_mode_removes_strict_hook(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            common = [
                "--skill", "execution-state",
                "--agent", "codex",
                "--project-dir", str(project),
                "--yes",
            ]
            self.assertEqual(0, installer.main(common + ["--enforcement", "strict"]))
            hook_script = project / ".codex/hooks/execution_state_policy.py"
            self.assertTrue(hook_script.is_file())
            self.assertEqual(0, installer.main(common + ["--enforcement", "instructions"]))
            self.assertFalse(hook_script.exists())
            hooks = (project / ".codex/hooks.json").read_text(encoding="utf-8")
            self.assertNotIn("execution_state_policy.py", hooks)


if __name__ == "__main__":
    unittest.main()
