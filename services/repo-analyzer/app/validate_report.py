#!/usr/bin/env python3
"""Validate Repo Library formal reports before they become official results."""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any

from extract_cards import build_cards_payload
from helpers import ENGINE_VERSION, EngineError, now_iso

HEADING_PATTERN = re.compile(r"^(#{1,6})\s+(.+?)\s*$")
FILE_REF_PATTERN = re.compile(r"`?([A-Za-z0-9_./-]+\.[A-Za-z0-9_+.-]+):(\d+)(?:-(\d+))?`?")
CHARACTERISTIC_PATTERN = re.compile(r"^特点\s+(\d+)\s*[:：]\s*(.+?)\s*$")
QUESTION_PATTERN = re.compile(r"^问题\s+(\d+)\s*[:：]\s*(.+?)\s*$")
LEADING_NUMBER_PREFIX = re.compile(r"^\s*\d+\s*[\.、:：)）]\s*|^\s*\d+\s+")

PART_ONE = "第一部分：技术栈与模块语言"
PART_TWO = "第二部分：项目用途与核心特点"
PART_THREE = "第三部分：特点实现思路"
PART_FOUR = "第四部分：提问与解答"
PART_FIVE = "第五部分：实现定位与证据"
RISK_SECTION = "风险与局限"

PART_ONE_REQUIRED_BULLETS = [
    "仓库",
    "请求 Ref",
    "解析 Ref",
    "Commit",
    "生成时间",
    "主要语言/技术栈总览",
    "后端",
    "前端",
    "其它模块",
]

PART_TWO_REQUIRED_BULLETS = [
    "项目做什么用",
    "典型使用场景",
    "核心特点概览",
]

CHARACTERISTIC_REQUIRED_BULLETS = [
    "动机",
    "目标",
    "思路",
    "取舍",
    "置信度",
]

QUESTION_REQUIRED_BULLETS = [
    "结论",
    "思路",
    "取舍",
    "置信度",
]

VALID_CONFIDENCE = {"high", "medium", "low"}


def normalize_question_title(value: str) -> str:
    trimmed = value.strip()
    trimmed = LEADING_NUMBER_PREFIX.sub("", trimmed)
    return trimmed.strip()


def parse_headings(lines: list[str]) -> list[dict[str, Any]]:
    headings: list[dict[str, Any]] = []
    for idx, line in enumerate(lines):
        match = HEADING_PATTERN.match(line.strip())
        if not match:
            continue
        headings.append({"level": len(match.group(1)), "title": match.group(2).strip(), "line": idx})
    return headings


def find_heading(headings: list[dict[str, Any]], level: int, title: str) -> dict[str, Any] | None:
    for item in headings:
        if item["level"] == level and item["title"] == title:
            return item
    return None


def block_lines(lines: list[str], start: int, end: int) -> list[str]:
    return lines[start:end]


def next_heading_line(headings: list[dict[str, Any]], start_line: int, *, min_level: int = 1) -> int | None:
    for item in headings:
        if item["line"] > start_line and item["level"] <= min_level:
            return item["line"]
    return None


def next_heading_any_level(headings: list[dict[str, Any]], start_line: int) -> int | None:
    for item in headings:
        if item["line"] > start_line:
            return item["line"]
    return None


def extract_section_block(
    lines: list[str],
    headings: list[dict[str, Any]],
    heading: dict[str, Any],
    boundary_line: int | None = None,
) -> list[str]:
    if boundary_line is None:
        boundary_line = next_heading_any_level(headings, heading["line"]) or len(lines)
    return block_lines(lines, heading["line"] + 1, boundary_line)


def has_bullet_prefix(block: list[str], label: str) -> bool:
    return bullet_value(block, label) is not None


def bullet_value(block: list[str], label: str) -> str | None:
    for line in block:
        stripped = line.strip()
        if not stripped.startswith("- "):
            continue
        body = stripped[2:].strip()
        prefix_variants = (f"{label}:", f"{label}：")
        for prefix in prefix_variants:
            if body.startswith(prefix):
                return body[len(prefix) :].strip()
    return None


def extract_matching_refs(block: list[str]) -> list[str]:
    refs: list[str] = []
    for line in block:
        refs.extend(match.group(0) for match in FILE_REF_PATTERN.finditer(line))
    return refs
def validate_report_structure(report_text: str, features: list[str]) -> tuple[list[str], dict[str, Any]]:
    errors: list[str] = []
    lines = report_text.splitlines()
    headings = parse_headings(lines)
    stats: dict[str, Any] = {
        "feature_count_expected": len(features),
        "feature_count_found": 0,
        "table_count": 0,
    }

    if "```" in report_text:
        errors.append("正式报告正文禁止包含 Markdown 代码围栏（```）。")

    if not headings:
        return ["缺少 Markdown 标题结构。"], stats

    first_line = lines[0].lstrip("\ufeff").strip()
    if first_line != "# GitHub 功能实现原理报告":
        errors.append("第一行必须直接是 `# GitHub 功能实现原理报告`。")

    required_h2_titles = [
        "Run 1",
        PART_ONE,
        PART_TWO,
        PART_THREE,
        PART_FOUR,
        PART_FIVE,
    ]
    positions: dict[str, int] = {}
    for title in required_h2_titles:
        item = find_heading(headings, 2, title)
        if item is None:
            errors.append(f"缺少 `## {title}` 标题。")
            continue
        positions[title] = item["line"]

    if len(positions) == len(required_h2_titles):
        ordered = [positions[title] for title in required_h2_titles]
        if ordered != sorted(ordered):
            errors.append("核心 H2 标题顺序不正确。")

    part_one_heading = find_heading(headings, 2, PART_ONE)
    part_two_heading = find_heading(headings, 2, PART_TWO)
    part_three_heading = find_heading(headings, 2, PART_THREE)
    part_four_heading = find_heading(headings, 2, PART_FOUR)
    part_five_heading = find_heading(headings, 2, PART_FIVE)

    def extract_part_block(part_heading: dict[str, Any] | None, next_part_heading: dict[str, Any] | None) -> list[str]:
        if part_heading is None:
            return []
        boundary = next_part_heading["line"] if next_part_heading else None
        return extract_section_block(lines, headings, part_heading, boundary)

    part_one_block = extract_part_block(part_one_heading, part_two_heading)
    part_two_block = extract_part_block(part_two_heading, part_three_heading)
    part_three_block = extract_part_block(part_three_heading, part_four_heading)
    part_four_block = extract_part_block(part_four_heading, part_five_heading)

    for label in PART_ONE_REQUIRED_BULLETS:
        if part_one_heading and not has_bullet_prefix(part_one_block, label):
            errors.append(f"`## {PART_ONE}` 缺少 `- {label}:` 字段。")

    for label in PART_TWO_REQUIRED_BULLETS:
        if part_two_heading and not has_bullet_prefix(part_two_block, label):
            errors.append(f"`## {PART_TWO}` 缺少 `- {label}:` 字段。")

    if part_two_heading:
        risk_heading = find_heading(headings, 3, RISK_SECTION)
        if risk_heading is None or not (part_two_heading["line"] < risk_heading["line"] < (part_three_heading["line"] if part_three_heading else len(lines))):
            errors.append(f"`## {PART_TWO}` 缺少 `### {RISK_SECTION}` 小节。")

    for idx, part_block in enumerate([part_one_block, part_two_block, part_three_block, part_four_block], start=1):
        refs = extract_matching_refs(part_block)
        if refs:
            errors.append(f"第一到第四部分禁止出现 `file:line` 引用（检测到 {len(refs)} 处）。")
            break

    characteristics: list[tuple[int, str, dict[str, Any]]] = []
    if part_three_heading and part_four_heading:
        characteristic_headings = [
            item
            for item in headings
            if item["level"] == 3 and part_three_heading["line"] < item["line"] < part_four_heading["line"] and CHARACTERISTIC_PATTERN.match(item["title"])
        ]
        characteristic_headings.sort(key=lambda item: item["line"])
        if not characteristic_headings:
            errors.append(f"`## {PART_THREE}` 至少需要 1 个 `### 特点 1: ...`。")
        for item in characteristic_headings:
            match = CHARACTERISTIC_PATTERN.match(item["title"])
            if not match:
                continue
            index = int(match.group(1))
            title = match.group(2).strip()
            characteristics.append((index, title, item))
        if characteristics:
            indices = [index for index, _, _ in characteristics]
            expected = list(range(1, len(indices) + 1))
            if indices != expected:
                errors.append(f"`## {PART_THREE}` 的特点编号必须从 1 开始连续递增（当前: {indices}）。")

        for position, (index, title, heading) in enumerate(characteristics):
            next_line = characteristic_headings[position + 1]["line"] if position + 1 < len(characteristic_headings) else part_four_heading["line"]
            block = extract_section_block(lines, headings, heading, next_line)
            for label in CHARACTERISTIC_REQUIRED_BULLETS:
                if not has_bullet_prefix(block, label):
                    errors.append(f"`特点 {index}: {title}` 缺少 `- {label}:` 字段。")
            confidence = bullet_value(block, "置信度")
            if confidence is None or confidence.casefold() not in VALID_CONFIDENCE:
                errors.append(f"`特点 {index}: {title}` 的 `- 置信度:` 必须是 high|medium|low。")

    normalized_features = [normalize_question_title(item) for item in features if item.strip()]
    questions: list[tuple[int, str, dict[str, Any]]] = []
    if part_four_heading and part_five_heading:
        question_headings = [
            item
            for item in headings
            if item["level"] == 3 and part_four_heading["line"] < item["line"] < part_five_heading["line"] and QUESTION_PATTERN.match(item["title"])
        ]
        question_headings.sort(key=lambda item: item["line"])
        stats["feature_count_found"] = len(question_headings)
        if len(question_headings) != len(normalized_features):
            errors.append(f"`## {PART_FOUR}` 问题数量不匹配：期望 {len(normalized_features)} 个，实际 {len(question_headings)} 个。")
        for item in question_headings:
            match = QUESTION_PATTERN.match(item["title"])
            if not match:
                continue
            index = int(match.group(1))
            title = match.group(2).strip()
            questions.append((index, title, item))
        if questions:
            indices = [index for index, _, _ in questions]
            expected = list(range(1, len(indices) + 1))
            if indices != expected:
                errors.append(f"`## {PART_FOUR}` 的问题编号必须从 1 开始连续递增（当前: {indices}）。")
        for index, expected_title in enumerate(normalized_features, start=1):
            heading = next((item for item in question_headings if item["title"].strip().endswith(expected_title)), None)
            if heading is None:
                errors.append(f"`## {PART_FOUR}` 缺少 `### 问题 {index}: {expected_title}`。")
                continue
        for position, (index, title, heading) in enumerate(questions):
            next_line = question_headings[position + 1]["line"] if position + 1 < len(question_headings) else part_five_heading["line"]
            block = extract_section_block(lines, headings, heading, next_line)
            for label in QUESTION_REQUIRED_BULLETS:
                if not has_bullet_prefix(block, label):
                    errors.append(f"`问题 {index}: {title}` 缺少 `- {label}:` 字段。")
            confidence = bullet_value(block, "置信度")
            if confidence is None or confidence.casefold() not in VALID_CONFIDENCE:
                errors.append(f"`问题 {index}: {title}` 的 `- 置信度:` 必须是 high|medium|low。")

    if part_five_heading:
        part_five_end = len(lines)
        part_five_block = extract_section_block(lines, headings, part_five_heading, part_five_end)
        if not extract_matching_refs(part_five_block):
            errors.append(f"`## {PART_FIVE}` 必须包含 `file:line` 证据引用。")

        evidence_headings = [
            item
            for item in headings
            if item["level"] == 3 and part_five_heading["line"] < item["line"] < part_five_end and (CHARACTERISTIC_PATTERN.match(item["title"]) or QUESTION_PATTERN.match(item["title"]))
        ]
        evidence_headings.sort(key=lambda item: item["line"])

        def evidence_block_for(heading: dict[str, Any]) -> list[str]:
            next_line = next(
                (item["line"] for item in evidence_headings if item["line"] > heading["line"]),
                part_five_end,
            )
            return extract_section_block(lines, headings, heading, next_line)

        for index, title, _ in characteristics:
            heading = next(
                (
                    item
                    for item in evidence_headings
                    if (match := CHARACTERISTIC_PATTERN.match(item["title"])) and int(match.group(1)) == index and match.group(2).strip() == title
                ),
                None,
            )
            if heading is None:
                errors.append(f"`## {PART_FIVE}` 缺少 `### 特点 {index}: {title}`。")
                continue
            block = evidence_block_for(heading)
            if not extract_matching_refs(block):
                errors.append(f"`特点 {index}: {title}` 在第五部分缺少合法的 `file:line` 证据引用。")

        for index, expected_title in enumerate(normalized_features, start=1):
            heading = next(
                (
                    item
                    for item in evidence_headings
                    if (match := QUESTION_PATTERN.match(item["title"])) and int(match.group(1)) == index and match.group(2).strip() == expected_title
                ),
                None,
            )
            if heading is None:
                errors.append(f"`## {PART_FIVE}` 缺少 `### 问题 {index}: {expected_title}`。")
                continue
            block = evidence_block_for(heading)
            if not extract_matching_refs(block):
                errors.append(f"`问题 {index}: {expected_title}` 在第五部分缺少合法的 `file:line` 证据引用。")

    return errors, stats


def validate_report_payload(
    *,
    report_path: Path,
    output_path: Path,
    features: list[str],
    repo_url: str | None = None,
    repo_key: str | None = None,
    snapshot_id: str | None = None,
    snapshot_dir: Path | None = None,
    run_id: str | None = None,
) -> dict[str, Any]:
    if not report_path.exists() or not report_path.is_file():
        raise EngineError(f"report_path does not exist: {report_path}")
    report_text = report_path.read_text(encoding="utf-8", errors="ignore")
    feature_list = [item.strip() for item in features if item and item.strip()]
    errors, stats = validate_report_structure(report_text, feature_list)
    cards_payload = build_cards_payload(
        report_path=report_path,
        output_path=output_path.with_name(output_path.stem + ".cards-scratch.json"),
        repo_url=repo_url,
        repo_key=repo_key,
        snapshot_id=snapshot_id,
        snapshot_dir=snapshot_dir,
        run_id=run_id,
    )
    card_count = int(cards_payload.get("card_count") or 0)
    evidence_count = int(cards_payload.get("evidence_count") or 0)
    if card_count <= 0:
        errors.append("报告未通过可抽卡校验：`build_cards_payload(...)` 未抽出任何知识卡片。")
    payload = {
        "status": "ok",
        "command": "validate-report",
        "engine_version": ENGINE_VERSION,
        "generated_at": now_iso(),
        "valid": len(errors) == 0,
        "errors": errors,
        "warnings": [],
        "report_path": str(report_path),
        "feature_count_expected": stats["feature_count_expected"],
        "feature_count_found": stats["feature_count_found"],
        "table_count": stats["table_count"],
        "card_count": card_count,
        "evidence_count": evidence_count,
        "type_counts": cards_payload.get("type_counts") or {},
    }
    return payload
