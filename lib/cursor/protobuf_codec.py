"""Minimal protobuf helpers used by Cursor's rule sync endpoints."""

from __future__ import annotations

from constants import (
    VARINT_CONTINUATION_BIT,
    VARINT_DATA_MASK,
    WIRE_TYPE_LENGTH_DELIMITED,
    WIRE_TYPE_MASK,
    WIRE_TYPE_VARINT,
)
from sync_types import FieldValue, ParsedField, ParsedMessage


def encode_varint(value: int) -> bytes:
    """Encode an integer into a protobuf varint byte sequence."""
    encoded_bytes: list[int] = []
    remaining_value = value

    while remaining_value > VARINT_DATA_MASK:
        encoded_bytes.append((remaining_value & VARINT_DATA_MASK) | VARINT_CONTINUATION_BIT)
        remaining_value >>= 7

    encoded_bytes.append(remaining_value & VARINT_DATA_MASK)
    return bytes(encoded_bytes)


def read_varint(data: bytes, position: int) -> tuple[int, int]:
    """Read one varint from a byte stream."""
    decoded_value = 0
    shift_count = 0
    cursor_position = position
    while cursor_position < len(data):
        current_byte = data[cursor_position]
        cursor_position += 1
        decoded_value |= (current_byte & VARINT_DATA_MASK) << shift_count
        if not (current_byte & VARINT_CONTINUATION_BIT):
            break
        shift_count += 7
    return decoded_value, cursor_position


def encode_bytes_field(field_number: int, value: str | bytes) -> bytes:
    """Encode a length-delimited field value."""
    payload_bytes = value if isinstance(value, bytes) else value.encode("utf-8")
    field_tag = (field_number << 3) | WIRE_TYPE_LENGTH_DELIMITED
    return bytes([field_tag]) + encode_varint(len(payload_bytes)) + payload_bytes


def parse_field(data: bytes, position: int) -> ParsedField:
    """Parse one protobuf field starting at the provided position."""
    if position >= len(data):
        return ParsedField(None, None, None, position)

    tag, next_position = read_varint(data, position)
    field_number = tag >> 3
    wire_type = tag & WIRE_TYPE_MASK

    if wire_type == WIRE_TYPE_VARINT:
        integer_value, end_position = read_varint(data, next_position)
        return ParsedField(field_number, wire_type, integer_value, end_position)

    if wire_type == WIRE_TYPE_LENGTH_DELIMITED:
        payload_length, end_position = read_varint(data, next_position)
        payload_start = end_position
        payload_end = payload_start + payload_length
        bytes_value = data[payload_start:payload_end]
        return ParsedField(field_number, wire_type, bytes_value, payload_end)

    return ParsedField(None, None, None, len(data))


def add_field_value(fields: ParsedMessage, field_number: int, value: int | bytes) -> None:
    """Store a parsed value for one field number, preserving repeated values."""
    if field_number not in fields:
        fields[field_number] = value
        return

    current_value = fields[field_number]
    if isinstance(current_value, list):
        fields[field_number] = [*current_value, value]
    else:
        fields[field_number] = [current_value, value]


def parse_message(data: bytes) -> ParsedMessage:
    """Parse a protobuf message into a field number to value map."""
    fields: ParsedMessage = {}
    cursor_position = 0
    while cursor_position < len(data):
        field = parse_field(data, cursor_position)
        if field.field_number is None or field.value is None:
            break
        add_field_value(fields, field.field_number, field.value)
        cursor_position = field.next_position
    return fields


def get_bytes_field(fields: ParsedMessage, field_number: int) -> bytes | None:
    """Return a bytes field value if present and typed as bytes."""
    field_value = fields.get(field_number)
    if isinstance(field_value, bytes):
        return field_value
    return None


def decode_bytes_field(value: bytes | None, default_value: str = "") -> str:
    """Decode bytes from a protobuf field into a string."""
    if value is None:
        return default_value
    return value.decode("utf-8")
