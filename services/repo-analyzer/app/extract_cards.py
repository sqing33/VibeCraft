#!/usr/bin/env python3
"""Extract structured Repo Library knowledge cards from analyzer outputs."""

from __future__ import annotations

import hashlib
import re
from pathlib import Path
from typing import Any

from helpers import ENGINE_VERSION, EngineError, now_iso, parse_repo_url, read_json

RUN_PATTERN = re.compile(r"^##\s+Run\s+\d+\s*$", flags=re.MULTILINE)
H2_PATTERN = re.compile(r"^##\s+(?!#)(.+?)\s*$")
H3_PATTERN = re.compile(r"^###\s+(?!#)(.+?)\s*$")
H4_PATTERN = re.compile(r"^####\s+(?!#)(.+?)\s*$")
H5_PATTERN = re.compile(r"^#####\s+(?!#)(.+?)\s*$")
FEATURE_TITLE_PATTERN = re.compile(r"^(?:功能|Feature)\s+\d+\s*:\s*(.+?)\s*$")
CHARACTERISTIC_TITLE_PATTERN = re.compile(r"^(?:项目特点|Characteristic)\s+\d+\s*:\s*(.+?)\s*$")
EVIDENCE_PATTERN = re.compile(
    r"`?([A-Za-z0-9_./-]+\.[A-Za-z0-9_+.-]+):(\d+)`?(?:\s+\[([^\]]+)\])?(?:\s+-\s+`?(.+?)`?)?$"
)

TITLE_KEYS = {
    "part_one": {"第一部分：项目参数与结构解析", "Part 1: Project Parameters and Structure"},
    "part_two": {"第二部分：面向人的功能说明", "Part 2: Human-readable Feature Explanation"},
    "part_three": {"第三部分：面向 AI 的实现细节与证据链", "Part 3: AI-facing Mechanism Details and Evidence"},
    "overview": {"仓库结构心智模型", "Repository Mental Model"},
    "project_characteristics": {"项目特点与标志实现", "Project Characteristics and Signature Implementations"},
    "feature_details": {"面向 AI 的实现细节", "Feature Principle Analysis"},
    "global_risks": {"跨功能耦合与系统风险", "Cross-feature Coupling and System Risks"},
}

LABEL_KEYS = {
    "confidence": {"置信度", "Confidence"},
    "evidence_refs": {"关键证据引用", "Key Evidence References"},
    "characteristic_source": {"来源", "Source"},
    "characteristic_signal": {"README 线索", "README Signal"},
    "characteristic_mechanism": {"实现机制", "Implementation Mechanism"},
    "function_role": {"功能作用", "Function Role"},
    "special_capability": {"特殊功能", "Special Capability"},
    "implementation_idea": {"实现想法", "Implementation Idea"},
    "key_evidence": {"关键证据", "Key Evidence"},
    "unknowns": {"推断与未知点", "Inference and Unknowns"},
}


def split_by_heading(lines: list[str], pattern: re.Pattern[str]) -> list[tuple[str, list[str]]]:
    """Split markdown content into blocks keyed by the requested heading level."""

    blocks: list[tuple[str, list[str]]] = []
    current_title: str | None = None
    current_lines: list[str] = []
    for line in lines:
        match = pattern.match(line)
        if match:
            if current_title is not None:
                blocks.append((current_title, current_lines))
            current_title = match.group(1).strip()
            current_lines = []
            continue
        if current_title is not None:
            current_lines.append(line)
    if current_title is not None:
        blocks.append((current_title, current_lines))
    return blocks


def section_key(title: str) -> str | None:
    """Map localized markdown titles to stable internal keys."""

    clean = title.strip()
    for key, aliases in TITLE_KEYS.items():
        if clean in aliases:
            return key
    return None


def label_key(label: str) -> str | None:
    """Map localized bullet labels to stable internal keys."""

    clean = label.strip()
    for key, aliases in LABEL_KEYS.items():
        if clean in aliases:
            return key
    return None


def latest_run_text(raw: str) -> str:
    """Return the latest appended run block from a rendered report."""

    matches = list(RUN_PATTERN.finditer(raw))
    if not matches:
        return raw
    last = matches[-1]
    return raw[last.start() :]


def parse_bullets(lines: list[str]) -> tuple[dict[str, Any], list[str]]:
    """Parse simple '- key: value' blocks into a structured dictionary."""

    fields: dict[str, Any] = {}
    free_lines: list[str] = []
    current_list_key: str | None = None
    for raw_line in lines:
        line = raw_line.rstrip()
        stripped = line.strip()
        if not stripped:
            current_list_key = None
            continue
        if stripped.startswith("- "):
            body = stripped[2:].strip()
            if ":" in body and not body.startswith("`"):
                label, value = body.split(":", 1)
                key = label_key(label)
                if key:
                    value_text = value.strip().strip("`")
                    if key == "evidence_refs":
                        fields[key] = []
                        current_list_key = key
                    else:
                        fields[key] = value_text
                        current_list_key = None
                    continue
            if current_list_key == "evidence_refs":
                refs = fields.setdefault(current_list_key, [])
                refs.append(body.strip().strip("`") )
                continue
            free_lines.append(body)
            current_list_key = None
            continue
        free_lines.append(stripped)
    return fields, free_lines


def parse_evidence_lines(lines: list[str], *, default_label: str, origin: str) -> list[dict[str, Any]]:
    """Parse report or subagent evidence into normalized evidence dictionaries."""

    evidence: list[dict[str, Any]] = []
    seen: set[tuple[str, int, str, str]] = set()
    for raw_line in lines:
        stripped = raw_line.strip().lstrip("-").strip()
        stripped = stripped.strip("`")
        if not stripped or stripped.lower() == "none" or stripped == "无":
            continue
        match = EVIDENCE_PATTERN.search(raw_line.strip())
        if not match:
            continue
        path = match.group(1)
        line = int(match.group(2))
        label = (match.group(3) or default_label or "evidence").strip()
        snippet = (match.group(4) or "").strip().strip("`")
        dedupe_key = (path, line, label, snippet)
        if dedupe_key in seen:
            continue
        seen.add(dedupe_key)
        evidence.append(
            {
                "source_path": path,
                "source_line": line,
                "label": label,
                "snippet": snippet,
                "origin": origin,
            }
        )
    return evidence


def merge_confidence(values: list[str]) -> str:
    """Merge multiple confidence values into one stable output level."""

    score_map = {"low": 1, "medium": 2, "high": 3}
    valid = [score_map[value] for value in values if value in score_map]
    if not valid:
        return "low"
    avg = sum(valid) / len(valid)
    if avg >= 2.5:
        return "high"
    if avg >= 1.5:
        return "medium"
    return "low"


def make_card_id(card_type: str, title: str, snapshot_id: str | None) -> str:
    """Generate a deterministic card id from stable card fields."""

    seed = "|".join([card_type, title.strip(), snapshot_id or "unknown"])
    return hashlib.sha1(seed.encode("utf-8")).hexdigest()[:20]


def make_evidence_id(card_id: str, item: dict[str, Any]) -> str:
    """Generate a deterministic evidence id for one card evidence item."""

    seed = "|".join(
        [
            card_id,
            str(item.get("source_path") or ""),
            str(item.get("source_line") or ""),
            str(item.get("label") or ""),
        ]
    )
    return hashlib.sha1(seed.encode("utf-8")).hexdigest()[:20]


def _normalize_report(report_text: str) -> dict[str, Any]:
    run_text = latest_run_text(report_text)
    h2_blocks = split_by_heading(run_text.splitlines(), H2_PATTERN)
    parts: dict[str, list[str]] = {"part_one": [], "part_two": [], "part_three": []}
    for title, body in h2_blocks:
        key = section_key(title)
        if key in parts:
            parts[key] = body

    part_one = dict(split_by_heading(parts["part_one"], H3_PATTERN))
    part_two = split_by_heading(parts["part_two"], H3_PATTERN)
    part_three = dict(split_by_heading(parts["part_three"], H3_PATTERN))

    feature_summaries: dict[str, dict[str, Any]] = {}
    for title, body in part_two:
        match = FEATURE_TITLE_PATTERN.match(title)
        if not match:
            continue
        feature_name = match.group(1).strip()
        fields, free_lines = parse_bullets(body)
        evidence = parse_evidence_lines(body, default_label="feature_summary", origin="report")
        feature_summaries[feature_name] = {
            "title": feature_name,
            "fields": fields,
            "notes": free_lines,
            "evidence": evidence,
        }

    characteristics: list[dict[str, Any]] = []
    raw_characteristics = part_one.get(next(iter(TITLE_KEYS["project_characteristics"]), ""), [])
    if not raw_characteristics:
        for title in TITLE_KEYS["project_characteristics"]:
            if title in part_one:
                raw_characteristics = part_one[title]
                break
    for title, body in split_by_heading(raw_characteristics, H4_PATTERN):
        match = CHARACTERISTIC_TITLE_PATTERN.match(title)
        item_title = match.group(1).strip() if match else title.strip()
        fields, free_lines = parse_bullets(body)
        characteristics.append(
            {
                "title": item_title,
                "fields": fields,
                "notes": free_lines,
                "evidence": parse_evidence_lines(body, default_label="project_characteristic", origin="report"),
            }
        )

    overview_body: list[str] = []
    for title in TITLE_KEYS["overview"]:
        if title in part_one:
            overview_body = part_one[title]
            break

    feature_details_body: list[str] = []
    for title in TITLE_KEYS["feature_details"]:
        if title in part_three:
            feature_details_body = part_three[title]
            break

    feature_details: dict[str, dict[str, Any]] = {}
    for title, body in split_by_heading(feature_details_body, H4_PATTERN):
        match = FEATURE_TITLE_PATTERN.match(title)
        feature_name = match.group(1).strip() if match else title.strip()
        subsections = split_by_heading(body, H5_PATTERN)
        detail_sections: dict[str, list[str]] = {}
        confidences: list[str] = []
        evidence_items: list[dict[str, Any]] = []
        notes: list[str] = []
        for subsection_title, subsection_lines in subsections:
            detail_sections[subsection_title] = subsection_lines
            fields, free_lines = parse_bullets(subsection_lines)
            confidence = fields.get("confidence")
            if isinstance(confidence, str):
                confidences.append(confidence)
            if section_key(subsection_title) is None and subsection_title in TITLE_KEYS["global_risks"]:
                notes.extend(free_lines)
            if subsection_title in TITLE_KEYS["global_risks"]:
                notes.extend(free_lines)
            if subsection_title in TITLE_KEYS["overview"]:
                notes.extend(free_lines)
            key_evidence = label_key(subsection_title)
            if key_evidence == "key_evidence":
                evidence_items.extend(parse_evidence_lines(subsection_lines, default_label="feature_detail", origin="report"))
            elif label_key(subsection_title) == "unknowns":
                notes.extend(free_lines)
        feature_details[feature_name] = {
            "title": feature_name,
            "sections": detail_sections,
            "confidence": merge_confidence(confidences),
            "evidence": evidence_items,
            "notes": notes,
        }

    risks_body: list[str] = []
    for title in TITLE_KEYS["global_risks"]:
        if title in part_three:
            risks_body = part_three[title]
            break

    return {
        "overview": overview_body,
        "characteristics": characteristics,
        "feature_summaries": feature_summaries,
        "feature_details": feature_details,
        "risks": [line.strip()[2:].strip() for line in risks_body if line.strip().startswith("- ")],
    }


def _append_card(
    *,
    cards: list[dict[str, Any]],
    evidence: list[dict[str, Any]],
    snapshot_id: str | None,
    repo_key: str | None,
    run_id: str | None,
    card_type: str,
    title: str,
    summary: str,
    content: str,
    confidence: str,
    tags: list[str],
    source: str,
    evidence_items: list[dict[str, Any]],
) -> None:
    card_id = make_card_id(card_type, title, snapshot_id)
    card_evidence_ids: list[str] = []
    for item in evidence_items:
        evidence_id = make_evidence_id(card_id, item)
        card_evidence_ids.append(evidence_id)
        evidence.append({
            "evidence_id": evidence_id,
            "card_id": card_id,
            **item,
        })

    cards.append(
        {
            "card_id": card_id,
            "type": card_type,
            "title": title,
            "summary": summary.strip(),
            "content": content.strip(),
            "confidence": confidence,
            "tags": [tag for tag in tags if tag],
            "repo_key": repo_key,
            "snapshot_id": snapshot_id,
            "run_id": run_id,
            "source": source,
            "evidence_ids": card_evidence_ids,
            "evidence_count": len(card_evidence_ids),
        }
    )


def build_cards_payload(
    *,
    report_path: Path,
    output_path: Path,
    repo_url: str | None = None,
    repo_key: str | None = None,
    snapshot_id: str | None = None,
    snapshot_dir: Path | None = None,
    run_id: str | None = None,
    subagent_results_path: Path | None = None,
) -> dict[str, Any]:
    """功能：从 report.md 与可选 subagent_results.json 抽取卡片与证据。
    参数/返回：接收报告路径、输出路径与上下文标识，返回包含 cards/evidence 的稳定 JSON。
    失败场景：报告文件缺失、JSON 非法或 markdown 结构无法读取时抛出 EngineError。
    副作用：读取报告和子代理产物，生成 cards/evidence JSON 并准备给调用方写盘。
    """

    if not report_path.exists() or not report_path.is_file():
        raise EngineError(f"report_path does not exist: {report_path}")
    if subagent_results_path and (not subagent_results_path.exists() or not subagent_results_path.is_file()):
        raise EngineError(f"subagent_results does not exist: {subagent_results_path}")

    resolved_repo_key = repo_key
    if repo_url and not resolved_repo_key:
        resolved_repo_key = parse_repo_url(repo_url).repo_key

    normalized = _normalize_report(report_path.read_text(encoding="utf-8", errors="ignore"))
    cards: list[dict[str, Any]] = []
    evidence: list[dict[str, Any]] = []

    overview_lines = [line.strip()[2:].strip() if line.strip().startswith("- ") else line.strip() for line in normalized["overview"] if line.strip()]
    if overview_lines:
        overview_text = "\n".join(overview_lines)
        _append_card(
            cards=cards,
            evidence=evidence,
            snapshot_id=snapshot_id,
            repo_key=resolved_repo_key,
            run_id=run_id,
            card_type="integration_note",
            title="Repository structure mental model",
            summary=overview_lines[0],
            content=overview_text,
            confidence="medium",
            tags=["overview", "integration"],
            source="report.md",
            evidence_items=parse_evidence_lines(normalized["overview"], default_label="overview", origin="report"),
        )

    for item in normalized["characteristics"]:
        fields = item["fields"]
        summary = str(fields.get("characteristic_mechanism") or item["title"])
        content_parts = []
        if fields.get("characteristic_source"):
            content_parts.append(f"source: {fields['characteristic_source']}")
        if fields.get("characteristic_signal"):
            content_parts.append(f"signal: {fields['characteristic_signal']}")
        content_parts.append(summary)
        content_parts.extend(item.get("notes", []))
        _append_card(
            cards=cards,
            evidence=evidence,
            snapshot_id=snapshot_id,
            repo_key=resolved_repo_key,
            run_id=run_id,
            card_type="project_characteristic",
            title=item["title"],
            summary=summary,
            content="\n".join(content_parts),
            confidence=str(fields.get("confidence") or ("medium" if item["evidence"] else "low")),
            tags=["characteristic", item["title"]],
            source="report.md",
            evidence_items=item["evidence"],
        )

    all_features = sorted(set(normalized["feature_summaries"]) | set(normalized["feature_details"]))
    for feature_name in all_features:
        summary_block = normalized["feature_summaries"].get(feature_name, {})
        detail_block = normalized["feature_details"].get(feature_name, {})
        summary_fields = summary_block.get("fields", {})
        content_lines: list[str] = []
        if summary_fields.get("function_role"):
            content_lines.append(f"function_role: {summary_fields['function_role']}")
        if summary_fields.get("special_capability"):
            content_lines.append(f"special_capability: {summary_fields['special_capability']}")
        if summary_fields.get("implementation_idea"):
            content_lines.append(f"implementation_idea: {summary_fields['implementation_idea']}")
        for section_title, section_lines in detail_block.get("sections", {}).items():
            cleaned = [line.strip()[2:].strip() if line.strip().startswith("- ") else line.strip() for line in section_lines if line.strip()]
            if cleaned:
                content_lines.append(f"{section_title}: {' '.join(cleaned)}")
        content_lines.extend(summary_block.get("notes", []))
        content_lines.extend(detail_block.get("notes", []))
        evidence_items = list(summary_block.get("evidence", [])) + list(detail_block.get("evidence", []))
        confidence = merge_confidence(
            [
                str(summary_fields.get("confidence") or ""),
                str(detail_block.get("confidence") or ""),
            ]
        )
        summary = str(summary_fields.get("implementation_idea") or summary_fields.get("function_role") or feature_name)
        _append_card(
            cards=cards,
            evidence=evidence,
            snapshot_id=snapshot_id,
            repo_key=resolved_repo_key,
            run_id=run_id,
            card_type="feature_pattern",
            title=feature_name,
            summary=summary,
            content="\n".join(content_lines) if content_lines else summary,
            confidence=confidence,
            tags=["feature", feature_name],
            source="report.md",
            evidence_items=evidence_items,
        )

    for index, risk in enumerate(normalized["risks"], start=1):
        _append_card(
            cards=cards,
            evidence=evidence,
            snapshot_id=snapshot_id,
            repo_key=resolved_repo_key,
            run_id=run_id,
            card_type="risk_note",
            title=f"Risk {index}",
            summary=risk,
            content=risk,
            confidence="medium",
            tags=["risk"],
            source="report.md",
            evidence_items=[],
        )

    if subagent_results_path:
        payload = read_json(subagent_results_path)
        overview = payload.get("overview") if isinstance(payload, dict) else None
        if isinstance(overview, dict) and isinstance(overview.get("summary"), str) and overview.get("summary", "").strip():
            sub_evidence = []
            for item in overview.get("evidence", []):
                if not isinstance(item, dict):
                    continue
                path = item.get("path")
                line = item.get("line")
                if not isinstance(path, str) or not isinstance(line, int) or line <= 0:
                    continue
                sub_evidence.append(
                    {
                        "source_path": path,
                        "source_line": line,
                        "label": str(item.get("for_dimension") or "overview"),
                        "snippet": str(item.get("snippet") or "").strip(),
                        "origin": "subagent",
                    }
                )
            _append_card(
                cards=cards,
                evidence=evidence,
                snapshot_id=snapshot_id,
                repo_key=resolved_repo_key,
                run_id=run_id,
                card_type="integration_note",
                title="Subagent overview",
                summary=str(overview["summary"]).strip(),
                content=str(overview["summary"]).strip(),
                confidence=str(overview.get("confidence") or "medium"),
                tags=["overview", "subagent"],
                source="subagent_results.json",
                evidence_items=sub_evidence,
            )

        architecture = payload.get("architecture") if isinstance(payload, dict) else None
        if isinstance(architecture, dict) and isinstance(architecture.get("summary"), str) and architecture.get("summary", "").strip():
            arch_evidence = []
            for item in architecture.get("evidence", []):
                if not isinstance(item, dict):
                    continue
                path = item.get("path")
                line = item.get("line")
                if not isinstance(path, str) or not isinstance(line, int) or line <= 0:
                    continue
                arch_evidence.append(
                    {
                        "source_path": path,
                        "source_line": line,
                        "label": str(item.get("for_dimension") or "architecture"),
                        "snippet": str(item.get("snippet") or "").strip(),
                        "origin": "subagent",
                    }
                )
            _append_card(
                cards=cards,
                evidence=evidence,
                snapshot_id=snapshot_id,
                repo_key=resolved_repo_key,
                run_id=run_id,
                card_type="integration_note",
                title="Subagent architecture",
                summary=str(architecture["summary"]).strip(),
                content=str(architecture["summary"]).strip(),
                confidence=str(architecture.get("confidence") or "medium"),
                tags=["architecture", "integration", "subagent"],
                source="subagent_results.json",
                evidence_items=arch_evidence,
            )

    type_counts: dict[str, int] = {}
    for card in cards:
        card_type = str(card.get("type") or "unknown")
        type_counts[card_type] = type_counts.get(card_type, 0) + 1

    return {
        "status": "ok",
        "command": "extract-cards",
        "engine_version": ENGINE_VERSION,
        "generated_at": now_iso(),
        "repo_key": resolved_repo_key,
        "snapshot_id": snapshot_id,
        "run_id": run_id,
        "snapshot_dir": str(snapshot_dir) if snapshot_dir else None,
        "report_path": str(report_path),
        "subagent_results_path": str(subagent_results_path) if subagent_results_path else None,
        "cards_path": str(output_path),
        "card_count": len(cards),
        "evidence_count": len(evidence),
        "type_counts": type_counts,
        "cards": cards,
        "evidence": evidence,
    }
