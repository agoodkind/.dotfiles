"""Orchestration for syncing local rules into Cursor cloud storage."""

from __future__ import annotations

import sys

from cli_config import build_sync_config
from cursor_api import (
    add_rule,
    delete_all_rules,
    get_cursor_auth_token,
    list_rules,
)
from file_rules import (
    collect_rule_files,
    format_rule_source,
    parse_mdc_content,
    resolve_rule_file,
    validate_rule_directories,
)
from sync_types import RuleRecord
from logging_utils import (
    configure_sync_logger,
    get_logger,
    log_message,
    log_verbose,
)


def print_header() -> None:
    """Print the sync header."""
    logger = get_logger()
    logger.info("Cursor rule sync start")


def build_rule_payload(title: str, body: str) -> str:
    """Build the API payload string for one local rule."""
    return f"{title}\n\n{body}"


def build_remote_rule_index(remote_rules: list[RuleRecord]) -> dict[str, RuleRecord]:
    """Index remote rules by title for fast verification lookup."""
    return {remote_rule.get("title", ""): remote_rule for remote_rule in remote_rules}


def sync_rules() -> None:
    """Upload local `.mdc` files to Cursor cloud and verify the result."""
    config = build_sync_config()
    logger = configure_sync_logger()

    print_header()

    if not config.cursor_db.exists():
        logger.error("Cursor database not found: %s", config.cursor_db)
        logger.error("Make sure Cursor is installed and you're logged in.")
        sys.exit(1)

    validate_rule_directories(config.rule_directories)
    log_message(f"API endpoint: {config.api_base}")

    log_message("Authenticating...")
    token = get_cursor_auth_token(config.cursor_db)
    if not token:
        log_message("Authentication failed. Make sure you're logged into Cursor.")
        sys.exit(1)
    log_message("Authentication successful")

    log_message("Clearing existing cloud rules...")
    delete_all_rules(
        token=token,
        api_base=config.api_base,
    )

    local_rule_files = collect_rule_files(config.rule_directories)
    if not local_rule_files:
        searched_directories = ", ".join(
            str(rule_directory) for rule_directory in config.rule_directories
        )
        log_message(f"No .mdc files found in: {searched_directories}")
        sys.exit(0)

    total_rules = len(local_rule_files)
    log_message(f"Uploading {total_rules} rule(s) to cloud...")

    attempted = 0
    succeeded = 0
    failed = 0
    for rule_file in local_rule_files:
        resolved_file = resolve_rule_file(rule_file)
        display_file = format_rule_source(rule_file)
        if not resolved_file.exists():
            continue

        title = resolved_file.stem
        body = parse_mdc_content(resolved_file.read_text())
        payload = build_rule_payload(title=title, body=body)
        attempted += 1

        log_message(f"[{attempted}/{total_rules}] uploading {title}")
        log_verbose(f"Source: {display_file}")
        log_verbose(f"Size: {len(payload)} bytes")

        success, response = add_rule(
            token=token,
            api_base=config.api_base,
            workspace_url=config.workspace_url,
            title=title,
            content=payload,
        )
        if success:
            succeeded += 1
            if response:
                log_verbose(f"Response: {response}")
            log_verbose("Status: uploaded")
        else:
            failed += 1
            log_message(f"Upload failed for {title}: {response}")

    log_message("Verifying uploaded rules...")
    remote_rules = list_rules(token, config.api_base)
    remote_by_title = build_remote_rule_index(remote_rules)

    verified = 0
    verify_failed = 0
    for rule_file in local_rule_files:
        resolved_file = resolve_rule_file(rule_file)
        if not resolved_file.exists():
            continue

        title = resolved_file.stem
        expected_content = build_rule_payload(
            title=title,
            body=parse_mdc_content(resolved_file.read_text()),
        )
        remote_rule = remote_by_title.get(title)
        if not remote_rule:
            log_message(f"Verification failed, missing rule: {title}")
            verify_failed += 1
            continue

        if remote_rule.get("content", "") != expected_content:
            log_message(f"Verification failed, content mismatch: {title}")
            log_verbose(
                (
                    f"Expected {len(expected_content)} bytes, "
                    f"got {len(remote_rule.get('content', ''))} bytes"
                ),
            )
            verify_failed += 1
            continue

        log_verbose(f"{title}: content verified")
        verified += 1

    if verify_failed == 0:
        log_message(f"Verification passed for {verified} rule(s)")
    else:
        log_message(f"Verification failed for {verify_failed} rule(s)")

    if failed == 0 and verify_failed == 0:
        log_message(f"Sync complete: {succeeded} rule(s) uploaded and verified")
    elif failed > 0:
        log_message(
            (
                f"Sync completed with errors: {succeeded}/{attempted} "
                f"succeeded, {failed} failed"
            )
        )
    else:
        log_message(
            (
                f"Sync completed but {verify_failed} rule(s) failed "
                f"content verification"
            )
        )
