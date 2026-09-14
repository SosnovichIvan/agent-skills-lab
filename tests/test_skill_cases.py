import importlib.util
from pathlib import Path
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("skill_cases", ROOT / "benchmarks/skills/prepare_cases.py")
cases = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(cases)


class SkillCaseTests(unittest.TestCase):
    def test_packets_are_self_contained_without_evaluator_answers(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "run"
            manifest = cases.prepare(output, "gpt-5.6-luna")
            self.assertEqual(manifest["model_requests"], 0)
            self.assertEqual(len(manifest["cases"]), 12)
            self.assertFalse(list(output.rglob("rubric.json")))
            for record in manifest["cases"]:
                folder = output / record["case"]
                self.assertTrue((folder / "skill/SKILL.md").is_file())
                self.assertTrue((folder / "output").is_dir())
                self.assertEqual(record["skill_sha256"], cases.installer.tree_sha256(folder / "skill"))
                self.assertEqual(record["input_sha256"], cases.installer.tree_sha256(folder / "input"))
            pg = output / "database-ddl/skill/references/options"
            self.assertEqual({p.name for p in pg.iterdir()}, {"postgresql.md"})
            self.assertTrue((output / "frontend-race/input/search.tsx").is_file())

    def test_existing_results_are_preserved_and_unknown_case_writes_nothing(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "run"
            cases.prepare(output, "gpt-5.6-terra", ["frontend-clean"])
            marker = output / "result.txt"
            marker.write_text("keep me")
            with self.assertRaises(FileExistsError):
                cases.prepare(output, "gpt-5.6-luna", ["frontend-clean"])
            self.assertEqual(marker.read_text(), "keep me")
            missing = Path(temporary) / "unknown"
            with self.assertRaises(ValueError):
                cases.prepare(missing, "gpt-5.6-luna", ["not-a-case"])
            self.assertFalse(missing.exists())


if __name__ == "__main__":
    unittest.main()
