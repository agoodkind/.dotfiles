#!/usr/bin/env python3
"""Sync .cursor/rules MDC files to Cursor cloud User Rules."""
from __future__ import annotations

from sync_runner import sync_rules


if __name__ == "__main__":
    sync_rules()
