import sys
import tempfile
import unittest
from pathlib import Path

APP_DIR = Path(__file__).resolve().parents[1] / "app"
if str(APP_DIR) not in sys.path:
    sys.path.insert(0, str(APP_DIR))

from extract_cards import build_cards_payload
from search import normalize_query_results, sync_snapshot_to_corpus


SAMPLE_REPORT = """# GitHub Feature Principle Report

## Run 1

## 第一部分：项目参数与结构解析

### 仓库结构心智模型
- 文件总数: `12`
- 可检索文本文件: `10`
- 运行入口线索: `cmd/main.go`
- 模块边界线索: `internal/api, internal/store`

### 项目特点与标志实现

#### 项目特点 1: CLI 驱动分析
- 来源: `README.md`
- README 线索: `feature analyzer`
- 实现机制: 通过统一 CLI 调度抓仓、索引和报告生成。
- 置信度: `high`
- 关键证据引用:
  - `app/cli.py:42`

## 第二部分：面向人的功能说明

### 功能 1: 搜索刷新
- 功能作用: 刷新搜索索引并返回命中结果。
- 特殊功能: 支持 repo 过滤。
- 实现想法: 先 build，再 query，并做结果归一化。
- 置信度: `medium`
- 关键证据引用:
  - `app/search.py:88`

## 第三部分：面向 AI 的实现细节与证据链

### 面向 AI 的实现细节

#### 功能 1: 搜索刷新

##### 运行时控制流
- 入口先同步 corpus，再调用 reference_retrieval build/query。
- 置信度: `high`
- inference: `false`

##### 关键证据
- `app/search.py:88` [runtime_control_flow] - `run build before query`

##### 推断与未知点
- 无

### 跨功能耦合与系统风险
- 向量依赖缺失会导致搜索无法刷新。
"""

SAMPLE_SUBAGENT = {
    "overview": {
        "summary": "该仓库通过 CLI 统一串起知识处理流程。",
        "confidence": "medium",
        "evidence": [
            {
                "path": "app/cli.py",
                "line": 12,
                "snippet": "统一 CLI 入口",
                "for_dimension": "overview",
            }
        ],
    },
    "architecture": {
        "summary": "ingest、extract、search 三段职责清晰。",
        "confidence": "high",
        "evidence": [
            {
                "path": "app/search.py",
                "line": 40,
                "snippet": "sync snapshot to corpus",
                "for_dimension": "architecture",
            }
        ],
    },
}

AI_REPORT = """# Repo Analysis

## Repository Overview
- CLI 串起 prepare、report 和 search 三段分析流。

## Key Characteristics

### CLI-driven pipeline
- Source: `README.md`
- README Signal: `repo analyzer`
- Implementation: 通过统一 CLI 先准备源码，再消费 AI 报告。
- Confidence: `high`
- Key Evidence References:
  - `app/cli.py:21`

## Features

### Search refresh
- Role: 刷新向量索引并返回命中结果。
- Capability: 支持 repo 过滤。
- Implementation: 先同步 corpus，再 build/query 并做结果归一化。
- Confidence: `medium`
- Key Evidence References:
  - `app/search.py:63`

## AI Details

### Search refresh

#### Runtime Control Flow
- 入口先同步 corpus，再调用 reference retrieval build/query。
- Confidence: `high`

#### Key Evidence
- `app/search.py:63` [runtime_control_flow] - `write normalized report into corpus`

#### Inference and Unknowns
- None

## Risks
- 缺少 embeddings 依赖会导致检索刷新失败。
"""


class ExtractCardsTest(unittest.TestCase):
    def test_build_cards_payload_extracts_expected_card_types(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            report_path = root / "report.md"
            subagent_path = root / "subagent_results.json"
            output_path = root / "cards.json"
            report_path.write_text(SAMPLE_REPORT, encoding="utf-8")
            subagent_path.write_text(__import__("json").dumps(SAMPLE_SUBAGENT, ensure_ascii=False), encoding="utf-8")

            payload = build_cards_payload(
                report_path=report_path,
                output_path=output_path,
                repo_key="octocat-Hello-World",
                snapshot_id="sha-abcdef123456",
                run_id="demo-run",
                subagent_results_path=subagent_path,
            )

            self.assertEqual(payload["status"], "ok")
            self.assertGreaterEqual(payload["card_count"], 5)
            self.assertIn("project_characteristic", payload["type_counts"])
            self.assertIn("feature_pattern", payload["type_counts"])
            self.assertIn("risk_note", payload["type_counts"])
            self.assertIn("integration_note", payload["type_counts"])
            self.assertGreater(payload["evidence_count"], 0)

    def test_normalize_query_results_maps_corpus_metadata(self) -> None:
        raw_hits = [
            {
                "score": 0.92,
                "repo": "octocat-Hello-World--sha-abcdef123456",
                "chunk_id": "chunk-1",
                "source_file": "report.md",
                "section_type": "report.feature_analysis",
                "section_title": "Feature 1: 搜索刷新",
                "evidence_refs": ["app/search.py:88"],
                "text_excerpt": "refresh index first",
                "text": "refresh index first and then query",
            }
        ]
        corpus_meta = {
            "octocat-Hello-World--sha-abcdef123456": {
                "repo": {"repo_key": "octocat-Hello-World", "repo_url": "https://github.com/octocat/Hello-World"},
                "snapshot": {"snapshot_id": "sha-abcdef123456", "path": "/tmp/snapshot", "report_path": "/tmp/snapshot/report.md"},
                "run": {"run_id": "demo-run"},
            }
        }

        results = normalize_query_results(raw_hits=raw_hits, corpus_meta=corpus_meta, limit=10)
        self.assertEqual(len(results), 1)
        self.assertEqual(results[0]["repository"]["repo_key"], "octocat-Hello-World")
        self.assertEqual(results[0]["snapshot"]["snapshot_id"], "sha-abcdef123456")
        self.assertEqual(results[0]["run"]["run_id"], "demo-run")

    def test_build_cards_payload_accepts_ai_generated_markdown(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            report_path = root / "report.md"
            output_path = root / "cards.json"
            report_path.write_text(AI_REPORT, encoding="utf-8")

            payload = build_cards_payload(
                report_path=report_path,
                output_path=output_path,
                repo_key="octocat-Hello-World",
                snapshot_id="sha-abcdef123456",
                run_id="demo-run",
            )

            self.assertEqual(payload["status"], "ok")
            self.assertIn("project_characteristic", payload["type_counts"])
            self.assertIn("feature_pattern", payload["type_counts"])
            self.assertIn("risk_note", payload["type_counts"])

    def test_sync_snapshot_to_corpus_normalizes_ai_report_headings(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            report_path = root / "report.md"
            report_path.write_text(AI_REPORT, encoding="utf-8")

            sync_summary = sync_snapshot_to_corpus(
                storage_root=root,
                report_path=report_path,
                repo_key="octocat-Hello-World",
                snapshot_id="sha-abcdef123456",
                snapshot_dir=root / "snapshot",
                run_id="demo-run",
            )

            self.assertIsNotNone(sync_summary)
            corpus_report = Path(sync_summary["corpus_dir"]) / "report.md"
            corpus_text = corpus_report.read_text(encoding="utf-8")
            self.assertIn("## Project Characteristics and Signature Implementations", corpus_text)
            self.assertIn("## Executive Principle Summary", corpus_text)
            self.assertIn("## Feature Principle Analysis", corpus_text)
            self.assertIn("## Cross-feature Coupling and System Risks", corpus_text)
            self.assertIn("### Search refresh", corpus_text)


if __name__ == "__main__":
    unittest.main()
