"""Configuration discovery for the Cursor rules sync workflow."""

from __future__ import annotations

import os
from pathlib import Path

from constants import API_BASE, DEFAULT_RULES_DIR, DEFAULT_WORKSPACE_URL
from sync_types import SyncConfig


def split_rule_directories(raw_rule_dirs: str) -> list[Path]:
    """Split a colon-delimited directory list and ignore blanks."""
    directories: list[Path] = []
    for raw_rule_dir in raw_rule_dirs.split(":"):
        trimmed_rule_dir = raw_rule_dir.strip()
        if trimmed_rule_dir:
            directories.append(Path(trimmed_rule_dir))
    return directories


def load_rule_directories() -> list[Path]:
    """Build the ordered source rule directories from defaults and environment."""
    raw_default_rule_dir = os.environ.get("CURSOR_RULES_DIR", str(DEFAULT_RULES_DIR))
    default_directory = Path(raw_default_rule_dir)

    raw_extra_rule_dirs = os.environ.get("CURSOR_EXTRA_RULE_DIRS", "")
    extra_directories = split_rule_directories(raw_extra_rule_dirs)

    return [default_directory, *extra_directories]


def build_sync_config() -> SyncConfig:
    """Build runtime configuration for the sync command."""
    workspace_url = os.environ.get("DEFAULT_RULE_URL", DEFAULT_WORKSPACE_URL)
    rule_directories = load_rule_directories()
    cursor_db = (
        Path.home()
        / "Library/Application Support/Cursor/User/globalStorage/state.vscdb"
    )
    return SyncConfig(
        cursor_db=cursor_db,
        api_base=API_BASE,
        workspace_url=workspace_url,
        rule_directories=rule_directories,
    )
