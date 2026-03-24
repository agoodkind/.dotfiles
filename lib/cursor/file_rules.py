"""Filesystem helpers for local `.mdc` rule discovery."""

from __future__ import annotations

import operator
from pathlib import Path

from logging_utils import log_message


def parse_mdc_content(raw_content: str) -> str:
    """Remove optional YAML front matter and surrounding empty lines."""
    if raw_content.startswith("---"):
        separator_index = raw_content.find("\n---", 3)
        if separator_index != -1:
            return raw_content[separator_index + 4 :].lstrip("\n").strip()
    return raw_content.strip()


def collect_rule_files(rule_directories: list[Path]) -> list[Path]:
    """Collect `.mdc` files from all configured directories."""
    selected_by_name: dict[str, Path] = {}

    for rule_directory in rule_directories:
        if not rule_directory.exists():
            continue

        for rule_file in sorted(rule_directory.glob("*.mdc")):
            selected_by_name[rule_file.stem] = rule_file

    return sorted(selected_by_name.values(), key=operator.attrgetter("stem"))


def resolve_rule_file(rule_file: Path) -> Path:
    """Return the target of a symlink or the original file."""
    if rule_file.is_symlink():
        return rule_file.resolve()
    return rule_file


def format_rule_source(rule_file: Path) -> str:
    """Return display text for a rule path, including symlink targets."""
    if rule_file.is_symlink():
        return f"{rule_file} -> {rule_file.resolve()}"
    return str(rule_file)


def validate_rule_directories(
    rule_directories: list[Path],
) -> None:
    """Log available directories and exit if none exist."""
    existing_directories: list[Path] = []

    for rule_directory in rule_directories:
        if rule_directory.exists():
            existing_directories.append(rule_directory)

    if not existing_directories:
        joined_directories = ", ".join(
            str(rule_directory) for rule_directory in rule_directories
        )
        log_message(f"No rules directories found. Checked: {joined_directories}")
        raise SystemExit(1)

    for rule_directory in rule_directories:
        status = "present" if rule_directory.exists() else "missing"
        log_message(f"Rules directory: {rule_directory} ({status})")
