#!/usr/bin/env python3
from __future__ import annotations

import json
import re
import sys
from difflib import SequenceMatcher
from typing import Union

JsonScalar = Union[str, int, float, bool, None]
JsonValue = Union[JsonScalar, list["JsonValue"], dict[str, "JsonValue"]]

MINIMUM_WORDS = 10
SEQUENCE_THRESHOLD = 0.94
TOKEN_THRESHOLD = 0.90


def prose_units(text: str) -> list[str]:
    without_code = re.sub(r"```.*?```", " ", text, flags=re.DOTALL)
    units = re.split(r"(?<=[.!?])\s+|\n+", without_code)
    return [unit.strip() for unit in units if unit.strip()]


def normalized_words(text: str) -> list[str]:
    without_links = re.sub(r"https?://\S+", " ", text)
    return re.findall(r"[a-z0-9]+", without_links.lower())


def repeats_prior(unit: str, prior_units: list[str]) -> bool:
    words = normalized_words(unit)
    if len(words) < MINIMUM_WORDS:
        return False
    normalized = " ".join(words)
    word_set = set(words)
    for prior in prior_units:
        prior_words = normalized_words(prior)
        if len(prior_words) < MINIMUM_WORDS:
            continue
        prior_normalized = " ".join(prior_words)
        if normalized == prior_normalized:
            return True
        shorter_length = min(len(words), len(prior_words))
        longer_length = max(len(words), len(prior_words))
        length_ratio = shorter_length / longer_length
        if length_ratio < TOKEN_THRESHOLD:
            continue
        prior_word_set = set(prior_words)
        union = word_set | prior_word_set
        if not union:
            continue
        token_ratio = len(word_set & prior_word_set) / len(union)
        sequence_ratio = SequenceMatcher(None, normalized, prior_normalized).ratio()
        if sequence_ratio >= SEQUENCE_THRESHOLD:
            return True
        if token_ratio >= TOKEN_THRESHOLD and sequence_ratio >= TOKEN_THRESHOLD:
            return True
    return False


def matched_values(payload: dict[str, JsonValue]) -> list[str]:
    matched = payload.get("matched")
    if not isinstance(matched, list):
        return []
    values: list[str] = []
    seen: set[str] = set()
    for item in matched:
        if not isinstance(item, dict):
            continue
        value = item.get("value")
        if isinstance(value, str) and value not in seen:
            seen.add(value)
            values.append(value)
    return values


def main() -> int:
    payload_value = json.load(sys.stdin)
    if not isinstance(payload_value, dict):
        return 1
    prior_units: list[str] = []
    for text in matched_values(payload_value):
        for unit in prose_units(text):
            if repeats_prior(unit, prior_units):
                print(
                    "State each fact once. Remove the repeated sentence or "
                    "near-duplicate restatement."
                )
                return 1
            prior_units.append(unit)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
