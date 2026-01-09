#!/usr/bin/env python3
"""
Protobuf encoding/decoding for Cursor's KnowledgeBase API.

Usage:
    cursor-proto.py parse-ids          # Read protobuf from stdin, output space-separated IDs
    cursor-proto.py encode-string <field_num> <value>   # Encode single string field
    cursor-proto.py encode-add <content> <title> [url]  # Encode KnowledgeBaseAdd request
"""

import sys


def encode_varint(value: int) -> bytes:
    """Encode an integer as a protobuf varint."""
    result = []
    while value > 127:
        result.append((value & 0x7F) | 0x80)
        value >>= 7
    result.append(value)
    return bytes(result)


def read_varint(data: bytes, pos: int) -> tuple[int, int]:
    """Read a varint from data at position, return (value, new_position)."""
    result = 0
    shift = 0
    while pos < len(data):
        b = data[pos]
        pos += 1
        result |= (b & 0x7F) << shift
        if not (b & 0x80):
            break
        shift += 7
    return result, pos


def encode_string_field(field_num: int, value: str) -> bytes:
    """Encode a string as a protobuf field (wire type 2 = length-delimited)."""
    tag = (field_num << 3) | 2
    data = value.encode("utf-8")
    return bytes([tag]) + encode_varint(len(data)) + data


def parse_field(data: bytes, pos: int) -> tuple[int | None, any, int]:
    """Parse a single protobuf field, return (field_num, value, new_position)."""
    if pos >= len(data):
        return None, None, pos

    tag = data[pos]
    field_num = tag >> 3
    wire_type = tag & 0x07
    pos += 1

    if wire_type == 0:  # varint
        val, pos = read_varint(data, pos)
        return field_num, val, pos
    elif wire_type == 2:  # length-delimited
        length, pos = read_varint(data, pos)
        val = data[pos : pos + length]
        return field_num, val, pos + length
    else:
        # Skip unknown wire types
        return None, None, len(data)


def parse_rule_ids(data: bytes) -> list[str]:
    """Parse KnowledgeBaseList response, extract rule IDs."""
    ids = []
    i = 0

    # Parse top-level message: field 1 = flag, field 2 = repeated entries
    while i < len(data):
        field_num, val, i = parse_field(data, i)
        if field_num == 2 and isinstance(val, bytes):
            # Parse nested entry message - first field should be ID
            j = 0
            entry_field, entry_val, j = parse_field(val, j)
            if entry_field == 1 and isinstance(entry_val, bytes):
                try:
                    id_str = entry_val.decode("utf-8")
                    if id_str.isdigit():
                        ids.append(id_str)
                except (UnicodeDecodeError, ValueError):
                    pass

    return ids


def cmd_parse_ids():
    """Read protobuf from stdin, output space-separated rule IDs."""
    data = sys.stdin.buffer.read()
    ids = parse_rule_ids(data)
    print(" ".join(ids))


def cmd_encode_string(field_num: int, value: str):
    """Encode a single string field and write to stdout."""
    sys.stdout.buffer.write(encode_string_field(field_num, value))


def cmd_encode_add(content: str, title: str, url: str):
    """
    Encode KnowledgeBaseAdd request.

    Matches Cursor capture:
    - field 1: content
    - field 2: title (Cursor UI uses [Untitled])
    - field 3: url (optional)
    """
    msg = encode_string_field(1, content) + encode_string_field(2, title)
    if url:
        msg += encode_string_field(3, url)
    sys.stdout.buffer.write(msg)


def main():
    if len(sys.argv) < 2:
        print(__doc__, file=sys.stderr)
        sys.exit(1)

    cmd = sys.argv[1]

    if cmd == "parse-ids":
        cmd_parse_ids()
    elif cmd == "encode-string":
        if len(sys.argv) != 4:
            print("Usage: cursor-proto.py encode-string <field_num> <value>", file=sys.stderr)
            sys.exit(1)
        cmd_encode_string(int(sys.argv[2]), sys.argv[3])
    elif cmd == "encode-add":
        if len(sys.argv) not in (4, 5):
            print(
                "Usage: cursor-proto.py encode-add <content> <title> [url]",
                file=sys.stderr,
            )
            sys.exit(1)
        url = sys.argv[4] if len(sys.argv) == 5 else ""
        cmd_encode_add(sys.argv[2], sys.argv[3], url)
    else:
        print(f"Unknown command: {cmd}", file=sys.stderr)
        print(__doc__, file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
