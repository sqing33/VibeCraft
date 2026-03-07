#!/usr/bin/env python3
"""Repo Library search wrapper around the existing reference retrieval scripts."""

from __future__ import annotations

import shutil
from pathlib import Path
from typing import Any

from extract_cards import render_search_compatible_report
from helpers import (
    ENGINE_VERSION,
    EngineError,
    analyzer_script,
    build_corpus_key,
    copy_file,
    ensure_dir,
    log_event,
    now_iso,
    parse_repo_url,
    read_json,
    run_json_command,
    write_json,
)


def _is_empty_index_error(exc: EngineError) -> bool:
    details = exc.details or {}
    stderr = str(details.get("stderr") or "")
    stdout = str(details.get("stdout") or "")
    combined = f"{exc}\n{stderr}\n{stdout}".lower()
    return "no chunk extracted from current reports" in combined or "no index entries available after build" in combined


def sync_snapshot_to_corpus(
    *,
    storage_root: Path,
    report_path: Path,
    repo_url: str | None = None,
    repo_key: str | None = None,
    snapshot_id: str | None = None,
    snapshot_dir: Path | None = None,
    run_id: str | None = None,
    subagent_results_path: Path | None = None,
    cards_path: Path | None = None,
) -> dict[str, Any] | None:
    """Mirror one snapshot into the retrieval corpus layout expected by reference_retrieval.py."""

    if not snapshot_id or not report_path.exists():
        return None
    resolved_repo_key = repo_key
    if repo_url and not resolved_repo_key:
        resolved_repo_key = parse_repo_url(repo_url).repo_key
    if not resolved_repo_key:
        raise EngineError("repo_key or repo_url is required when syncing search corpus")

    corpus_root = ensure_dir(storage_root / "search" / "corpus")
    corpus_key = build_corpus_key(resolved_repo_key, snapshot_id)
    corpus_dir = corpus_root / corpus_key
    if corpus_dir.exists():
        shutil.rmtree(corpus_dir)
    ensure_dir(corpus_dir / "artifacts")

    normalized_report = render_search_compatible_report(report_path.read_text(encoding="utf-8", errors="ignore"))
    (corpus_dir / "report.md").write_text(normalized_report, encoding="utf-8")
    synced_subagent_path = None
    if subagent_results_path and subagent_results_path.exists():
        synced_subagent_path = str(copy_file(subagent_results_path, corpus_dir / "artifacts" / "subagent_results.json"))

    metadata = {
        "engine_version": ENGINE_VERSION,
        "corpus_key": corpus_key,
        "repo": {
            "repo_key": resolved_repo_key,
            "repo_url": repo_url,
            "path": None,
        },
        "snapshot": {
            "snapshot_id": snapshot_id,
            "path": str(snapshot_dir) if snapshot_dir else None,
            "report_path": str(report_path),
            "subagent_results_path": str(subagent_results_path) if subagent_results_path else None,
            "cards_path": str(cards_path) if cards_path else None,
        },
        "run": {
            "run_id": run_id,
        },
        "synced_at": now_iso(),
    }
    write_json(corpus_dir / "repo_library_snapshot.json", metadata)
    return {
        "corpus_key": corpus_key,
        "corpus_dir": str(corpus_dir),
        "metadata_path": str(corpus_dir / "repo_library_snapshot.json"),
        "subagent_results_path": synced_subagent_path,
    }


def load_corpus_metadata(corpus_root: Path) -> dict[str, dict[str, Any]]:
    """Load Repo Library metadata for every synced corpus directory."""

    results: dict[str, dict[str, Any]] = {}
    if not corpus_root.exists() or not corpus_root.is_dir():
        return results
    for candidate in sorted(corpus_root.iterdir(), key=lambda item: item.name):
        metadata_path = candidate / "repo_library_snapshot.json"
        if not metadata_path.exists() or not metadata_path.is_file():
            continue
        payload = read_json(metadata_path)
        if isinstance(payload, dict):
            results[candidate.name] = payload
    return results


def expand_repo_filters(corpus_meta: dict[str, dict[str, Any]], requested_filters: list[str]) -> list[str]:
    """Resolve repository-level filters into concrete corpus keys for retrieval."""

    if not requested_filters:
        return []
    normalized = {item.strip() for item in requested_filters if item.strip()}
    corpus_keys: list[str] = []
    for corpus_key, payload in corpus_meta.items():
        repo = payload.get("repo") if isinstance(payload, dict) else None
        snapshot = payload.get("snapshot") if isinstance(payload, dict) else None
        repo_key = repo.get("repo_key") if isinstance(repo, dict) else None
        snapshot_id = snapshot.get("snapshot_id") if isinstance(snapshot, dict) else None
        if corpus_key in normalized or repo_key in normalized or snapshot_id in normalized:
            corpus_keys.append(corpus_key)
    return sorted(set(corpus_keys))


def normalize_query_results(
    *,
    raw_hits: list[dict[str, Any]],
    corpus_meta: dict[str, dict[str, Any]],
    limit: int,
) -> list[dict[str, Any]]:
    """Map raw retrieval hits back to Repo Library repository and snapshot metadata."""

    results: list[dict[str, Any]] = []
    for hit in raw_hits:
        corpus_key = str(hit.get("repo") or "")
        meta = corpus_meta.get(corpus_key, {})
        repo_meta = meta.get("repo") if isinstance(meta, dict) else {}
        snapshot_meta = meta.get("snapshot") if isinstance(meta, dict) else {}
        run_meta = meta.get("run") if isinstance(meta, dict) else {}
        results.append(
            {
                "score": float(hit.get("score") or 0.0),
                "repository": repo_meta if isinstance(repo_meta, dict) else {},
                "snapshot": snapshot_meta if isinstance(snapshot_meta, dict) else {},
                "run": run_meta if isinstance(run_meta, dict) else {},
                "chunk": {
                    "chunk_id": hit.get("chunk_id"),
                    "source_file": hit.get("source_file"),
                    "section_type": hit.get("section_type"),
                    "section_title": hit.get("section_title"),
                },
                "evidence_refs": [str(item) for item in hit.get("evidence_refs", []) if str(item).strip()],
                "text_excerpt": hit.get("text_excerpt"),
                "text": hit.get("text"),
                "corpus_key": corpus_key,
            }
        )
        if len(results) >= limit:
            break
    return results


def run_search(
    *,
    storage_root: str,
    output_path: Path,
    refresh: str = "auto",
    query: str | None = None,
    repo_filters: list[str] | None = None,
    limit: int = 10,
    min_score: float = 0.35,
    mode: str = "semi",
    repo_url: str | None = None,
    repo_key: str | None = None,
    snapshot_id: str | None = None,
    snapshot_dir: Path | None = None,
    run_id: str | None = None,
    report_path: Path | None = None,
    subagent_results_path: Path | None = None,
    cards_path: Path | None = None,
) -> dict[str, Any]:
    """功能：构建或查询 Repo Library 向量索引，并返回归一化搜索结果。
    参数/返回：接收存储路径、刷新策略、查询条件与可选 snapshot 上下文，返回稳定 JSON 搜索摘要。
    失败场景：语义索引脚本执行失败、索引缺失或查询参数非法时抛出 EngineError。
    副作用：同步检索语料目录、调用参考检索脚本刷新索引、按需写入查询结果文件。
    """

    storage_root_path = Path(storage_root).resolve()
    search_root = ensure_dir(storage_root_path / "search")
    corpus_root = ensure_dir(search_root / "corpus")
    index_root = ensure_dir(search_root / "index")

    sync_summary = None
    if report_path:
        sync_summary = sync_snapshot_to_corpus(
            storage_root=storage_root_path,
            report_path=report_path,
            repo_url=repo_url,
            repo_key=repo_key,
            snapshot_id=snapshot_id,
            snapshot_dir=snapshot_dir,
            run_id=run_id,
            subagent_results_path=subagent_results_path,
            cards_path=cards_path,
        )

    refresh_summary = None
    wrapper_script = analyzer_script("reference_retrieval_uv.sh")
    if refresh in {"auto", "force"}:
        build_cmd = [
            "bash",
            str(wrapper_script),
            "build",
            "--storage-root",
            str(corpus_root),
            "--index-root",
            str(index_root),
        ]
        if refresh == "force":
            build_cmd.append("--force")
        log_event("INFO", "search-refresh", "刷新语义索引", refresh=refresh, storage_root=str(corpus_root))
        try:
            refresh_summary = run_json_command(build_cmd)
        except EngineError as exc:
            if _is_empty_index_error(exc):
                refresh_summary = {
                    "status": "empty",
                    "message": str(exc),
                    "details": exc.details,
                }
            else:
                raise

    corpus_meta = load_corpus_metadata(corpus_root)
    resolved_repo_filters = expand_repo_filters(corpus_meta, repo_filters or [])
    results: list[dict[str, Any]] = []

    if query and query.strip() and not (isinstance(refresh_summary, dict) and refresh_summary.get("status") == "empty"):
        query_cmd = [
            "bash",
            str(wrapper_script),
            "query",
            "--query",
            query.strip(),
            "--storage-root",
            str(corpus_root),
            "--index-root",
            str(index_root),
            "--format",
            "json",
            "--mode",
            mode,
            "--top-k",
            str(max(limit * 4, limit, 20)),
            "--min-score",
            str(min_score),
            "--refresh",
            "never",
        ]
        for item in resolved_repo_filters:
            query_cmd.extend(["--repo", item])
        try:
            raw_query = run_json_command(query_cmd)
            raw_hits = raw_query.get("hits", []) if isinstance(raw_query, dict) else []
            results = normalize_query_results(raw_hits=raw_hits, corpus_meta=corpus_meta, limit=limit)
        except EngineError as exc:
            if _is_empty_index_error(exc):
                results = []
                if not refresh_summary:
                    refresh_summary = {
                        "status": "empty",
                        "message": str(exc),
                        "details": exc.details,
                    }
            else:
                raise

    return {
        "status": "ok",
        "command": "search",
        "engine_version": ENGINE_VERSION,
        "generated_at": now_iso(),
        "storage_root": str(storage_root_path),
        "corpus_root": str(corpus_root),
        "index_root": str(index_root),
        "output": str(output_path),
        "query": query,
        "limit": limit,
        "refresh": refresh,
        "repo_filters_requested": repo_filters or [],
        "repo_filters_resolved": resolved_repo_filters,
        "sync": sync_summary,
        "search_refresh": refresh_summary,
        "result_count": len(results),
        "results": results,
    }
