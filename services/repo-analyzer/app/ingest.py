#!/usr/bin/env python3
"""Repo Library ingest flow backed by existing analyzer scripts."""

from __future__ import annotations

import shutil
import sys
from pathlib import Path
from typing import Any

from helpers import (
    ENGINE_VERSION,
    EngineError,
    analyzer_script,
    copy_file,
    ensure_dir,
    log_event,
    make_snapshot_id,
    normalize_run_id,
    now_iso,
    parse_repo_url,
    repo_storage_paths,
    replace_dir,
    reset_dir,
    run_json_command,
    summarize_paths,
    write_json,
)


def _copy_optional(src: Path | None, dst: Path) -> str | None:
    if src is None:
        if dst.exists():
            dst.unlink()
        return None
    copy_file(src, dst)
    return str(dst)


def _run_prepare_flow(
    *,
    command: str,
    repo_url: str,
    ref: str,
    storage_root: str,
    run_id: str | None,
    snapshot_dir: str | None = None,
    subagent_results: str | None = None,
    features: list[str] | None = None,
    render_report: bool,
    language: str = "zh",
    depth: str = "standard",
    fetch_mode: str = "mcp-first",
    timeout: int = 60,
) -> dict[str, Any]:
    """功能：执行 Repo Library prepare/ingest 共享链路，准备 snapshot、代码索引与可选报告。
    参数/返回：接收仓库地址、存储根目录、运行 ID 与报告开关，返回稳定 JSON 元数据。
    失败场景：抓仓、建索引、渲染报告或路径落盘失败时抛出 EngineError。
    副作用：创建存储目录、下载仓库源码、写入索引与可选报告、更新 repo/snapshot/run 元数据。
    """

    if render_report and not features:
        raise EngineError("at least one --feature is required for ingest")

    feature_list = [item for item in (features or []) if item]

    started_at = now_iso()
    repo_ref = parse_repo_url(repo_url)
    storage_root_path = Path(storage_root).resolve()
    run_key = normalize_run_id(run_id)
    paths = repo_storage_paths(storage_root_path, repo_ref.repo_key, run_key)

    for key in [
        "storage_root",
        "repositories_root",
        "repo_dir",
        "runs_dir",
        "run_dir",
        "run_workspace",
        "snapshots_dir",
        "search_root",
        "search_corpus_root",
        "search_index_root",
    ]:
        ensure_dir(paths[key])

    requested_snapshot_dir = Path(snapshot_dir) if snapshot_dir else None
    if requested_snapshot_dir and not requested_snapshot_dir.is_absolute():
        raise EngineError("snapshot_dir must be an absolute path")

    staged_snapshot_dir = requested_snapshot_dir.resolve() if requested_snapshot_dir else None
    stage_source_dir = reset_dir((staged_snapshot_dir / "source") if staged_snapshot_dir else (paths["run_workspace"] / "source"))
    subagent_src = Path(subagent_results).resolve() if subagent_results else None
    if subagent_src and (not subagent_src.exists() or not subagent_src.is_file()):
        raise EngineError(f"subagent_results does not exist: {subagent_src}")

    start_action = f"{command}-start"
    start_message = "开始执行仓库入库" if render_report else "开始准备仓库快照"
    log_event("INFO", start_action, start_message, repo_url=repo_url, ref=ref, run_id=run_key)
    fetch_summary = run_json_command(
        [
            sys.executable,
            str(analyzer_script("fetch_repo.py")),
            "--repo-url",
            repo_url,
            "--ref",
            ref,
            "--source-dir",
            str(stage_source_dir),
            "--mode",
            fetch_mode,
            "--timeout",
            str(timeout),
        ]
    )

    resolved_ref = str(fetch_summary.get("resolved_ref") or ref)
    commit_sha = fetch_summary.get("commit_sha")
    snapshot_id = make_snapshot_id(commit_sha if isinstance(commit_sha, str) else None, resolved_ref)
    snapshot_dir = staged_snapshot_dir if staged_snapshot_dir else (paths["snapshots_dir"] / snapshot_id)
    snapshot_source_dir = snapshot_dir / "source"
    snapshot_artifacts_dir = snapshot_dir / "artifacts"
    snapshot_meta_path = snapshot_dir / "snapshot.json"
    snapshot_report_path = snapshot_dir / "report.md"
    snapshot_code_index_path = snapshot_artifacts_dir / "code_index.json"
    snapshot_subagent_path = snapshot_artifacts_dir / "subagent_results.json"

    ensure_dir(snapshot_dir)
    ensure_dir(snapshot_artifacts_dir)
    if staged_snapshot_dir is None:
        replace_dir(stage_source_dir, snapshot_source_dir)
    if snapshot_report_path.exists():
        snapshot_report_path.unlink()
    copied_subagent_path = _copy_optional(subagent_src, snapshot_subagent_path)

    code_index_summary = run_json_command(
        [
            sys.executable,
            str(analyzer_script("build_code_index.py")),
            "--source-dir",
            str(snapshot_source_dir),
            "--output",
            str(snapshot_code_index_path),
        ]
    )

    report_summary: dict[str, Any] | None = None
    report_ready = False
    if render_report:
        render_cmd = [
            sys.executable,
            str(analyzer_script("render_report.py")),
            "--repo-url",
            repo_url,
            "--ref",
            ref,
            "--source-dir",
            str(snapshot_source_dir),
            "--index-json",
            str(snapshot_code_index_path),
            "--output",
            str(snapshot_report_path),
            "--resolved-ref",
            resolved_ref,
            "--language",
            language,
            "--depth",
            depth,
        ]
        if commit_sha:
            render_cmd.extend(["--commit-sha", str(commit_sha)])
        if copied_subagent_path:
            render_cmd.extend(["--subagent-results", copied_subagent_path])
        for feature in feature_list:
            render_cmd.extend(["--feature", feature])

        report_summary = run_json_command(render_cmd)
        copy_file(snapshot_report_path, paths["run_report"])
        report_ready = True
    elif paths["run_report"].exists():
        paths["run_report"].unlink()

    finished_at = now_iso()
    repo_meta = {
        "engine_version": ENGINE_VERSION,
        "repo_key": repo_ref.repo_key,
        "repo_url": repo_ref.repo_url,
        "owner": repo_ref.owner,
        "repo": repo_ref.repo,
        "path": str(paths["repo_dir"]),
        "updated_at": finished_at,
    }
    snapshot_meta = {
        "engine_version": ENGINE_VERSION,
        "repo_key": repo_ref.repo_key,
        "snapshot_id": snapshot_id,
        "path": str(snapshot_dir),
        "requested_ref": ref,
        "resolved_ref": resolved_ref,
        "commit_sha": commit_sha,
        "source_dir": str(snapshot_source_dir),
        "artifacts_dir": str(snapshot_artifacts_dir),
        "report_path": str(snapshot_report_path),
        "report_ready": report_ready,
        "code_index_path": str(snapshot_code_index_path),
        "subagent_results_path": copied_subagent_path,
        "updated_at": finished_at,
        "latest_run_id": run_key,
    }
    run_meta = {
        "engine_version": ENGINE_VERSION,
        "run_id": run_key,
        "status": "succeeded",
        "path": str(paths["run_dir"]),
        "repo_key": repo_ref.repo_key,
        "snapshot_id": snapshot_id,
        "requested_ref": ref,
        "resolved_ref": resolved_ref,
        "commit_sha": commit_sha,
        "report_path": str(snapshot_report_path),
        "report_ready": report_ready,
        "started_at": started_at,
        "finished_at": finished_at,
    }
    run_meta["status"] = "succeeded" if render_report else "prepared"
    write_json(paths["repo_meta"], repo_meta)
    write_json(snapshot_meta_path, snapshot_meta)
    write_json(paths["run_meta"], run_meta)

    flow_summary: dict[str, Any] = {
        "fetch": fetch_summary,
        "code_index": code_index_summary,
        "report": report_summary
        if report_summary is not None
        else {
            "status": "pending",
            "message": "report rendering deferred",
            "path": str(snapshot_report_path),
        },
        "card_count": 0,
        "evidence_count": 0,
    }
    if render_report:
        flow_summary["features"] = feature_list

    payload: dict[str, Any] = {
        "status": "ok",
        "command": command,
        "engine_version": ENGINE_VERSION,
        "generated_at": finished_at,
        "resolved_ref": resolved_ref,
        "commit_sha": commit_sha,
        "report_ready": report_ready,
        **summarize_paths(paths["repo_dir"], snapshot_dir, paths["run_dir"], snapshot_report_path),
        "repo": {
            **repo_meta,
            "metadata_path": str(paths["repo_meta"]),
        },
        "snapshot": {
            **snapshot_meta,
            "metadata_path": str(snapshot_meta_path),
        },
        "run": {
            **run_meta,
            "metadata_path": str(paths["run_meta"]),
        },
        command: flow_summary,
    }

    log_event(
        "INFO",
        f"{command}-finish",
        "仓库入库完成" if render_report else "仓库快照准备完成",
        repo_key=repo_ref.repo_key,
        snapshot_id=snapshot_id,
        run_id=run_key,
        resolved_ref=resolved_ref,
    )
    return payload


def run_prepare(
    *,
    repo_url: str,
    ref: str,
    storage_root: str,
    run_id: str | None,
    snapshot_dir: str | None = None,
    subagent_results: str | None = None,
    fetch_mode: str = "mcp-first",
    timeout: int = 60,
) -> dict[str, Any]:
    """功能：执行 Repo Library prepare，只准备 snapshot 源码与代码索引。
    参数/返回：接收仓库地址、ref、存储根目录和运行 ID，返回可供后续 AI 生成报告的稳定 JSON。
    失败场景：抓仓、建索引或路径落盘失败时抛出 EngineError。
    副作用：创建存储目录、写入 snapshot/source、artifacts/code_index.json 与 repo/snapshot/run 元数据。
    """

    return _run_prepare_flow(
        command="prepare",
        repo_url=repo_url,
        ref=ref,
        storage_root=storage_root,
        run_id=run_id,
        snapshot_dir=snapshot_dir,
        subagent_results=subagent_results,
        features=None,
        render_report=False,
        fetch_mode=fetch_mode,
        timeout=timeout,
    )


def run_ingest(
    *,
    repo_url: str,
    ref: str,
    features: list[str],
    storage_root: str,
    run_id: str | None,
    snapshot_dir: str | None = None,
    subagent_results: str | None = None,
    language: str = "zh",
    depth: str = "standard",
    fetch_mode: str = "mcp-first",
    timeout: int = 60,
) -> dict[str, Any]:
    """功能：执行 Repo Library ingest，生成源码快照、代码索引与报告。
    参数/返回：接收仓库地址、ref、feature 列表、存储根目录和运行 ID，返回稳定 JSON 元数据。
    失败场景：抓仓、建索引、渲染报告或路径落盘失败时抛出 EngineError。
    副作用：创建存储目录、下载仓库源码、写入索引与报告文件、更新 repo/snapshot/run 元数据。
    """

    return _run_prepare_flow(
        command="ingest",
        repo_url=repo_url,
        ref=ref,
        storage_root=storage_root,
        run_id=run_id,
        snapshot_dir=snapshot_dir,
        subagent_results=subagent_results,
        features=features,
        render_report=True,
        language=language,
        depth=depth,
        fetch_mode=fetch_mode,
        timeout=timeout,
    )
