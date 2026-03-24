"""Constants shared by the Cursor rules synchronization tooling."""

from __future__ import annotations

from pathlib import Path


API_BASE = "https://api2.cursor.sh/aiserver.v1.AiService"
DEFAULT_RULES_DIR = Path.home() / ".dotfiles/.cursor/rules"
DEFAULT_WORKSPACE_URL = "https://github.com/agoodkind/.dotfiles"

CURSOR_AUTH_HEADER = "authorization: Bearer"
CURSOR_AUTH_TOKEN_QUERY = (
    "SELECT value FROM ItemTable WHERE key = 'cursorAuth/accessToken'"
)
CONTENT_TYPE_HEADER = "content-type: application/proto"
CONNECT_PROTOCOL_HEADER = "connect-protocol-version: 1"

ENDPOINT_LIST = "KnowledgeBaseList"
ENDPOINT_REMOVE = "KnowledgeBaseRemove"
ENDPOINT_ADD = "KnowledgeBaseAdd"

FIELD_RULES_LIST = 2
FIELD_RULE_ID = 1
FIELD_RULE_CONTENT = 2
FIELD_RULE_TITLE = 3
FIELD_RULE_TIMESTAMP = 4

FIELD_ADD_CONTENT = 1
FIELD_ADD_TITLE = 2
FIELD_ADD_WORKSPACE_URL = 3
FIELD_REMOVE_RULE_ID = 1

VARINT_CONTINUATION_BIT = 0x80
VARINT_DATA_MASK = 0x7F
WIRE_TYPE_MASK = 0x07
WIRE_TYPE_VARINT = 0
WIRE_TYPE_LENGTH_DELIMITED = 2

MAX_PARALLEL_WORKERS = 10
