"""Typed data models used by the Cursor rules sync modules."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path


type CurlResult = tuple[int, str]
type FieldValue = int | bytes | list[int | bytes]
type ParsedMessage = dict[int, FieldValue]
type RuleRecord = dict[str, str]


@dataclass(frozen=True)
class SyncConfig:
    """Runtime options loaded from environment and execution context."""

    cursor_db: Path
    api_base: str
    workspace_url: str
    rule_directories: list[Path]


@dataclass(frozen=True)
class ParsedField:
    """Result of reading one protobuf field."""

    field_number: int | None
    wire_type: int | None
    value: int | bytes | None
    next_position: int
