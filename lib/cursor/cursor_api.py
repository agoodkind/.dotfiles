"""Cursor API transport and rule serialization logic."""

from __future__ import annotations

import concurrent.futures
import sqlite3
import subprocess
from pathlib import Path

from constants import (
    CURSOR_AUTH_HEADER,
    CURSOR_AUTH_TOKEN_QUERY,
    CONTENT_TYPE_HEADER,
    CONNECT_PROTOCOL_HEADER,
    ENDPOINT_ADD,
    ENDPOINT_LIST,
    ENDPOINT_REMOVE,
    FIELD_ADD_CONTENT,
    FIELD_ADD_TITLE,
    FIELD_ADD_WORKSPACE_URL,
    FIELD_REMOVE_RULE_ID,
    FIELD_RULE_ID,
    FIELD_RULES_LIST,
    FIELD_RULE_CONTENT,
    FIELD_RULE_TIMESTAMP,
    FIELD_RULE_TITLE,
    MAX_PARALLEL_WORKERS,
)
from logging_utils import log_error, log_message, log_verbose
from protobuf_codec import (
    decode_bytes_field,
    encode_bytes_field,
    get_bytes_field,
    parse_message,
)
from sync_types import CurlResult, ParsedMessage, RuleRecord


def call_cursor_api(
    token: str,
    api_base: str,
    endpoint: str,
    payload: bytes,
) -> bytes:
    """Call a Cursor API endpoint and return the raw response bytes."""
    command = [
        "curl",
        "-s",
        "-X",
        "POST",
        f"{api_base}/{endpoint}",
        "-H",
        f"{CURSOR_AUTH_HEADER} {token}",
        "-H",
        CONTENT_TYPE_HEADER,
        "-H",
        CONNECT_PROTOCOL_HEADER,
        "--data-binary",
        "@-",
    ]
    result = subprocess.run(
        command,
        input=payload,
        capture_output=True,
    )
    return result.stdout


def call_cursor_api_with_status(
    token: str,
    api_base: str,
    endpoint: str,
    payload: bytes,
) -> CurlResult:
    """Call an endpoint and return `(http_code, response_body)`."""
    command = [
        "curl",
        "-s",
        "-w",
        "%{http_code}",
        "-X",
        "POST",
        f"{api_base}/{endpoint}",
        "-H",
        f"{CURSOR_AUTH_HEADER} {token}",
        "-H",
        CONTENT_TYPE_HEADER,
        "-H",
        CONNECT_PROTOCOL_HEADER,
        "--data-binary",
        "@-",
    ]
    result = subprocess.run(
        command,
        input=payload,
        capture_output=True,
    )
    output = result.stdout.decode("utf-8", errors="ignore")
    if len(output) >= 3:
        status_code = int(output[-3:])
        body = output[:-3]
    else:
        status_code = 0
        body = output
    return status_code, body


def get_cursor_auth_token(cursor_db: Path) -> str:
    """Load Cursor auth token from its SQLite database."""
    with sqlite3.connect(cursor_db) as connection:
        cursor = connection.execute(CURSOR_AUTH_TOKEN_QUERY)
        row = cursor.fetchone()
        if row is None:
            return ""
        return str(row[0].strip('"'))


def build_add_rule_payload(content: str, title: str, workspace_url: str) -> bytes:
    """Build a `KnowledgeBaseAdd` payload."""
    payload = b""
    payload += encode_bytes_field(FIELD_ADD_CONTENT, content)
    payload += encode_bytes_field(FIELD_ADD_TITLE, title)
    payload += encode_bytes_field(FIELD_ADD_WORKSPACE_URL, workspace_url)
    return payload


def list_rules(token: str, api_base: str) -> list[RuleRecord]:
    """Fetch existing Cursor rule records from the remote service."""
    response = call_cursor_api(token, api_base, ENDPOINT_LIST, b"")

    if not response:
        return []

    parsed_response = parse_message(response)
    raw_rule_messages = collect_raw_rule_messages(parsed_response)

    records: list[RuleRecord] = []

    for raw_rule_message in raw_rule_messages:
        parsed_entry = parse_message(raw_rule_message)
        rule_record = build_rule_record(parsed_entry)

        if rule_record["id"]:
            records.append(rule_record)

    return records


def list_rule_ids(token: str, api_base: str) -> list[str]:
    """Fetch remote rule IDs only."""
    ids: list[str] = []
    for rule in list_rules(token, api_base):
        rule_id = rule.get("id", "")
        if rule_id:
            ids.append(rule_id)
    return ids


def remove_rule(token: str, api_base: str, rule_id: str) -> bytes:
    """Delete one remote rule by ID."""
    payload = encode_bytes_field(FIELD_REMOVE_RULE_ID, rule_id)
    return call_cursor_api(token, api_base, ENDPOINT_REMOVE, payload)


def build_rule_record(parsed_entry: ParsedMessage) -> RuleRecord:
    """Build a typed rule record from parsed protobuf fields."""
    return {
        "id": decode_bytes_field(get_bytes_field(parsed_entry, FIELD_RULE_ID)),
        "content": decode_bytes_field(get_bytes_field(parsed_entry, FIELD_RULE_CONTENT)),
        "title": decode_bytes_field(get_bytes_field(parsed_entry, FIELD_RULE_TITLE)),
        "timestamp": decode_bytes_field(
            get_bytes_field(parsed_entry, FIELD_RULE_TIMESTAMP)
        ),
    }


def collect_raw_rule_messages(
    parsed_response: ParsedMessage,
) -> list[bytes]:
    """Normalize nested protobuf field data into a list of message bytes."""
    raw_rules = parsed_response.get(FIELD_RULES_LIST, [])

    if isinstance(raw_rules, bytes):
        return [raw_rules]

    if not isinstance(raw_rules, list):
        return []

    raw_rule_messages: list[bytes] = []
    for raw_rule in raw_rules:
        if isinstance(raw_rule, bytes):
            raw_rule_messages.append(raw_rule)
    return raw_rule_messages


def add_rule(
    token: str,
    api_base: str,
    workspace_url: str,
    title: str,
    content: str,
) -> tuple[bool, str]:
    """Upload one rule and return `(succeeded, response_body)`."""
    payload = build_add_rule_payload(content, title, workspace_url)
    status_code, response_body = call_cursor_api_with_status(
        token,
        api_base,
        ENDPOINT_ADD,
        payload,
    )
    return 200 <= status_code < 300, response_body


def delete_all_rules(
    token: str,
    api_base: str,
) -> int:
    """Remove all remote rules before re-syncing local files."""
    existing_rule_ids = list_rule_ids(token, api_base)
    if not existing_rule_ids:
        log_message("No existing cloud rules found")
        return 0

    total = len(existing_rule_ids)
    log_message(f"Removing {total} existing cloud rule(s)")

    removal_errors: list[str] = []

    with concurrent.futures.ThreadPoolExecutor(
        max_workers=MAX_PARALLEL_WORKERS,
    ) as executor:
        futures = [
            executor.submit(remove_rule, token, api_base, rule_id)
            for rule_id in existing_rule_ids
        ]
        for index, completion in enumerate(
            concurrent.futures.as_completed(futures),
            start=1,
        ):
            try:
                completion.result()
                log_verbose(f"Removed rule {index}/{total}")
            except Exception as removal_error:
                removal_errors.append(str(removal_error))
                log_error(f"Rule remove failed: {removal_error}")

    remaining_rule_count = len(list_rule_ids(token, api_base))
    if remaining_rule_count > 0:
        log_message(
            f"{remaining_rule_count} rule(s) still present after deletion."
            f" The API may have rejected some removes.",
        )
    else:
        log_message(f"Removed {total} cloud rule(s)")

    if removal_errors:
        return total - len(removal_errors)

    return total
