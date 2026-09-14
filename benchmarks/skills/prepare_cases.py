#!/usr/bin/env python3
"""Prepare independent model packets; never invoke a model or include its rubric."""

import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
SPEC = importlib.util.spec_from_file_location("case_installer", ROOT / "install.py")
installer = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(installer)


def prepare(output, model, case_ids=None):
    cases = json.loads((HERE / "cases.json").read_text())["cases"]
    known = {case["id"] for case in cases}
    if case_ids and set(case_ids) - known:
        raise ValueError("Unknown case IDs: " + ", ".join(sorted(set(case_ids) - known)))
    selected = [case for case in cases if not case_ids or case["id"] in case_ids]
    catalog = {skill["id"]: skill for skill in installer.load_catalog()["skills"]}
    output = Path(output).resolve()
    output.mkdir(parents=True, exist_ok=False)
    records = []
    for case in selected:
        directory = output / case["id"]
        directory.mkdir()
        skill_path = directory / "skill"
        skill = catalog[case["skill"]]
        options = installer.resolve_options(skill, set(case["options"]))
        installer.validate_selection_groups(skill, options)
        installer.build_skill(skill, options, skill_path)
        inputs = directory / "input"
        inputs.mkdir()
        (directory / "output").mkdir()
        for filename in case["fixtures"]:
            shutil.copy2(HERE / "fixtures" / filename, inputs / filename)
        prompt = (
            f"Используй skill {skill_path / 'SKILL.md'}.\n"
            f"Входные файлы: {inputs}.\n\n{case['prompt']}\n\n"
            f"Результаты записывай только в {directory / 'output'}. "
            "Не меняй input или skill. Не читай другие case directories, "
            "репозиторий-источник, rubric или результаты других моделей. "
            "Не используй сеть, production или установку пакетов. "
            "В output/result.md укажи ответ и реально выполненные проверки.\n"
        )
        (directory / "prompt.md").write_text(prompt, encoding="utf-8")
        records.append({
            "case": case["id"], "skill": case["skill"], "model": model,
            "options": case["options"],
            "skill_sha256": installer.tree_sha256(skill_path),
            "input_sha256": installer.tree_sha256(inputs),
            "prompt_sha256": hashlib.sha256(prompt.encode()).hexdigest(),
            "prompt": str(directory / "prompt.md"),
        })
    manifest = {"schema_version": 1, "model_requests": 0, "cases": records}
    (output / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--model", choices=["gpt-5.6-luna", "gpt-5.6-terra"], default="gpt-5.6-luna")
    parser.add_argument("--case", action="append")
    args = parser.parse_args()
    try:
        manifest = prepare(args.output, args.model, args.case)
    except (OSError, ValueError, installer.InstallError) as error:
        parser.exit(2, str(error) + "\n")
    print(json.dumps({"output": str(args.output.resolve()), "cases": len(manifest["cases"]), "model_requests": 0}))


if __name__ == "__main__":
    main()
