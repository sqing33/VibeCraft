#!/usr/bin/env python3
"""Single-entry Repo Library CLI for prepare, ingest, extraction, search, and pipeline runs."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Any

CURRENT_DIR = Path(__file__).resolve().parent
if str(CURRENT_DIR) not in sys.path:
    sys.path.insert(0, str(CURRENT_DIR))

from extract_cards import build_cards_payload
from helpers import ENGINE_VERSION, EngineError, log_event, write_json, write_result
from ingest import run_ingest, run_prepare
from search import run_search


def handle_prepare(args: argparse.Namespace) -> dict[str, Any]:
    return run_prepare(
        repo_url=args.repo_url,
        ref=args.ref,
        storage_root=args.storage_root,
        run_id=args.run_id,
        snapshot_dir=args.snapshot_dir,
        subagent_results=args.subagent_results,
        fetch_mode=args.fetch_mode,
        timeout=args.timeout,
    )


def handle_ingest(args: argparse.Namespace) -> dict[str, Any]:
    return run_ingest(
        repo_url=args.repo_url,
        ref=args.ref,
        features=args.feature,
        storage_root=args.storage_root,
        run_id=args.run_id,
        snapshot_dir=args.snapshot_dir,
        subagent_results=args.subagent_results,
        language=args.language,
        depth=args.depth,
        fetch_mode=args.fetch_mode,
        timeout=args.timeout,
    )


def handle_extract_cards(args: argparse.Namespace) -> dict[str, Any]:
    report_path = Path(args.report_path).resolve()
    output_path = Path(args.output).resolve()
    snapshot_dir = Path(args.snapshot_dir).resolve() if args.snapshot_dir else None
    subagent_results_path = Path(args.subagent_results).resolve() if args.subagent_results else None
    return build_cards_payload(
        report_path=report_path,
        output_path=output_path,
        repo_url=args.repo_url,
        repo_key=args.repo_key,
        snapshot_id=args.snapshot_id,
        snapshot_dir=snapshot_dir,
        run_id=args.run_id,
        subagent_results_path=subagent_results_path,
    )


def handle_search(args: argparse.Namespace) -> dict[str, Any]:
    output_path = Path(args.output).resolve()
    return run_search(
        storage_root=args.storage_root,
        output_path=output_path,
        refresh=args.refresh,
        query=args.query,
        repo_filters=args.repo,
        limit=args.limit,
        min_score=args.min_score,
        mode=args.mode,
        repo_url=args.repo_url,
        repo_key=args.repo_key,
        snapshot_id=args.snapshot_id,
        snapshot_dir=Path(args.snapshot_dir).resolve() if args.snapshot_dir else None,
        run_id=args.run_id,
        report_path=Path(args.report_path).resolve() if args.report_path else None,
        subagent_results_path=Path(args.subagent_results).resolve() if args.subagent_results else None,
        cards_path=Path(args.cards_path).resolve() if args.cards_path else None,
    )


def handle_pipeline(args: argparse.Namespace) -> dict[str, Any]:
    ingest_payload = handle_ingest(args)
    snapshot = ingest_payload.get("snapshot", {})
    run_meta = ingest_payload.get("run", {})

    cards_output = Path(snapshot["artifacts_dir"]) / "cards.json"
    cards_payload = build_cards_payload(
        report_path=Path(snapshot["report_path"]),
        output_path=cards_output,
        repo_url=args.repo_url,
        repo_key=ingest_payload.get("repo", {}).get("repo_key"),
        snapshot_id=snapshot.get("snapshot_id"),
        snapshot_dir=Path(snapshot["path"]),
        run_id=run_meta.get("run_id"),
        subagent_results_path=Path(snapshot["subagent_results_path"]).resolve() if snapshot.get("subagent_results_path") else None,
    )
    write_json(cards_output, cards_payload)

    search_payload = run_search(
        storage_root=args.storage_root,
        output_path=Path(args.output).resolve(),
        refresh=args.search_refresh,
        query=None,
        repo_filters=[],
        repo_url=args.repo_url,
        repo_key=ingest_payload.get("repo", {}).get("repo_key"),
        snapshot_id=snapshot.get("snapshot_id"),
        snapshot_dir=Path(snapshot["path"]),
        run_id=run_meta.get("run_id"),
        report_path=Path(snapshot["report_path"]),
        subagent_results_path=Path(snapshot["subagent_results_path"]).resolve() if snapshot.get("subagent_results_path") else None,
        cards_path=cards_output,
    )

    payload: dict[str, Any] = {
        "status": "ok",
        "command": "pipeline",
        "engine_version": ENGINE_VERSION,
        "generated_at": search_payload.get("generated_at"),
        "resolved_ref": ingest_payload.get("resolved_ref"),
        "commit_sha": ingest_payload.get("commit_sha"),
        "repo_path": ingest_payload.get("repo_path"),
        "snapshot_path": ingest_payload.get("snapshot_path"),
        "run_path": ingest_payload.get("run_path"),
        "report_path": ingest_payload.get("report_path"),
        "cards_path": cards_payload.get("cards_path"),
        "card_count": cards_payload.get("card_count"),
        "evidence_count": cards_payload.get("evidence_count"),
        "search_refresh": search_payload.get("search_refresh"),
        "repo": ingest_payload.get("repo"),
        "snapshot": ingest_payload.get("snapshot"),
        "run": ingest_payload.get("run"),
        "ingest": ingest_payload.get("ingest"),
        "cards": {
            "cards_path": cards_payload.get("cards_path"),
            "card_count": cards_payload.get("card_count"),
            "evidence_count": cards_payload.get("evidence_count"),
            "type_counts": cards_payload.get("type_counts"),
        },
        "search": {
            "storage_root": search_payload.get("storage_root"),
            "corpus_root": search_payload.get("corpus_root"),
            "index_root": search_payload.get("index_root"),
            "sync": search_payload.get("sync"),
            "refresh": search_payload.get("search_refresh"),
        },
    }
    return payload


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    def add_repo_prepare_flags(target: argparse.ArgumentParser) -> None:
        target.add_argument("--repo-url", required=True)
        target.add_argument("--ref", default="main")
        target.add_argument("--storage-root", required=True)
        target.add_argument("--run-id", default=None)
        target.add_argument("--snapshot-dir", default=None, help="Absolute snapshot directory preallocated by backend")
        target.add_argument("--subagent-results", default=None)
        target.add_argument("--fetch-mode", choices=["mcp-first", "git-only", "api-only"], default="mcp-first")
        target.add_argument("--timeout", type=int, default=60)
        target.add_argument("--output", required=True)

    def add_report_render_flags(target: argparse.ArgumentParser) -> None:
        target.add_argument("--feature", action="append", required=True)
        target.add_argument("--language", choices=["zh", "en"], default="zh")
        target.add_argument("--depth", choices=["standard", "deep"], default="standard")

    pipeline_parser = subparsers.add_parser("pipeline", help="Run ingest + extract-cards + search refresh")
    add_repo_prepare_flags(pipeline_parser)
    add_report_render_flags(pipeline_parser)
    pipeline_parser.add_argument("--search-refresh", choices=["auto", "force", "never"], default="auto")
    pipeline_parser.set_defaults(handler=handle_pipeline)

    prepare_parser = subparsers.add_parser("prepare", help="Fetch source and build code index without rendering report")
    add_repo_prepare_flags(prepare_parser)
    prepare_parser.set_defaults(handler=handle_prepare)

    ingest_parser = subparsers.add_parser("ingest", help="Fetch source, build index, render report")
    add_repo_prepare_flags(ingest_parser)
    add_report_render_flags(ingest_parser)
    ingest_parser.set_defaults(handler=handle_ingest)

    extract_parser = subparsers.add_parser("extract-cards", help="Derive cards and evidence from analyzer outputs")
    extract_parser.add_argument("--report-path", required=True)
    extract_parser.add_argument("--subagent-results", default=None)
    extract_parser.add_argument("--repo-url", default=None)
    extract_parser.add_argument("--repo-key", default=None)
    extract_parser.add_argument("--snapshot-id", default=None)
    extract_parser.add_argument("--snapshot-dir", default=None)
    extract_parser.add_argument("--run-id", default=None)
    extract_parser.add_argument("--output", required=True)
    extract_parser.set_defaults(handler=handle_extract_cards)

    search_parser = subparsers.add_parser("search", help="Refresh or query the Repo Library vector index")
    search_parser.add_argument("--storage-root", required=True)
    search_parser.add_argument("--refresh", choices=["auto", "force", "never"], default="auto")
    search_parser.add_argument("--query", default=None)
    search_parser.add_argument("--repo", action="append", default=[])
    search_parser.add_argument("--limit", type=int, default=10)
    search_parser.add_argument("--min-score", type=float, default=0.35)
    search_parser.add_argument("--mode", choices=["compact", "semi", "full"], default="semi")
    search_parser.add_argument("--repo-url", default=None)
    search_parser.add_argument("--repo-key", default=None)
    search_parser.add_argument("--snapshot-id", default=None)
    search_parser.add_argument("--snapshot-dir", default=None)
    search_parser.add_argument("--run-id", default=None)
    search_parser.add_argument("--report-path", default=None)
    search_parser.add_argument("--subagent-results", default=None)
    search_parser.add_argument("--cards-path", default=None)
    search_parser.add_argument("--output", required=True)
    search_parser.set_defaults(handler=handle_search)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    output_path = Path(args.output).resolve()
    try:
        payload = args.handler(args)
        write_result(output_path, payload)
        return 0
    except EngineError as exc:
        log_event("ERROR", "command-failed", "Repo Library 命令失败", command=args.command, error=str(exc))
        payload = {
            "status": "error",
            "command": args.command,
            "engine_version": ENGINE_VERSION,
            "error": {
                "message": str(exc),
                "details": exc.details,
            },
        }
        write_result(output_path, payload)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
