"""Validate deliverable packages, not the prose of skill instructions."""

import importlib.util
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("package_installer", ROOT / "install.py")
installer = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(installer)


class SkillPackageTests(unittest.TestCase):
    def test_every_option_builds_with_resolvable_local_markdown_links(self):
        """Catch references that work in source but disappear in selected packages."""
        with tempfile.TemporaryDirectory() as temporary:
            for skill in installer.load_catalog()["skills"]:
                selections = [set()]
                if skill["mode"] == "modular":
                    baseline = {"typescript", "structure-flexible"} if skill["id"] == "frontend-engineering" else set()
                    selections = [baseline | {option["id"]} for option in skill["options"]]
                for index, selected in enumerate(selections):
                    if "structure-fsd" in selected:
                        selected.discard("structure-flexible")
                    options = installer.resolve_options(skill, selected)
                    installer.validate_selection_groups(skill, options)
                    target = Path(temporary) / skill["id"] / str(index)
                    installer.build_skill(skill, options, target)
                    with self.subTest(skill=skill["id"], options=selected):
                        files = [target / "SKILL.md"]
                        files.extend((target / "references").rglob("*.md"))
                        for file in files:
                            for link in re.findall(r"\]\(([^\s)]+)\)", file.read_text()):
                                if "://" in link or link.startswith("#"):
                                    continue
                                path = (file.parent / link.split("#", 1)[0]).resolve()
                                self.assertTrue(path.exists(), f"Broken installed link: {file}: {link}")
                                self.assertTrue(path.is_relative_to(target.resolve()), f"Non-self-contained link: {link}")

    def test_local_caches_do_not_change_bundled_or_modular_packages(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "source"
            source.mkdir()
            (source / "SKILL.md").write_text(
                "---\nname: sample\ndescription: Sample\n---\n"
                + installer.PROFILE_START + "\n" + installer.PROFILE_END + "\n"
            )
            scripts = source / "scripts"
            scripts.mkdir()
            (scripts / "helper.py").write_text("print('ok')\n")
            with mock.patch.object(installer, "ROOT", root):
                for mode in ("bundled", "modular"):
                    skill = {"source": "source", "mode": mode}
                    before = root / (mode + "-before")
                    installer.build_skill(skill, [], before)
                    (source / ".DS_Store").write_bytes(b"finder")
                    (source / "loose.pyc").write_bytes(b"cache")
                    cache = scripts / "__pycache__"
                    cache.mkdir(exist_ok=True)
                    (cache / "helper.pyc").write_bytes(b"cache")
                    after = root / (mode + "-after")
                    installer.build_skill(skill, [], after)
                    self.assertEqual(installer.tree_sha256(before), installer.tree_sha256(after))
                    self.assertTrue((after / "scripts/helper.py").is_file())
                    self.assertFalse((after / "scripts/__pycache__").exists())
                    self.assertFalse((after / ".DS_Store").exists())
                    self.assertFalse((after / "loose.pyc").exists())
                    (source / ".DS_Store").unlink()
                    (source / "loose.pyc").unlink()
                    (cache / "helper.pyc").unlink()
                    cache.rmdir()

    def test_new_skills_install_independently_with_version_and_hash(self):
        names = ["frontend-review", "backend-review", "database-engineering",
                 "database-review", "project-architecture", "content-writing",
                 "ux-writing", "brand-logo-design", "icon-system"]
        for name in names:
            with self.subTest(skill=name), tempfile.TemporaryDirectory() as temporary:
                command = [sys.executable, str(ROOT / "install.py"), "--skill", name,
                           "--project-dir", temporary, "--yes"]
                if name.startswith("database-"):
                    command.extend(["--option", name + ":postgresql"])
                completed = subprocess.run(command, capture_output=True, text=True)
                self.assertEqual(completed.returncode, 0, completed.stderr)
                skills_root = Path(temporary) / ".agents/skills"
                manifest = json.loads((skills_root / ".agent-skills-lab.json").read_text())
                self.assertEqual(set(manifest["skills"]), {name})
                record = manifest["skills"][name]
                self.assertEqual(record["version"], "0.1.0")
                self.assertEqual(record["content_sha256"], installer.tree_sha256(skills_root / name))
                if name.startswith("database-"):
                    installed = skills_root / name / "references/options"
                    self.assertEqual({p.name for p in installed.iterdir()}, {"postgresql.md"})


if __name__ == "__main__":
    unittest.main()
