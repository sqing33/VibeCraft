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
FEATURE_TITLE_PATTERN = re.compile(r"^(?:(?:功能|feature)(?:\s+\d+)?)\s*:\s*(.+?)\s*$", flags=re.IGNORECASE)
CHARACTERISTIC_TITLE_PATTERN = re.compile(r"^(?:(?:项目特点|characteristic)(?:\s+\d+)?)\s*:\s*(.+?)\s*$", flags=re.IGNORECASE)
EVIDENCE_PATTERN = re.compile(
    r"`?([A-Za-z0-9_./-]+\.[A-Za-z0-9_+.-]+):(\d+)(?:-(\d+))?`?"
    r"(?:\s+\[([^\]]+)\])?(?:\s+-\s+`?(.+?)`?)?$"
)

TITLE_KEYS = {
    "part_one": {"第一部分：项目参数与结构解析", "Part 1: Project Parameters and Structure"},
    "part_two": {"第二部分：面向人的功能说明", "Part 2: Human-readable Feature Explanation"},
    "part_three": {"第三部分：面向 AI 的实现细节与证据链", "Part 3: AI-facing Mechanism Details and Evidence"},
    "overview": {"仓库结构心智模型", "Repository Mental Model"},
    "project_characteristics": {"项目特点与标志实现", "Project Characteristics and Signature Implementations"},
    "executive_summary": {"面向人的功能说明", "Executive Principle Summary", "Features", "功能说明"},
    "feature_details": {"面向 AI 的实现细节", "Feature Principle Analysis"},
    "global_risks": {"跨功能耦合与系统风险", "Cross-feature Coupling and System Risks"},
}

LABEL_KEYS = {
    "confidence": {"置信度", "Confidence"},
    "evidence_refs": {"关键证据引用", "Key Evidence References", "Evidence References"},
    "characteristic_source": {"来源", "Source"},
    "characteristic_signal": {"README 线索", "README Signal"},
    "characteristic_mechanism": {"实现机制", "Implementation Mechanism"},
    "function_role": {"功能作用", "Function Role", "Role"},
    "special_capability": {"特殊功能", "Special Capability", "Capability", "Capabilities"},
    "implementation_idea": {"实现想法", "Implementation Idea", "Implementation", "Approach", "实现思路"},
    "key_evidence": {"关键证据", "Key Evidence", "Evidence"},
    "unknowns": {"推断与未知点", "Inference and Unknowns", "Unknowns", "Notes and Unknowns"},
}

FORMAL_REPORT_MARKERS = {
    "# github 功能实现原理报告",
    "## run 1",
    "## 第一部分：项目参数与结构解析",
    "## 第二部分：面向人的功能说明",
    "## 第三部分：面向 ai 的实现细节与证据链",
    "## 第一部分：技术栈与模块语言",
    "## 第二部分：项目用途与核心特点",
    "## 第三部分：特点实现思路",
    "## 第四部分：提问与解答",
    "## 第五部分：实现定位与证据",
}

FORMAL_REPORT_V2_TITLES = {
    "第一部分：技术栈与模块语言": "part_one",
    "第二部分：项目用途与核心特点": "part_two",
    "第三部分：特点实现思路": "part_three",
    "第四部分：提问与解答": "part_four",
    "第五部分：实现定位与证据": "part_five",
}

FORMAL_REPORT_V2_CHARACTERISTIC_PATTERN = re.compile(r"^特点\s+(\d+)\s*[:：]\s*(.+?)\s*$")
FORMAL_REPORT_V2_QUESTION_PATTERN = re.compile(r"^问题\s+(\d+)\s*[:：]\s*(.+?)\s*$")


def _strip_guidance_lines(lines: list[str]) -> list[str]:
    cleaned: list[str] = []
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("（") and stripped.endswith("）"):
            continue
        cleaned.append(line)
    return cleaned


def _parse_simple_bullets(lines: list[str]) -> tuple[dict[str, str], list[str]]:
    fields: dict[str, str] = {}
    notes: list[str] = []
    for raw_line in lines:
        stripped = raw_line.strip()
        if not stripped:
            continue
        if stripped.startswith("- "):
            body = stripped[2:].strip()
            if ":" in body or "：" in body:
                label, value = re.split(r"[:：]", body, maxsplit=1)
                label = label.strip()
                value = value.strip()
                if label:
                    fields[label] = value
                    continue
            notes.append(body)
            continue
        notes.append(stripped)
    return fields, notes


def _safe_join_lines(lines: list[str]) -> str:
    out: list[str] = []
    for line in lines:
        stripped = line.rstrip()
        if stripped:
            out.append(stripped)
    return "\n".join(out).strip()

SEARCH_SECTION_TITLES = {
    "project_characteristics": "Project Characteristics and Signature Implementations",
    "executive_summary": "Executive Principle Summary",
    "feature_details": "Feature Principle Analysis",
    "global_risks": "Cross-feature Coupling and System Risks",
    "overview": "Repository Mental Model",
}

DETAIL_SECTION_TITLES = {
    "runtime_control_flow": {"运行时控制流", "Runtime Control Flow", "Runtime"},
    "data_flow": {"数据流", "Data Flow"},
    "state_lifecycle": {"状态与生命周期", "State and Lifecycle", "Lifecycle"},
    "failure_recovery": {"失败与恢复", "Failure and Recovery", "Recovery"},
    "concurrency_timing": {"并发与时序", "Concurrency and Timing", "Concurrency"},
    "key_evidence": {"关键证据", "Key Evidence", "Evidence"},
    "unknowns": {"推断与未知点", "Inference and Unknowns", "Unknowns"},
}


def _normalize_key(text: str) -> str:
    return re.sub(r"[^0-9a-z\u4e00-\u9fff]+", " ", text.casefold()).strip()


NORMALIZED_TITLE_KEYS = {
    key: {_normalize_key(alias) for alias in aliases}
    for key, aliases in TITLE_KEYS.items()
}

NORMALIZED_LABEL_KEYS = {
    key: {_normalize_key(alias) for alias in aliases}
    for key, aliases in LABEL_KEYS.items()
}

NORMALIZED_DETAIL_SECTION_TITLES = {
    key: {_normalize_key(alias) for alias in aliases}
    for key, aliases in DETAIL_SECTION_TITLES.items()
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

    clean = _normalize_key(title)
    for key, aliases in NORMALIZED_TITLE_KEYS.items():
        if clean in aliases:
            return key
    if clean.startswith("part 1") or "第一部分" in clean:
        return "part_one"
    if clean.startswith("part 2") or "第二部分" in clean:
        return "part_two"
    if clean.startswith("part 3") or "第三部分" in clean:
        return "part_three"
    if clean in {"overview", "repository overview", "project overview"} or any(
        token in clean
        for token in ["mental model", "仓库结构心智模型", "仓库概览", "项目概览", "architecture overview"]
    ):
        return "overview"
    if clean == "features":
        return "executive_summary"
    if any(token in clean for token in ["project characteristics", "signature implementations", "key characteristics", "项目特点", "标志实现"]):
        return "project_characteristics"
    if any(token in clean for token in ["executive principle summary", "human readable feature", "功能说明", "feature summary"]):
        return "executive_summary"
    if any(token in clean for token in ["feature principle analysis", "ai details", "implementation details", "实现细节", "证据链"]):
        return "feature_details"
    if any(token in clean for token in ["cross feature", "system risks", "risk notes", "风险", "risks"]):
        return "global_risks"
    return None


def label_key(label: str) -> str | None:
    """Map localized bullet labels to stable internal keys."""

    clean = _normalize_key(label)
    for key, aliases in NORMALIZED_LABEL_KEYS.items():
        if clean in aliases:
            return key
    if "confidence" in clean or "置信度" in clean:
        return "confidence"
    if "evidence references" in clean or "关键证据引用" in clean:
        return "evidence_refs"
    if clean in {"evidence", "key evidence"} or "关键证据" in clean:
        return "key_evidence"
    if clean in {"role", "function role"} or "功能作用" in clean:
        return "function_role"
    if "capability" in clean or "特殊功能" in clean:
        return "special_capability"
    if "implementation" in clean or "实现" in clean or "approach" in clean:
        return "implementation_idea"
    if "unknown" in clean or "未知" in clean:
        return "unknowns"
    return None


def detail_section_title(title: str) -> str:
    clean = _normalize_key(title)
    for key, aliases in NORMALIZED_DETAIL_SECTION_TITLES.items():
        if clean in aliases:
            mapping = {
                "runtime_control_flow": "Runtime Control Flow",
                "data_flow": "Data Flow",
                "state_lifecycle": "State and Lifecycle",
                "failure_recovery": "Failure and Recovery",
                "concurrency_timing": "Concurrency and Timing",
                "key_evidence": "Key Evidence",
                "unknowns": "Inference and Unknowns",
            }
            return mapping[key]
    return title.strip()


def latest_run_text(raw: str) -> str:
    """Return the latest appended run block from a rendered report."""

    matches = list(RUN_PATTERN.finditer(raw))
    if not matches:
        return raw
    last = matches[-1]
    return raw[last.start() :]


def _normalize_report_v2(report_text: str) -> dict[str, Any]:
    run_text = latest_run_text(report_text)
    h2_blocks = split_by_heading(run_text.splitlines(), H2_PATTERN)
    parts: dict[str, list[str]] = {key: [] for key in FORMAL_REPORT_V2_TITLES.values()}
    for title, body in h2_blocks:
        title = title.strip()
        key = FORMAL_REPORT_V2_TITLES.get(title)
        if key:
            parts[key] = _strip_guidance_lines(body)

    def section_exists(key: str) -> bool:
        return bool(parts.get(key))

    if not any(section_exists(key) for key in parts):
        return {
            "tech_stack": {},
            "project_overview": {},
            "risks": [],
            "characteristics": [],
            "questions": [],
            "evidence_blocks": {},
        }

    tech_fields, tech_notes = _parse_simple_bullets(parts["part_one"])
    project_fields, project_notes = _parse_simple_bullets(parts["part_two"])

    risk_blocks: list[str] = []
    h3_parts = split_by_heading(parts["part_two"], H3_PATTERN)
    for h3_title, h3_body in h3_parts:
        if h3_title.strip() == "风险与局限":
            risk_blocks = h3_body
            break
    _, risk_notes = _parse_simple_bullets(risk_blocks)
    risks = [line.strip()[2:].strip() for line in risk_blocks if line.strip().startswith("- ")]
    if not risks:
        risks = [line.strip() for line in risk_notes if line.strip()]

    characteristics: list[dict[str, Any]] = []
    characteristic_headings = split_by_heading(parts["part_three"], H3_PATTERN)
    for h3_title, h3_body in characteristic_headings:
        match = FORMAL_REPORT_V2_CHARACTERISTIC_PATTERN.match(h3_title.strip())
        if not match:
            continue
        index = int(match.group(1))
        title = match.group(2).strip()
        fields, notes = _parse_simple_bullets(h3_body)
        characteristics.append(
            {
                "index": index,
                "title": title,
                "fields": fields,
                "notes": notes,
            }
        )
    characteristics.sort(key=lambda item: int(item.get("index") or 0))

    questions: list[dict[str, Any]] = []
    question_headings = split_by_heading(parts["part_four"], H3_PATTERN)
    for h3_title, h3_body in question_headings:
        match = FORMAL_REPORT_V2_QUESTION_PATTERN.match(h3_title.strip())
        if not match:
            continue
        index = int(match.group(1))
        title = match.group(2).strip()
        fields, notes = _parse_simple_bullets(h3_body)
        questions.append(
            {
                "index": index,
                "title": title,
                "fields": fields,
                "notes": notes,
            }
        )
    questions.sort(key=lambda item: int(item.get("index") or 0))

    evidence_blocks: dict[str, dict[str, Any]] = {}
    evidence_headings = split_by_heading(parts["part_five"], H3_PATTERN)
    for h3_title, h3_body in evidence_headings:
        stripped = h3_title.strip()
        key = None
        match = FORMAL_REPORT_V2_CHARACTERISTIC_PATTERN.match(stripped)
        if match:
            key = f"characteristic:{int(match.group(1))}"
        else:
            q_match = FORMAL_REPORT_V2_QUESTION_PATTERN.match(stripped)
            if q_match:
                key = f"question:{int(q_match.group(1))}"
        if not key:
            continue
        evidence_blocks[key] = {
            "title": stripped,
            "lines": h3_body,
        }

    return {
        "tech_stack": {
            "fields": tech_fields,
            "notes": tech_notes,
        },
        "project_overview": {
            "fields": project_fields,
            "notes": project_notes,
        },
        "risks": risks,
        "characteristics": characteristics,
        "questions": questions,
        "evidence_blocks": evidence_blocks,
    }


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
                    if key in {"evidence_refs", "key_evidence"}:
                        fields[key] = []
                        current_list_key = key
                    else:
                        fields[key] = value_text
                        current_list_key = None
                    continue
            if current_list_key in {"evidence_refs", "key_evidence"}:
                list_key = "evidence_refs" if current_list_key == "key_evidence" else current_list_key
                refs = fields.setdefault(current_list_key, [])
                refs.append(body.strip().strip("`") )
                if current_list_key != list_key:
                    fields[list_key] = refs
                continue
            free_lines.append(body)
            current_list_key = None
            continue
        free_lines.append(stripped)
    return fields, free_lines


def _split_first_matching(lines: list[str], patterns: tuple[re.Pattern[str], ...]) -> list[tuple[str, list[str]]]:
    for pattern in patterns:
        blocks = split_by_heading(lines, pattern)
        if blocks:
            return blocks
    return []


def _feature_name(title: str) -> str | None:
    match = FEATURE_TITLE_PATTERN.match(title.strip())
    if match:
        return match.group(1).strip()
    clean = title.strip()
    if not clean or section_key(clean) is not None:
        return None
    return clean


def _characteristic_title(title: str) -> str:
    match = CHARACTERISTIC_TITLE_PATTERN.match(title.strip())
    if match:
        return match.group(1).strip()
    return title.strip()


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
        end_line_raw = match.group(3)
        label = (match.group(4) or default_label or "evidence").strip()
        snippet = (match.group(5) or "").strip().strip("`")
        if end_line_raw:
            end_line = int(end_line_raw)
            range_hint = f"lines {line}-{end_line}"
            snippet = f"{range_hint} {snippet}".strip()
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
    if looks_like_formal_report(report_text):
        normalized_v2 = _normalize_report_v2(report_text)
        if any(
            [
                normalized_v2.get("tech_stack", {}).get("fields"),
                normalized_v2.get("project_overview", {}).get("fields"),
                normalized_v2.get("characteristics"),
                normalized_v2.get("questions"),
            ]
        ):
            return normalized_v2

    run_text = latest_run_text(report_text)
    h2_blocks = split_by_heading(run_text.splitlines(), H2_PATTERN)
    parts: dict[str, list[str]] = {"part_one": [], "part_two": [], "part_three": []}
    sections: dict[str, list[str]] = {
        "overview": [],
        "project_characteristics": [],
        "executive_summary": [],
        "feature_details": [],
        "global_risks": [],
    }
    for title, body in h2_blocks:
        key = section_key(title)
        if key in parts:
            parts[key] = body
            continue
        if key in sections:
            sections[key] = body

    part_one = split_by_heading(parts["part_one"], H3_PATTERN)
    part_three = split_by_heading(parts["part_three"], H3_PATTERN)
    if parts["part_two"] and not sections["executive_summary"]:
        sections["executive_summary"] = parts["part_two"]

    for title, body in part_one:
        key = section_key(title)
        if key in {"overview", "project_characteristics"} and not sections[key]:
            sections[key] = body

    for title, body in part_three:
        key = section_key(title)
        if key in {"feature_details", "global_risks"} and not sections[key]:
            sections[key] = body

    feature_summaries: dict[str, dict[str, Any]] = {}
    for title, body in split_by_heading(sections["executive_summary"], H3_PATTERN):
        feature_name = _feature_name(title)
        if not feature_name:
            continue
        fields, free_lines = parse_bullets(body)
        evidence = parse_evidence_lines(body, default_label="feature_summary", origin="report")
        feature_summaries[feature_name] = {
            "title": feature_name,
            "fields": fields,
            "notes": free_lines,
            "evidence": evidence,
        }

    characteristics: list[dict[str, Any]] = []
    characteristic_blocks = _split_first_matching(sections["project_characteristics"], (H4_PATTERN, H3_PATTERN))
    for title, body in characteristic_blocks:
        item_title = _characteristic_title(title)
        fields, free_lines = parse_bullets(body)
        characteristics.append(
            {
                "title": item_title,
                "fields": fields,
                "notes": free_lines,
                "evidence": parse_evidence_lines(body, default_label="project_characteristic", origin="report"),
            }
        )

    overview_body = sections["overview"]
    feature_details_body = sections["feature_details"]

    feature_details: dict[str, dict[str, Any]] = {}
    feature_blocks = _split_first_matching(feature_details_body, (H4_PATTERN, H3_PATTERN))
    for title, body in feature_blocks:
        feature_name = _feature_name(title)
        if not feature_name:
            continue
        subsections = _split_first_matching(body, (H5_PATTERN, H4_PATTERN))
        detail_sections: dict[str, list[str]] = {}
        confidences: list[str] = []
        evidence_items: list[dict[str, Any]] = []
        notes: list[str] = []
        explicit_evidence = False
        fallback_evidence = parse_evidence_lines(body, default_label="feature_detail", origin="report")
        if not subsections and body:
            subsections = [("Summary", body)]
        for subsection_title, subsection_lines in subsections:
            detail_sections[subsection_title] = subsection_lines
            fields, free_lines = parse_bullets(subsection_lines)
            confidence = fields.get("confidence")
            if isinstance(confidence, str):
                confidences.append(confidence)
            subsection_key = label_key(subsection_title)
            if subsection_key == "key_evidence":
                explicit_evidence = True
                evidence_items.extend(parse_evidence_lines(subsection_lines, default_label="feature_detail", origin="report"))
            elif subsection_key == "unknowns":
                notes.extend(free_lines)
            elif subsection_title == "Summary":
                notes.extend(free_lines)
        feature_details[feature_name] = {
            "title": feature_name,
            "sections": detail_sections,
            "confidence": merge_confidence(confidences),
            "evidence": evidence_items if explicit_evidence else fallback_evidence,
            "notes": notes,
        }

    risks_body = sections["global_risks"]
    _, risk_notes = parse_bullets(risks_body)
    risks = [line.strip()[2:].strip() for line in risks_body if line.strip().startswith("- ")]
    if not risks:
        risks = [line.strip() for line in risk_notes if line.strip()]

    return {
        "overview": overview_body,
        "characteristics": characteristics,
        "feature_summaries": feature_summaries,
        "feature_details": feature_details,
        "risks": risks,
    }



def looks_like_formal_report(report_text: str) -> bool:
    normalized = latest_run_text(report_text).casefold()
    hit_count = sum(1 for marker in FORMAL_REPORT_MARKERS if marker in normalized)
    return hit_count >= 3

def render_search_compatible_report(report_text: str) -> str:
    """功能：将当前 report.md 规范化为 search/retrieval 可稳定分块的 markdown。
    参数/返回：接收原始 markdown，返回包含规范 H2/H3/H4 层级的兼容版 markdown。
    失败场景：不抛出业务异常；若无法识别结构则回退为最新 run 的原文。
    副作用：无，仅执行内存内 markdown 解析与重组。
    """

    normalized = _normalize_report(report_text)
    if "tech_stack" in normalized:
        return render_search_compatible_report_v2(normalized)

    if not any(
        [
            normalized["overview"],
            normalized["characteristics"],
            normalized["feature_summaries"],
            normalized["feature_details"],
            normalized["risks"],
        ]
    ):
        return latest_run_text(report_text)

    lines = ["# Repo Library Retrieval Report", ""]

    lines.extend([f"## {SEARCH_SECTION_TITLES['project_characteristics']}", ""])
    for item in normalized["characteristics"]:
        lines.append(f"### {item['title']}")
        fields = item.get("fields", {})
        if fields.get("characteristic_source"):
            lines.append(f"- Source: `{fields['characteristic_source']}`")
        if fields.get("characteristic_signal"):
            lines.append(f"- README Signal: `{fields['characteristic_signal']}`")
        if fields.get("characteristic_mechanism"):
            lines.append(f"- Implementation Mechanism: {fields['characteristic_mechanism']}")
        confidence = fields.get("confidence")
        if confidence:
            lines.append(f"- Confidence: `{confidence}`")
        evidence_items = item.get("evidence", [])
        if evidence_items:
            lines.append("- Key Evidence References:")
            for evidence_item in evidence_items:
                lines.append(f"  - `{evidence_item['source_path']}:{evidence_item['source_line']}`")
        for note in item.get("notes", []):
            if note.strip():
                lines.append(f"- {note.strip()}")
        lines.append("")

    lines.extend([f"## {SEARCH_SECTION_TITLES['executive_summary']}", ""])
    if normalized["overview"]:
        lines.append(f"### {SEARCH_SECTION_TITLES['overview']}")
        for line in normalized["overview"]:
            stripped = line.strip()
            if stripped:
                lines.append(stripped if stripped.startswith("-") else f"- {stripped}")
        lines.append("")

    all_features = sorted(set(normalized["feature_summaries"]) | set(normalized["feature_details"]))
    for feature_name in all_features:
        summary_block = normalized["feature_summaries"].get(feature_name, {})
        lines.append(f"### {feature_name}")
        summary_fields = summary_block.get("fields", {})
        if summary_fields.get("function_role"):
            lines.append(f"- Function Role: {summary_fields['function_role']}")
        if summary_fields.get("special_capability"):
            lines.append(f"- Special Capability: {summary_fields['special_capability']}")
        if summary_fields.get("implementation_idea"):
            lines.append(f"- Implementation Idea: {summary_fields['implementation_idea']}")
        summary_confidence = summary_fields.get("confidence")
        if summary_confidence:
            lines.append(f"- Confidence: `{summary_confidence}`")
        summary_evidence = summary_block.get("evidence", [])
        if summary_evidence:
            lines.append("- Key Evidence References:")
            for evidence_item in summary_evidence:
                lines.append(f"  - `{evidence_item['source_path']}:{evidence_item['source_line']}`")
        for note in summary_block.get("notes", []):
            if note.strip():
                lines.append(f"- {note.strip()}")
        lines.append("")

    lines.extend([f"## {SEARCH_SECTION_TITLES['feature_details']}", ""])
    for feature_name in all_features:
        detail_block = normalized["feature_details"].get(feature_name, {})
        lines.append(f"### {feature_name}")
        detail_sections = detail_block.get("sections", {})
        for section_title, section_lines in detail_sections.items():
            if section_title == "Summary":
                continue
            lines.append(f"#### {detail_section_title(section_title)}")
            for line in section_lines:
                stripped = line.strip()
                if stripped:
                    lines.append(stripped)
            lines.append("")
        detail_evidence = detail_block.get("evidence", [])
        if detail_evidence and not any(label_key(title) == "key_evidence" for title in detail_sections):
            lines.append("#### Key Evidence")
            for evidence_item in detail_evidence:
                lines.append(f"- `{evidence_item['source_path']}:{evidence_item['source_line']}`")
            lines.append("")
        for note in detail_block.get("notes", []):
            if note.strip():
                lines.append(f"- {note.strip()}")
        lines.append("")

    lines.extend([f"## {SEARCH_SECTION_TITLES['global_risks']}", ""])
    for risk in normalized["risks"]:
        if risk.strip():
            lines.append(f"- {risk.strip()}")
    lines.append("")
    return "\n".join(lines).strip() + "\n"


def render_search_compatible_report_v2(normalized: dict[str, Any]) -> str:
    if not isinstance(normalized, dict):
        return ""
    tech_fields = normalized.get("tech_stack", {}).get("fields", {})
    project_fields = normalized.get("project_overview", {}).get("fields", {})
    characteristics = normalized.get("characteristics", [])
    questions = normalized.get("questions", [])
    risks = normalized.get("risks", [])
    evidence_blocks = normalized.get("evidence_blocks", {})

    lines = ["# Repo Library Retrieval Report", ""]

    lines.extend([f"## {SEARCH_SECTION_TITLES['project_characteristics']}", ""])
    for item in characteristics:
        title = str(item.get("title") or "").strip()
        index = int(item.get("index") or 0)
        if not title or index <= 0:
            continue
        lines.append(f"### {title}")
        fields = item.get("fields", {}) if isinstance(item.get("fields"), dict) else {}
        for label in ["动机", "目标", "思路", "取舍"]:
            value = str(fields.get(label) or "").strip()
            if value:
                lines.append(f"- {label}: {value}")
        confidence = str(fields.get("置信度") or "").strip().strip("`")
        if confidence:
            lines.append(f"- Confidence: `{confidence}`")
        evidence_lines = evidence_blocks.get(f"characteristic:{index}", {}).get("lines", [])
        evidence_items = parse_evidence_lines(evidence_lines, default_label="characteristic", origin="report")
        if evidence_items:
            lines.append("- Key Evidence References:")
            for evidence_item in evidence_items:
                lines.append(f"  - `{evidence_item['source_path']}:{evidence_item['source_line']}`")
        lines.append("")

    lines.extend([f"## {SEARCH_SECTION_TITLES['executive_summary']}", ""])
    if tech_fields:
        lines.append("### Tech Stack and Modules")
        for label, value in tech_fields.items():
            text = str(value or "").strip()
            if text:
                lines.append(f"- {label}: {text}")
        lines.append("")

    if project_fields:
        lines.append("### Project Purpose and Core Characteristics")
        for label, value in project_fields.items():
            text = str(value or "").strip()
            if text:
                lines.append(f"- {label}: {text}")
        lines.append("")

    lines.extend([f"## {SEARCH_SECTION_TITLES['feature_details']}", ""])
    for item in questions:
        title = str(item.get("title") or "").strip()
        index = int(item.get("index") or 0)
        if not title or index <= 0:
            continue
        lines.append(f"### {title}")
        fields = item.get("fields", {}) if isinstance(item.get("fields"), dict) else {}
        for label in ["结论", "思路", "取舍", "置信度"]:
            value = str(fields.get(label) or "").strip()
            if value:
                if label == "置信度":
                    lines.append(f"- Confidence: `{value.strip('`')}`")
                else:
                    lines.append(f"- {label}: {value}")
        evidence_lines = evidence_blocks.get(f"question:{index}", {}).get("lines", [])
        evidence_items = parse_evidence_lines(evidence_lines, default_label="question", origin="report")
        if evidence_items:
            lines.append("- Key Evidence References:")
            for evidence_item in evidence_items:
                lines.append(f"  - `{evidence_item['source_path']}:{evidence_item['source_line']}`")
        lines.append("")

    lines.extend([f"## {SEARCH_SECTION_TITLES['global_risks']}", ""])
    for risk in risks:
        if str(risk).strip():
            lines.append(f"- {str(risk).strip()}")
    lines.append("")
    return "\n".join(lines).strip() + "\n"


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

    def normalize_confidence(value: Any, *, fallback: str = "low") -> str:
        text = str(value or "").strip().strip("`").casefold()
        if text in {"high", "medium", "low"}:
            return text
        return fallback

    if "tech_stack" in normalized:
        tech_fields = normalized.get("tech_stack", {}).get("fields", {})
        project_fields = normalized.get("project_overview", {}).get("fields", {})
        evidence_blocks = normalized.get("evidence_blocks", {})

        if isinstance(tech_fields, dict) and tech_fields:
            summary = str(
                tech_fields.get("主要语言/技术栈总览")
                or tech_fields.get("后端")
                or tech_fields.get("前端")
                or "技术栈与模块语言"
            ).strip()
            content_lines = [
                f"{label}: {tech_fields[label]}"
                for label in [
                    "主要语言/技术栈总览",
                    "后端",
                    "前端",
                    "其它模块",
                ]
                if str(tech_fields.get(label) or "").strip()
            ]
            _append_card(
                cards=cards,
                evidence=evidence,
                snapshot_id=snapshot_id,
                repo_key=resolved_repo_key,
                run_id=run_id,
                card_type="integration_note",
                title="技术栈与模块语言",
                summary=summary,
                content="\n".join(content_lines) if content_lines else summary,
                confidence="medium",
                tags=["tech_stack"],
                source="report.md",
                evidence_items=[],
            )

        if isinstance(project_fields, dict) and project_fields:
            summary = str(project_fields.get("项目做什么用") or project_fields.get("核心特点概览") or "项目用途与核心特点").strip()
            content_lines = [
                f"{label}: {project_fields[label]}"
                for label in [
                    "项目做什么用",
                    "典型使用场景",
                    "核心特点概览",
                ]
                if str(project_fields.get(label) or "").strip()
            ]
            _append_card(
                cards=cards,
                evidence=evidence,
                snapshot_id=snapshot_id,
                repo_key=resolved_repo_key,
                run_id=run_id,
                card_type="integration_note",
                title="项目用途与核心特点",
                summary=summary,
                content="\n".join(content_lines) if content_lines else summary,
                confidence="medium",
                tags=["project_overview"],
                source="report.md",
                evidence_items=[],
            )

        for item in normalized.get("characteristics", []) or []:
            if not isinstance(item, dict):
                continue
            title = str(item.get("title") or "").strip()
            index = int(item.get("index") or 0)
            if not title or index <= 0:
                continue
            fields = item.get("fields", {}) if isinstance(item.get("fields"), dict) else {}
            summary = str(fields.get("思路") or fields.get("目标") or title).strip()
            content_parts = []
            for label in ["动机", "目标", "思路", "取舍"]:
                value = str(fields.get(label) or "").strip()
                if value:
                    content_parts.append(f"{label}: {value}")
            notes = item.get("notes", [])
            if isinstance(notes, list):
                content_parts.extend([str(line).strip() for line in notes if str(line).strip()])
            evidence_lines = evidence_blocks.get(f"characteristic:{index}", {}).get("lines", [])
            evidence_items = parse_evidence_lines(evidence_lines, default_label="characteristic", origin="report")
            confidence = normalize_confidence(fields.get("置信度"), fallback="medium" if evidence_items else "low")
            _append_card(
                cards=cards,
                evidence=evidence,
                snapshot_id=snapshot_id,
                repo_key=resolved_repo_key,
                run_id=run_id,
                card_type="project_characteristic",
                title=title,
                summary=summary,
                content="\n".join(content_parts) if content_parts else summary,
                confidence=confidence,
                tags=["characteristic", title],
                source="report.md",
                evidence_items=evidence_items,
            )

        for item in normalized.get("questions", []) or []:
            if not isinstance(item, dict):
                continue
            title = str(item.get("title") or "").strip()
            index = int(item.get("index") or 0)
            if not title or index <= 0:
                continue
            fields = item.get("fields", {}) if isinstance(item.get("fields"), dict) else {}
            summary = str(fields.get("结论") or title).strip()
            content_parts = []
            for label in ["结论", "思路", "取舍"]:
                value = str(fields.get(label) or "").strip()
                if value:
                    content_parts.append(f"{label}: {value}")
            notes = item.get("notes", [])
            if isinstance(notes, list):
                content_parts.extend([str(line).strip() for line in notes if str(line).strip()])
            evidence_lines = evidence_blocks.get(f"question:{index}", {}).get("lines", [])
            evidence_items = parse_evidence_lines(evidence_lines, default_label="question", origin="report")
            confidence = normalize_confidence(fields.get("置信度"), fallback="medium" if evidence_items else "low")
            _append_card(
                cards=cards,
                evidence=evidence,
                snapshot_id=snapshot_id,
                repo_key=resolved_repo_key,
                run_id=run_id,
                card_type="feature_pattern",
                title=title,
                summary=summary,
                content="\n".join(content_parts) if content_parts else summary,
                confidence=confidence,
                tags=["question", title],
                source="report.md",
                evidence_items=evidence_items,
            )

        # Risks are already covered in the "项目用途与核心特点" report section shown as integration notes.
        # Avoid generating one-line, evidence-less cards that pollute the card list.

    else:

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

        # Risks are already covered in the "项目用途与核心特点" report section shown as integration notes.
        # Avoid generating one-line, evidence-less cards that pollute the card list.

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
