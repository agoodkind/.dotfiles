"""Unified logger helpers for Cursor rule synchronization."""

from __future__ import annotations

import logging
import sys

LOGGER_NAME = "cursor_rules_sync"
LOG_MESSAGE_FORMAT = "%(levelname)s: %(message)s"


def get_logger() -> logging.Logger:
    """Return the shared logger."""
    return logging.getLogger(LOGGER_NAME)


def configure_sync_logger() -> logging.Logger:
    """Configure logger once for a sync run and return it."""
    logger = get_logger()
    logger.handlers.clear()
    logger.propagate = False

    logger.setLevel(logging.DEBUG)

    stream_handler = logging.StreamHandler(sys.stdout)
    stream_handler.setLevel(logging.INFO)
    stream_handler.setFormatter(logging.Formatter(LOG_MESSAGE_FORMAT))

    logger.addHandler(stream_handler)
    return logger


def log_message(message: str) -> None:
    """Log an informational message."""
    get_logger().info(message)


def log_verbose(message: str) -> None:
    """Log a debug-level message."""
    get_logger().debug(message)


def log_error(message: str) -> None:
    """Log an error message."""
    get_logger().error(message)


