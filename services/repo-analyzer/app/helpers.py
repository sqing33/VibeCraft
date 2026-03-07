#!/usr/bin/env python3
"""Shared helpers for the Repo Library Python engine."""

from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

ENGINE_VERSION = "1"


class EngineError(RuntimeError):
    """Raised when the Repo Library engine cannot complete a command."""

    def __init__(self, message: str, *, details: dict[str, Any] | None = None) -> None:
        super().__init__(message)
        self.details = details or {}


@dataclass(frozen=True)
class RepoRef:
    """Normalized GitHub repository identity."""

    repo_url: str
    owner: str
    repo: str
    repo_key: str


def now_iso() -> str:
    """Return the current UTC timestamp as ISO-8601 text."""

    return datetime.now(timezone.utc).isoformat()


def discover_repo_root() -> Path:
    """Find the repository root so sibling scripts can be located reliably."""

    start = Path(__file__).resolve()
    for candidate in [start.parent, *start.parents]:
        if (candidate / ".git").exists() or (candidate / "PROJECT_STRUCTURE.md").exists():
            return candidate
    return Path.cwd().resolve()


def slugify(value: str) -> str:
    """Convert arbitrary text into a filesystem-safe slug."""

    slug = re.sub(r"[^A-Za-z0-9._-]+", "-", value.strip())
    slug = re.sub(r"-+", "-", slug).strip("-")
    return slug or "unknown"


def parse_repo_url(repo_url: str) -> RepoRef:
    """Parse supported GitHub URLs into a stable repository identity."""

    raw = repo_url.strip()
    if not raw:
        raise EngineError("repo_url is required")

    if raw.startswith("git@github.com:"):
        path_part = raw.split(":", 1)[1]
    else:
        parsed = urlparse(raw)
        if parsed.scheme not in {"http", "https"}:
            raise EngineError("repo_url must use http(s) or git@github.com format")
        if parsed.netloc.lower() != "github.com":
            raise EngineError("only github.com repositories are supported")
        path_part = parsed.path.lstrip("/")

    if path_part.endswith(".git"):
        path_part = path_part[:-4]

    segments = [segment for segment in path_part.split("/") if segment]
    if len(segments) < 2:
        raise EngineError("repo_url must include owner and repo segments")

    owner, repo = segments[0], segments[1]
    if not re.fullmatch(r"[A-Za-z0-9_.-]+", owner):
        raise EngineError(f"invalid repository owner: {owner}")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+", repo):
        raise EngineError(f"invalid repository name: {repo}")

    return RepoRef(repo_url=repo_url, owner=owner, repo=repo, repo_key=slugify(f"{owner}-{repo}"))


def normalize_run_id(run_id: str | None) -> str:
    """Return a stable run id, generating one when the caller omits it."""

    if run_id and run_id.strip():
        return slugify(run_id)
    return f"run-{datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')}"


def make_snapshot_id(commit_sha: str | None, resolved_ref: str | None) -> str:
    """Build a stable snapshot id from commit SHA or fallback ref text."""

    sha = (commit_sha or "").strip()
    if sha:
        return f"sha-{sha[:12]}"
    ref = slugify((resolved_ref or "unknown").strip() or "unknown")
    return f"ref-{ref}"


def build_corpus_key(repo_key: str, snapshot_id: str) -> str:
    """Return the search-corpus directory name for one repository snapshot."""

    return f"{repo_key}--{snapshot_id}"


def ensure_dir(path: Path) -> Path:
    """Create a directory and return the normalized path."""

    path.mkdir(parents=True, exist_ok=True)
    return path


def reset_dir(path: Path) -> Path:
    """Recreate a directory from scratch to avoid stale artifacts."""

    if path.exists():
        shutil.rmtree(path)
    path.mkdir(parents=True, exist_ok=True)
    return path


def replace_dir(src: Path, dst: Path) -> Path:
    """Move a prepared directory into place, replacing any existing destination."""

    if not src.exists() or not src.is_dir():
        raise EngineError(f"source directory does not exist: {src}")
    if dst.exists():
        shutil.rmtree(dst)
    dst.parent.mkdir(parents=True, exist_ok=True)
    shutil.move(str(src), str(dst))
    return dst


def copy_file(src: Path, dst: Path) -> Path:
    """Copy a file while ensuring the destination parent directory exists."""

    if not src.exists() or not src.is_file():
        raise EngineError(f"file does not exist: {src}")
    dst.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(src, dst)
    return dst


def read_json(path: Path) -> Any:
    """Load JSON from disk."""

    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, payload: Any) -> Path:
    """Write UTF-8 JSON with a trailing newline."""

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return path


def log_event(level: str, action: str, msg: str, **fields: Any) -> None:
    """Emit a compact structured log line to stderr for long-running runs."""

    parts = [
        f"level={level}",
        "module=repo-analyzer",
        f"action={action}",
        f'msg={json.dumps(msg, ensure_ascii=False)}',
    ]
    for key, value in sorted(fields.items()):
        if value is None:
            continue
        if isinstance(value, (int, float)):
            parts.append(f"{key}={value}")
            continue
        text = str(value)
        if not text:
            continue
        if re.fullmatch(r"[A-Za-z0-9._:/@+-]+", text):
            parts.append(f"{key}={text}")
        else:
            parts.append(f"{key}={json.dumps(text, ensure_ascii=False)}")
    print(" ".join(parts), file=sys.stderr)


def _extract_json_text(raw: str) -> Any:
    text = raw.strip()
    if not text:
        raise EngineError("command did not produce JSON output")

    try:
        return json.loads(text)
    except json.JSONDecodeError:
        pass

    candidates = [index for index, char in enumerate(text) if char in "[{"]
    for index in candidates:
        try:
            return json.loads(text[index:])
        except json.JSONDecodeError:
            continue
    raise EngineError("command output is not valid JSON", details={"stdout": text[-4000:]})


def run_command(cmd: list[str], *, cwd: Path | None = None, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
    """Execute a subprocess and return the completed process."""

    command_text = " ".join(cmd)
    log_event("INFO", "spawn", "启动外部脚本", command=command_text)
    completed = subprocess.run(
        cmd,
        cwd=str(cwd) if cwd else None,
        env=env or os.environ.copy(),
        text=True,
        capture_output=True,
        check=False,
    )
    if completed.stderr.strip():
        for line in completed.stderr.strip().splitlines()[-20:]:
            print(line, file=sys.stderr)
    return completed


def run_json_command(cmd: list[str], *, cwd: Path | None = None, env: dict[str, str] | None = None) -> Any:
    """Execute a subprocess that is expected to return JSON on stdout."""

    completed = run_command(cmd, cwd=cwd, env=env)
    if completed.returncode != 0:
        raise EngineError(
            "external command failed",
            details={
                "command": cmd,
                "returncode": completed.returncode,
                "stdout": completed.stdout[-4000:],
                "stderr": completed.stderr[-4000:],
            },
        )
    return _extract_json_text(completed.stdout)


def analyzer_script(name: str) -> Path:
    """Locate one reusable script under github-feature-analyzer."""

    path = discover_repo_root() / ".codex" / "skills" / "github-feature-analyzer" / "scripts" / name
    if not path.exists():
        raise EngineError(f"analyzer script not found: {path}")
    return path


def write_result(output: Path, payload: dict[str, Any]) -> None:
    """Persist the final command payload and mirror it to stdout."""

    write_json(output, payload)
    print(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True))


def repo_storage_paths(storage_root: Path, repo_key: str, run_id: str) -> dict[str, Path]:
    """Return the standard Repo Library storage layout for one repository run."""

    repositories_root = storage_root / "repositories"
    repo_dir = repositories_root / repo_key
    runs_dir = repo_dir / "runs"
    run_dir = runs_dir / run_id
    snapshots_dir = repo_dir / "snapshots"
    return {
        "storage_root": storage_root,
        "repositories_root": repositories_root,
        "repo_dir": repo_dir,
        "repo_meta": repo_dir / "repo.json",
        "runs_dir": runs_dir,
        "run_dir": run_dir,
        "run_meta": run_dir / "run.json",
        "run_workspace": run_dir / "workspace",
        "run_report": run_dir / "report.md",
        "snapshots_dir": snapshots_dir,
        "search_root": storage_root / "search",
        "search_corpus_root": storage_root / "search" / "corpus",
        "search_index_root": storage_root / "search" / "index",
    }


def summarize_paths(repo_dir: Path, snapshot_dir: Path, run_dir: Path, report_path: Path) -> dict[str, str]:
    """Return top-level path aliases for stable backend consumption."""

    return {
        "repo_path": str(repo_dir),
        "snapshot_path": str(snapshot_dir),
        "run_path": str(run_dir),
        "report_path": str(report_path),
    }
