#!/usr/bin/env python3
"""Interactive installer for Agent Skills Lab skills."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import sys
import tempfile
from datetime import datetime, timezone


ROOT = Path(__file__).resolve().parent
CATALOG_PATH = ROOT / "skills" / "catalog.json"
PROFILE_START = "<!-- agent-skills-lab:references:start -->"
PROFILE_END = "<!-- agent-skills-lab:references:end -->"
AGENT_START = "<!-- agent-skills-lab:skills:start -->"
AGENT_END = "<!-- agent-skills-lab:skills:end -->"
POLICY_START = "<!-- agent-skills-lab:execution-policy:start -->"
POLICY_END = "<!-- agent-skills-lab:execution-policy:end -->"
MANIFEST_NAME = ".agent-skills-lab.json"
INSTALLER_VERSION = "1.0.0"
INSTALLER_ROOT = ROOT / "installer"
PROMPTS_ROOT = INSTALLER_ROOT / "prompts"
CODEX_HOOK_SOURCE = INSTALLER_ROOT / "adapters" / "codex" / "execution_state_policy.py"
POLICY_SCOPES = {"explicit", "dependent_tasks", "all_tasks"}
ENFORCEMENT_MODES = {"instructions", "strict"}
BUILD_IGNORE = shutil.ignore_patterns("__pycache__", "*.pyc", "*.pyo", ".DS_Store")


class InstallError(RuntimeError):
    pass


def load_catalog() -> dict:
    catalog = json.loads(CATALOG_PATH.read_text(encoding="utf-8"))
    if catalog.get("schema_version") != 2:
        raise InstallError("Unsupported catalog schema_version")
    adapters = catalog.get("agent_adapters")
    if not isinstance(adapters, dict) or not adapters:
        raise InstallError("Catalog must declare agent_adapters")
    for adapter_id, adapter in adapters.items():
        if not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", adapter_id):
            raise InstallError(f"Invalid agent adapter id: {adapter_id}")
        for field in ("name", "description", "skills_dir", "agent_file"):
            if not isinstance(adapter.get(field), str) or not adapter[field].strip():
                raise InstallError(f"Agent adapter {adapter_id} is missing {field}")
    seen: set[str] = set()
    for skill in catalog.get("skills", []):
        skill_id = skill["id"]
        if not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", skill_id):
            raise InstallError(f"Invalid skill id: {skill_id}")
        if skill_id in seen:
            raise InstallError(f"Duplicate skill id: {skill_id}")
        seen.add(skill_id)
        source = ROOT / skill["source"]
        try:
            source.resolve().relative_to(ROOT)
        except ValueError as error:
            raise InstallError(f"Skill source escapes repository: {source}") from error
        if not (source / "SKILL.md").is_file():
            raise InstallError(f"Missing SKILL.md for {skill_id}: {source}")
        option_ids = {option["id"] for option in skill.get("options", [])}
        groups = skill.get("selection_groups", [])
        group_ids = {group["id"] for group in groups}
        if len(group_ids) != len(groups):
            raise InstallError(f"Duplicate selection group for {skill_id}")
        for group in groups:
            minimum = group.get("min", 0)
            maximum = group.get("max")
            if minimum < 0 or (maximum is not None and maximum < minimum):
                raise InstallError(f"Invalid selection limits for {skill_id}:{group['id']}")
        for option in skill.get("options", []):
            if groups and option.get("group") not in group_ids:
                raise InstallError(f"Unknown selection group for {skill_id}:{option['id']}")
            unknown = set(option.get("requires", [])) - option_ids
            if unknown:
                raise InstallError(f"Unknown dependencies for {skill_id}: {sorted(unknown)}")
        declared_references = reference_paths(skill, option_ids)
        for relative in declared_references:
            reference = (source / relative).resolve()
            try:
                reference.relative_to(source.resolve())
            except ValueError as error:
                raise InstallError(f"Reference escapes skill {skill_id}: {relative}") from error
            if not reference.is_file():
                raise InstallError(f"Missing reference for {skill_id}: {relative}")
        if skill.get("mode") == "modular":
            actual_references = {
                path.relative_to(source).as_posix()
                for path in (source / "references").rglob("*.md")
            }
            if actual_references != declared_references:
                missing = sorted(actual_references - declared_references)
                stale = sorted(declared_references - actual_references)
                details = []
                if missing:
                    details.append(f"unregistered={missing}")
                if stale:
                    details.append(f"missing={stale}")
                raise InstallError(f"Reference catalog mismatch for {skill_id}: {'; '.join(details)}")
        activation = skill.get("activation")
        if activation is not None:
            supported = set(activation.get("supported_scopes", []))
            default_scope = activation.get("default_scope")
            if not supported or not supported <= POLICY_SCOPES or default_scope not in supported:
                raise InstallError(f"Invalid activation policy for {skill_id}")
    by_id = {skill["id"]: skill for skill in catalog.get("skills", [])}
    for skill in by_id.values():
        for dependency in skill.get("requires_skills", []):
            if isinstance(dependency, str):
                dependency_id = dependency
            elif isinstance(dependency, dict):
                dependency_id = dependency.get("id")
                if dependency.get("activation", "required") != "required":
                    raise InstallError(
                        f"Unsupported dependency activation for {skill['id']}: {dependency.get('activation')}"
                    )
            else:
                raise InstallError(f"Invalid skill dependency for {skill['id']}")
            if dependency_id not in by_id:
                raise InstallError(f"Unknown skill dependency for {skill['id']}: {dependency_id}")
            if dependency_id == skill["id"]:
                raise InstallError(f"Skill cannot depend on itself: {skill['id']}")
    resolve_skill_dependencies(catalog, list(by_id))
    return catalog


def resolve_skill_dependencies(catalog: dict, requested_ids: list[str]) -> tuple[list[dict], dict[str, list[str]]]:
    by_id = {skill["id"]: skill for skill in catalog.get("skills", [])}
    ordered: list[dict] = []
    visiting: set[str] = set()
    visited: set[str] = set()
    required_by: dict[str, list[str]] = {}

    def visit(skill_id: str) -> None:
        if skill_id in visiting:
            raise InstallError(f"Circular skill dependency at {skill_id}")
        if skill_id in visited:
            return
        if skill_id not in by_id:
            raise InstallError(f"Unknown skill: {skill_id}")
        visiting.add(skill_id)
        for dependency in by_id[skill_id].get("requires_skills", []):
            dependency_id = dependency if isinstance(dependency, str) else dependency["id"]
            required_by.setdefault(dependency_id, []).append(skill_id)
            visit(dependency_id)
        visiting.remove(skill_id)
        visited.add(skill_id)
        ordered.append(by_id[skill_id])

    for requested_id in requested_ids:
        visit(requested_id)
    return ordered, {key: list(dict.fromkeys(value)) for key, value in required_by.items()}


def reference_paths(skill: dict, option_ids: set[str]) -> set[str]:
    paths = set(skill.get("common_references", []))
    for option in skill.get("options", []):
        if option["id"] in option_ids:
            paths.update(option.get("references", []))
    return paths


def parse_selection(
    raw: str,
    count: int,
    allow_all: bool = True,
    allow_empty: bool = False,
    maximum: int | None = None,
) -> list[int]:
    value = raw.strip().lower()
    if not value and allow_empty:
        return []
    if allow_all and value in {"all", "все", "*"}:
        return list(range(count))
    selected: list[int] = []
    for token in value.replace(" ", "").split(","):
        if not token:
            continue
        if not token.isdigit() or not 1 <= int(token) <= count:
            raise InstallError(f"Некорректный номер: {token}")
        index = int(token) - 1
        if index not in selected:
            selected.append(index)
    if not selected:
        raise InstallError("Нужно выбрать хотя бы один пункт")
    if maximum is not None and len(selected) > maximum:
        raise InstallError(f"Можно выбрать не более {maximum}")
    return selected


def prompt_selection(title: str, items: list[dict], allow_all: bool = True) -> list[dict]:
    print(f"\n{title}")
    for index, item in enumerate(items, start=1):
        kind = f" [{item['kind']}]" if item.get("kind") else ""
        print(f"  {index}. {item['name']}{kind} — {item['description']}")
    hint = "номера через запятую или all" if allow_all else "номера через запятую"
    while True:
        try:
            indexes = parse_selection(input(f"Выбор ({hint}): "), len(items), allow_all)
            return [items[index] for index in indexes]
        except InstallError as error:
            print(f"Ошибка: {error}")


def custom_structure_option(raw_path: str) -> dict:
    source = Path(raw_path).expanduser().resolve()
    if not source.is_file():
        raise InstallError(f"Custom structure file not found: {source}")
    try:
        source.read_text(encoding="utf-8")
    except UnicodeDecodeError as error:
        raise InstallError(f"Custom structure must be UTF-8 text: {source}") from error
    return {
        "id": "custom-structure",
        "kind": "structure",
        "group": "structure",
        "name": f"Custom structure ({source.name})",
        "description": "Пользовательские правила структуры кода; при конфликте они имеют приоритет над встроенными рекомендациями.",
        "references": ["references/options/custom-structure.md"],
        "source_reference": str(source),
    }


def prompt_grouped_options(skill: dict) -> list[dict]:
    selected_ids: set[str] = set()
    custom_options: list[dict] = []
    for group in skill.get("selection_groups", []):
        items = [option for option in skill.get("options", []) if option.get("group") == group["id"]]
        display_items = list(items)
        if group.get("allow_custom_file"):
            display_items.append(
                {
                    "id": "__custom_file__",
                    "name": "Собственный файл правил",
                    "description": "Указать путь к UTF-8 файлу со структурой проекта.",
                    "kind": "custom",
                }
            )
        print(f"\n{group['name']}: {group['description']}")
        for index, item in enumerate(display_items, start=1):
            print(f"  {index}. {item['name']} [{item['kind']}] — {item['description']}")
        optional = group.get("min", 0) == 0
        hint = "номера через запятую"
        if optional:
            hint += "; Enter — пропустить"
        while True:
            try:
                indexes = parse_selection(
                    input(f"Выбор ({hint}): "),
                    len(display_items),
                    allow_all=group.get("max") != 1,
                    allow_empty=optional,
                    maximum=group.get("max"),
                )
                chosen = [display_items[index] for index in indexes]
                custom_chosen = [item for item in chosen if item["id"] == "__custom_file__"]
                if custom_chosen:
                    custom_options.append(custom_structure_option(input("Путь к файлу структуры: ").strip()))
                selected_ids.update(item["id"] for item in chosen if item["id"] != "__custom_file__")
                break
            except InstallError as error:
                print(f"Ошибка: {error}")
    return resolve_options(skill, selected_ids) + custom_options


def resolve_options(skill: dict, selected_ids: set[str]) -> list[dict]:
    by_id = {option["id"]: option for option in skill.get("options", [])}
    unknown = selected_ids - set(by_id)
    if unknown:
        raise InstallError(f"Unknown options for {skill['id']}: {', '.join(sorted(unknown))}")
    resolved = set(selected_ids)
    pending = list(selected_ids)
    while pending:
        current = pending.pop()
        for required in by_id[current].get("requires", []):
            if required not in resolved:
                resolved.add(required)
                pending.append(required)
    return [option for option in skill.get("options", []) if option["id"] in resolved]


def validate_selection_groups(skill: dict, options: list[dict]) -> None:
    for group in skill.get("selection_groups", []):
        count = sum(option.get("group") == group["id"] for option in options)
        minimum = group.get("min", 0)
        maximum = group.get("max")
        if count < minimum:
            raise InstallError(
                f"{skill['id']} requires at least {minimum} selection(s) in group {group['id']}"
            )
        if maximum is not None and count > maximum:
            raise InstallError(
                f"{skill['id']} allows at most {maximum} selection(s) in group {group['id']}"
            )


def render_profile(options: list[dict]) -> str:
    lines = [PROFILE_START, "Установленный профиль. Обязательно прочитай все выбранные references:", ""]
    for option in options:
        for reference in option.get("references", []):
            lines.append(f"- [{option['name']}]({reference}) — {option['description']}")
    lines.append(PROFILE_END)
    return "\n".join(lines)


def replace_block(text: str, start: str, end: str, replacement: str) -> str:
    start_at = text.find(start)
    end_at = text.find(end)
    if start_at < 0 or end_at < start_at:
        raise InstallError(f"Managed block not found: {start}")
    end_at += len(end)
    return text[:start_at] + replacement + text[end_at:]


def build_skill(skill: dict, options: list[dict], destination: Path) -> None:
    source = ROOT / skill["source"]
    if skill["mode"] == "bundled":
        shutil.copytree(source, destination, ignore=BUILD_IGNORE)
        return

    destination.mkdir(parents=True)
    ignored = BUILD_IGNORE(str(source), [child.name for child in source.iterdir()])
    for child in source.iterdir():
        if child.name == "references" or child.name in ignored:
            continue
        target = destination / child.name
        if child.is_dir():
            shutil.copytree(child, target, ignore=BUILD_IGNORE)
        else:
            shutil.copy2(child, target)

    chosen_paths = set(skill.get("common_references", []))
    for option in options:
        if not option.get("source_reference"):
            chosen_paths.update(option.get("references", []))
    for relative in sorted(chosen_paths):
        target = destination / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source / relative, target)
    for option in options:
        custom_source = option.get("source_reference")
        if not custom_source:
            continue
        for relative in option.get("references", []):
            target = destination / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(Path(custom_source).read_text(encoding="utf-8"), encoding="utf-8")

    skill_file = destination / "SKILL.md"
    content = skill_file.read_text(encoding="utf-8")
    content = replace_block(content, PROFILE_START, PROFILE_END, render_profile(options))
    skill_file.write_text(content, encoding="utf-8")


def backup_path(path: Path, backup_root: Path | None = None) -> Path:
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    root = backup_root or (path.parent.parent / ".agent-skills-lab" / "backups")
    root.mkdir(parents=True, exist_ok=True)
    candidate = root / f"{path.name}-{stamp}"
    counter = 2
    while candidate.exists():
        candidate = root / f"{path.name}-{stamp}-{counter}"
        counter += 1
    return candidate


def install_skill(
    skill: dict,
    options: list[dict],
    skills_root: Path,
    backup_root: Path | None = None,
) -> Path | None:
    skills_root.mkdir(parents=True, exist_ok=True)
    target = skills_root / skill["id"]
    staging_root = Path(tempfile.mkdtemp(prefix=f".{skill['id']}-", dir=skills_root))
    stage = staging_root / "payload"
    backup: Path | None = None
    try:
        build_skill(skill, options, stage)
        if target.exists() or target.is_symlink():
            backup = backup_path(target, backup_root)
            shutil.move(str(target), str(backup))
        os.replace(stage, target)
        staging_root.rmdir()
    except Exception:
        if staging_root.exists():
            shutil.rmtree(staging_root)
        if backup is not None and backup.exists() and not target.exists():
            shutil.move(str(backup), str(target))
        raise
    return backup


def read_manifest(skills_root: Path) -> dict:
    path = skills_root / MANIFEST_NAME
    if not path.exists():
        return {
            "schema_version": 2,
            "installer_version": INSTALLER_VERSION,
            "agent_adapter": None,
            "skills": {},
            "policies": {},
        }
    data = json.loads(path.read_text(encoding="utf-8"))
    if data.get("schema_version") == 1 and isinstance(data.get("skills"), dict):
        data = {
            "schema_version": 2,
            "installer_version": INSTALLER_VERSION,
            "agent_adapter": None,
            "skills": data["skills"],
            "policies": {},
        }
    if data.get("schema_version") != 2 or not isinstance(data.get("skills"), dict):
        raise InstallError(f"Invalid installer manifest: {path}")
    if not isinstance(data.get("policies", {}), dict):
        raise InstallError(f"Invalid installer policies: {path}")
    data.setdefault("agent_adapter", None)
    data.setdefault("policies", {})
    execution_policy = data["policies"].get("execution-state")
    if execution_policy is not None:
        if not isinstance(execution_policy, dict):
            raise InstallError(f"Invalid execution-state policy: {path}")
        if execution_policy.get("scope") not in POLICY_SCOPES:
            raise InstallError(f"Invalid execution-state policy scope: {path}")
        if execution_policy.get("enforcement") not in ENFORCEMENT_MODES:
            raise InstallError(f"Invalid execution-state enforcement: {path}")
    data["installer_version"] = INSTALLER_VERSION
    return data


def write_atomic(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(prefix=f".{path.name}-", dir=path.parent)
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            handle.write(content)
        os.replace(temporary, path)
    except Exception:
        temporary.unlink(missing_ok=True)
        raise


def tree_sha256(root: Path) -> str:
    digest = hashlib.sha256()
    for path in sorted(item for item in root.rglob("*") if item.is_file()):
        relative = path.relative_to(root).as_posix().encode("utf-8")
        digest.update(len(relative).to_bytes(4, "big"))
        digest.update(relative)
        payload = path.read_bytes()
        digest.update(len(payload).to_bytes(8, "big"))
        digest.update(payload)
    return digest.hexdigest()


def build_manifest(
    skills_root: Path,
    installed: list[tuple[dict, list[dict]]],
    *,
    agent_adapter: str | None = None,
    required_by: dict[str, list[str]] | None = None,
    execution_policy: dict | None = None,
) -> dict:
    manifest = read_manifest(skills_root)
    now = datetime.now(timezone.utc).isoformat()
    required_by = required_by or {}
    for skill, options in installed:
        release_path = ROOT / skill["source"] / "release.json"
        version = None
        if release_path.is_file():
            version = json.loads(release_path.read_text(encoding="utf-8")).get("version")
        manifest["skills"][skill["id"]] = {
            "name": skill["name"],
            "description": skill["description"],
            "options": [option["id"] for option in options],
            "option_names": [option["name"] for option in options],
            "version": version,
            "required_by": required_by.get(skill["id"], []),
            "installed_at": now,
        }
    if agent_adapter is not None:
        manifest["agent_adapter"] = agent_adapter
    if execution_policy is not None:
        manifest["policies"]["execution-state"] = execution_policy
    return manifest


def update_manifest(
    skills_root: Path,
    installed: list[tuple[dict, list[dict]]],
    *,
    agent_adapter: str | None = None,
    required_by: dict[str, list[str]] | None = None,
    execution_policy: dict | None = None,
) -> dict:
    manifest = build_manifest(
        skills_root,
        installed,
        agent_adapter=agent_adapter,
        required_by=required_by,
        execution_policy=execution_policy,
    )
    for skill, _ in installed:
        installed_path = skills_root / skill["id"]
        if installed_path.is_dir():
            manifest["skills"][skill["id"]]["content_sha256"] = tree_sha256(installed_path)
    write_atomic(skills_root / MANIFEST_NAME, json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    return manifest


def render_agent_block(manifest: dict, skills_root: Path, agent_file: Path) -> str:
    entries: list[str] = []
    for skill_id, record in sorted(manifest["skills"].items()):
        skill_file = skills_root / skill_id / "SKILL.md"
        relative = os.path.relpath(skill_file, start=agent_file.parent)
        options = ", ".join(record.get("option_names", [])) or "bundled"
        required = record.get("required_by", [])
        dependency = f" Required by: {', '.join(required)}." if required else ""
        entries.append(
            f"- `{skill_id}`: read [{relative}]({relative}) for matching tasks. "
            f"Profile: {options}.{dependency}"
        )
    template = (PROMPTS_ROOT / "installed-skills.md").read_text(encoding="utf-8").strip()
    return template.format(skill_entries="\n".join(entries))


def render_execution_policy(manifest: dict, skills_root: Path, agent_file: Path) -> str | None:
    policy = manifest.get("policies", {}).get("execution-state")
    if not policy:
        return None
    scope = policy["scope"]
    rules = {
        "explicit": "Apply execution-state only when the user explicitly invokes it.",
        "dependent_tasks": (
            "Before a task governed by a skill that requires `execution-state`, route the task "
            "through execution-state before implementation or mutation begins."
        ),
        "all_tasks": (
            "Before any task, route it through execution-state before implementation or mutation "
            "begins. Read-only inspection needed to estimate the workload is allowed."
        ),
    }
    skill_file = skills_root / "execution-state" / "SKILL.md"
    relative = os.path.relpath(skill_file, start=agent_file.parent)
    template = (PROMPTS_ROOT / "execution-state-policy.md").read_text(encoding="utf-8").strip()
    return template.format(
        scope=scope,
        enforcement=policy["enforcement"],
        policy_rule=rules[scope],
        skill_path=relative,
    )


def update_agent_file(agent_file: Path, block: str) -> None:
    if agent_file.exists():
        content = agent_file.read_text(encoding="utf-8")
    else:
        content = "# Agent instructions\n"
    if AGENT_START in content or AGENT_END in content:
        if AGENT_START not in content or AGENT_END not in content:
            raise InstallError(f"Incomplete managed block in {agent_file}")
        content = replace_block(content, AGENT_START, AGENT_END, block)
    else:
        content = content.rstrip() + "\n\n" + block + "\n"
    write_atomic(agent_file, content)


def updated_agent_content(agent_file: Path, skills_block: str, policy_block: str | None) -> str:
    if agent_file.exists():
        content = agent_file.read_text(encoding="utf-8")
    else:
        content = "# Agent instructions\n"
    for start, end, replacement in (
        (AGENT_START, AGENT_END, skills_block),
        (POLICY_START, POLICY_END, policy_block),
    ):
        has_start = start in content
        has_end = end in content
        if has_start != has_end:
            raise InstallError(f"Incomplete managed block in {agent_file}: {start}")
        if replacement is None:
            continue
        if has_start:
            content = replace_block(content, start, end, replacement)
        else:
            content = content.rstrip() + "\n\n" + replacement + "\n"
    return content


def render_codex_hooks(hooks_file: Path, hook_script: Path, *, enabled: bool = True) -> str:
    if hooks_file.exists():
        data = json.loads(hooks_file.read_text(encoding="utf-8"))
    else:
        data = {"description": "Project lifecycle hooks."}
    if not isinstance(data, dict):
        raise InstallError(f"Invalid Codex hooks document: {hooks_file}")
    hooks = data.setdefault("hooks", {})
    if not isinstance(hooks, dict):
        raise InstallError(f"Invalid Codex hooks object: {hooks_file}")
    command = f'python3 "{hook_script}"'
    for event in ("UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"):
        groups = hooks.setdefault(event, [])
        if not isinstance(groups, list):
            raise InstallError(f"Invalid Codex hook event {event}: {hooks_file}")
        filtered = []
        for group in groups:
            handlers = group.get("hooks", []) if isinstance(group, dict) else []
            managed = any(
                isinstance(handler, dict)
                and "execution_state_policy.py" in str(handler.get("command", ""))
                for handler in handlers
            )
            if not managed:
                filtered.append(group)
        if enabled:
            new_group: dict = {
                "hooks": [
                    {
                        "type": "command",
                        "command": command,
                        "commandWindows": f'py -3 "{hook_script}"',
                        "timeout": 10,
                        "statusMessage": "Enforcing execution-state routing",
                    }
                ]
            }
            if event == "PreToolUse":
                new_group["matcher"] = ".*"
            elif event == "PostToolUse":
                new_group["matcher"] = "^Bash$"
            filtered.append(new_group)
        hooks[event] = filtered
    return json.dumps(data, ensure_ascii=False, indent=2) + "\n"


def remove_path(path: Path) -> None:
    if path.is_symlink() or path.is_file():
        path.unlink(missing_ok=True)
    elif path.exists():
        shutil.rmtree(path)


def restore_file(path: Path, original: bytes | None) -> None:
    if original is None:
        path.unlink(missing_ok=True)
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(prefix=f".{path.name}-restore-", dir=path.parent)
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(original)
        os.replace(temporary, path)
    except Exception:
        temporary.unlink(missing_ok=True)
        raise


def apply_installation(
    *,
    project_dir: Path,
    skills_root: Path,
    agent_file: Path,
    installed: list[tuple[dict, list[dict]]],
    manifest: dict,
    agent_content: str,
    strict_codex: bool,
) -> list[Path]:
    manifest_path = skills_root / MANIFEST_NAME
    policy_dir = project_dir / ".agent-skills-lab"
    policy_path = policy_dir / "policy.json"
    strict_context_path = policy_dir / "codex-strict-context.md"
    hook_script = project_dir / ".codex" / "hooks" / "execution_state_policy.py"
    hooks_file = project_dir / ".codex" / "hooks.json"
    file_contents: dict[Path, str | None] = {
        manifest_path: json.dumps(manifest, ensure_ascii=False, indent=2) + "\n",
        agent_file: agent_content,
        policy_path: json.dumps(
            {
                "schema_version": 1,
                "execution_state": manifest.get("policies", {}).get("execution-state"),
            },
            ensure_ascii=False,
            indent=2,
        )
        + "\n",
    }
    codex_adapter = manifest.get("agent_adapter") == "codex"
    if strict_codex:
        skill_file = skills_root / "execution-state" / "SKILL.md"
        statectl_file = skill_file.parent / "scripts" / "statectl.py"
        strict_template = (PROMPTS_ROOT / "codex-strict-context.md").read_text(encoding="utf-8")
        file_contents[strict_context_path] = strict_template.format(
            statectl_path=statectl_file,
            skill_path=skill_file,
        )
        file_contents[hook_script] = CODEX_HOOK_SOURCE.read_text(encoding="utf-8")
        file_contents[hooks_file] = render_codex_hooks(hooks_file, hook_script, enabled=True)
    elif codex_adapter and hooks_file.exists():
        file_contents[strict_context_path] = None
        file_contents[hook_script] = None
        file_contents[hooks_file] = render_codex_hooks(hooks_file, hook_script, enabled=False)

    originals = {path: path.read_bytes() if path.is_file() else None for path in file_contents}
    staging_root = Path(tempfile.mkdtemp(prefix=".agent-skills-lab-", dir=project_dir))
    staged_skills = staging_root / "skills"
    backups: dict[Path, Path] = {}
    backup_root = policy_dir / "backups"
    installed_targets: list[Path] = []
    try:
        for skill, options in installed:
            staged = staged_skills / skill["id"]
            build_skill(skill, options, staged)
            record = manifest.get("skills", {}).get(skill["id"])
            if not isinstance(record, dict):
                raise InstallError(f"Manifest is missing installed skill: {skill['id']}")
            record["content_sha256"] = tree_sha256(staged)
        file_contents[manifest_path] = json.dumps(manifest, ensure_ascii=False, indent=2) + "\n"
        skills_root.mkdir(parents=True, exist_ok=True)
        for skill, _ in installed:
            target = skills_root / skill["id"]
            if target.exists() or target.is_symlink():
                backup = backup_path(target, backup_root)
                shutil.move(str(target), str(backup))
                backups[target] = backup
            shutil.move(str(staged_skills / skill["id"]), str(target))
            installed_targets.append(target)
        for path, content in file_contents.items():
            if content is None:
                remove_path(path)
            else:
                write_atomic(path, content)
    except Exception:
        for target in reversed(installed_targets):
            remove_path(target)
        for target, backup in backups.items():
            if backup.exists():
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.move(str(backup), str(target))
        for path, original in originals.items():
            restore_file(path, original)
        raise
    finally:
        if staging_root.exists():
            shutil.rmtree(staging_root)
    return list(backups.values())


def defaulted_input(prompt: str, default: str) -> str:
    value = input(f"{prompt} [{default}]: ").strip()
    return value or default


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Install Agent Skills Lab skills")
    parser.add_argument("--list", action="store_true", help="show available skills and options")
    parser.add_argument("--skill", action="append", default=[], help="skill id; repeatable")
    parser.add_argument("--option", action="append", default=[], metavar="SKILL:OPTION", help="selected option; repeatable")
    parser.add_argument(
        "--custom-structure",
        action="append",
        default=[],
        metavar="SKILL=PATH",
        help="custom UTF-8 structure rules for a selected skill; repeatable",
    )
    parser.add_argument("--project-dir", help="target project directory")
    parser.add_argument("--skills-dir", help="skills folder, relative to project or absolute")
    parser.add_argument("--agent-file", help="agent instructions file, relative to project or absolute")
    parser.add_argument("--agent", choices=["codex", "claude", "gemini", "generic"], help="target agent adapter")
    parser.add_argument(
        "--execution-state-policy",
        choices=sorted(POLICY_SCOPES),
        help="execution-state activation scope",
    )
    parser.add_argument(
        "--enforcement",
        choices=sorted(ENFORCEMENT_MODES),
        default="instructions",
        help="instruction-only or strict runtime enforcement",
    )
    parser.add_argument("--yes", action="store_true", help="accept replacement and non-interactive defaults")
    parser.add_argument("--dry-run", action="store_true", help="show resolved installation without writing")
    return parser.parse_args(argv)


def print_catalog(catalog: dict) -> None:
    for skill in catalog["skills"]:
        print(f"{skill['id']}: {skill['description']}")
        dependencies = [
            dependency if isinstance(dependency, str) else dependency["id"]
            for dependency in skill.get("requires_skills", [])
        ]
        if dependencies:
            print(f"  requires skills: {', '.join(dependencies)}")
        if skill.get("activation"):
            activation = skill["activation"]
            print(
                f"  activation: default={activation['default_scope']}; "
                f"supported={','.join(activation['supported_scopes'])}"
            )
        for option in skill.get("options", []):
            print(f"  - {option['id']} [{option['kind']}]: {option['description']}")


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    try:
        catalog = load_catalog()
        if args.list:
            print_catalog(catalog)
            return 0
        by_id = {skill["id"]: skill for skill in catalog["skills"]}
        if args.skill:
            unknown = set(args.skill) - set(by_id)
            if unknown:
                raise InstallError(f"Unknown skills: {', '.join(sorted(unknown))}")
            requested_skill_ids = list(dict.fromkeys(args.skill))
        else:
            requested_skill_ids = [
                skill["id"] for skill in prompt_selection("Доступные скилы", catalog["skills"])
            ]
        selected_skills, required_by = resolve_skill_dependencies(catalog, requested_skill_ids)

        requested: dict[str, set[str]] = {skill["id"]: set() for skill in selected_skills}
        requested_custom: dict[str, list[dict]] = {skill["id"]: [] for skill in selected_skills}
        for raw in args.option:
            if ":" not in raw:
                raise InstallError(f"Option must use SKILL:OPTION format: {raw}")
            skill_id, option_id = raw.split(":", 1)
            if skill_id not in requested:
                raise InstallError(f"Option provided for unselected skill: {skill_id}")
            requested[skill_id].add(option_id)
        for raw in args.custom_structure:
            if "=" not in raw:
                raise InstallError(f"Custom structure must use SKILL=PATH format: {raw}")
            skill_id, path = raw.split("=", 1)
            if skill_id not in requested_custom:
                raise InstallError(f"Custom structure provided for unselected skill: {skill_id}")
            skill = by_id[skill_id]
            supports_custom = any(
                group.get("id") == "structure" and group.get("allow_custom_file")
                for group in skill.get("selection_groups", [])
            )
            if not supports_custom:
                raise InstallError(f"Custom structure is not supported for {skill_id}")
            requested_custom[skill_id].append(custom_structure_option(path))

        resolved: list[tuple[dict, list[dict]]] = []
        for skill in selected_skills:
            choices = requested[skill["id"]]
            custom_options = requested_custom[skill["id"]]
            if skill["mode"] == "modular" and not choices and not custom_options:
                if args.yes:
                    raise InstallError(f"At least one --option is required for {skill['id']}")
                if skill.get("selection_groups"):
                    options = prompt_grouped_options(skill)
                else:
                    chosen = prompt_selection(f"Языки и фреймворки для {skill['name']}", skill["options"])
                    options = resolve_options(skill, {option["id"] for option in chosen})
            else:
                options = resolve_options(skill, choices) + custom_options
            validate_selection_groups(skill, options)
            resolved.append((skill, options))

        adapters = catalog["agent_adapters"]
        if args.agent:
            agent_id = args.agent
        elif args.yes:
            agent_id = "codex"
        else:
            adapter_items = [
                {"id": key, "name": value["name"], "description": value["description"]}
                for key, value in adapters.items()
            ]
            agent_id = prompt_selection("Целевой ИИ-агент", adapter_items, allow_all=False)[0]["id"]
        adapter = adapters[agent_id]

        project_raw = args.project_dir or (str(Path.cwd()) if args.yes else defaulted_input("Путь до проекта", str(Path.cwd())))
        project_dir = Path(project_raw).expanduser().resolve()
        if not project_dir.is_dir():
            raise InstallError(f"Project directory not found: {project_dir}")
        skills_raw = args.skills_dir or (
            adapter["skills_dir"]
            if args.yes
            else defaulted_input("Папка размещения skills", adapter["skills_dir"])
        )
        skills_value = Path(skills_raw).expanduser()
        skills_root = skills_value.resolve() if skills_value.is_absolute() else (project_dir / skills_value).resolve()
        agent_raw = args.agent_file or (
            adapter["agent_file"]
            if args.yes
            else defaulted_input("Путь до файла инструкций ИИ-агента", adapter["agent_file"])
        )
        agent_value = Path(agent_raw).expanduser()
        agent_file = agent_value.resolve() if agent_value.is_absolute() else (project_dir / agent_value).resolve()

        execution_policy = None
        installed_ids = {skill["id"] for skill, _ in resolved}
        if "execution-state" in installed_ids:
            execution_skill = by_id["execution-state"]
            if args.execution_state_policy:
                scope = args.execution_state_policy
            elif "execution-state" in requested_skill_ids:
                scope = execution_skill["activation"]["default_scope"]
            else:
                scope = "dependent_tasks"
            if scope not in set(execution_skill["activation"]["supported_scopes"]):
                raise InstallError(f"Unsupported execution-state scope: {scope}")
            if args.enforcement == "strict":
                if adapter.get("strict_enforcement") != "codex-hooks":
                    raise InstallError(f"Strict enforcement is not supported by agent adapter: {agent_id}")
                if scope != "all_tasks":
                    raise InstallError("Strict enforcement currently requires all_tasks scope")
            execution_policy = {
                "scope": scope,
                "routing": "adaptive",
                "enforcement": args.enforcement,
                "failure_mode": "block" if args.enforcement == "strict" else "report",
                "skill_path": os.path.relpath(
                    skills_root / "execution-state" / "SKILL.md", start=project_dir
                ),
            }
        elif args.execution_state_policy or args.enforcement == "strict":
            raise InstallError(
                "Execution-state policy/enforcement requires installing execution-state directly "
                "or through requires_skills"
            )

        print("\nБудут установлены:")
        for skill, options in resolved:
            profile = ", ".join(option["name"] for option in options) or "bundled"
            print(f"  - {skill['name']}: {profile}")
        print(f"Skills: {skills_root}")
        print(f"Agent file: {agent_file}")
        print(f"Agent adapter: {agent_id}")
        if execution_policy:
            print(
                "Execution State: "
                f"scope={execution_policy['scope']}, enforcement={execution_policy['enforcement']}"
            )
        if args.dry_run:
            print("Dry run: файлы не изменены.")
            return 0
        if not args.yes:
            confirmation = input("Продолжить? [y/N]: ").strip().lower()
            if confirmation not in {"y", "yes", "д", "да"}:
                print("Установка отменена.")
                return 0

        manifest = build_manifest(
            skills_root,
            resolved,
            agent_adapter=agent_id,
            required_by=required_by,
            execution_policy=execution_policy,
        )
        skills_block = render_agent_block(manifest, skills_root, agent_file)
        policy_block = render_execution_policy(manifest, skills_root, agent_file)
        agent_content = updated_agent_content(agent_file, skills_block, policy_block)
        strict_codex = (
            manifest.get("policies", {}).get("execution-state", {}).get("enforcement")
            == "strict"
        )
        backups = apply_installation(
            project_dir=project_dir,
            skills_root=skills_root,
            agent_file=agent_file,
            installed=resolved,
            manifest=manifest,
            agent_content=agent_content,
            strict_codex=strict_codex,
        )
        print("\nУстановка завершена.")
        for backup in backups:
            print(f"Резервная копия предыдущей версии: {backup}")
        if strict_codex:
            print("Codex strict hooks установлены в .codex/hooks.json; проверь и доверь их через /hooks.")
        return 0
    except (InstallError, OSError, json.JSONDecodeError) as error:
        print(f"Ошибка: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
