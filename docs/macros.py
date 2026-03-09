"""MkDocs macros that expose the DeclaREST docs version."""

from __future__ import annotations

import os
import subprocess
from typing import Any


DEFAULT_VERSION = "dev"


def define_env(env: Any) -> None:
    env.macro("declarest_version", declarest_version)
    env.macro("declarest_tag", declarest_tag)


def declarest_version() -> str:
    value = (
        os.getenv("DECLAREST_DOCS_VERSION", "").strip()
        or os.getenv("GITHUB_REF_NAME", "").strip()
        or _latest_release_tag()
        or DEFAULT_VERSION
    )
    if value.startswith("v"):
        return value[1:]
    return value


def declarest_tag() -> str:
    version = declarest_version()
    if version == DEFAULT_VERSION:
        return version
    return f"v{version}"


def _latest_release_tag() -> str:
    try:
        completed = subprocess.run(
            ["git", "tag", "--points-at", "HEAD"],
            check=True,
            capture_output=True,
            text=True,
        )
    except (OSError, subprocess.CalledProcessError):
        return ""
    for line in completed.stdout.splitlines():
        value = line.strip()
        if value.startswith("v"):
            return value
    return ""
