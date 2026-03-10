import sys
import tempfile
import unittest
from pathlib import Path

APP_DIR = Path(__file__).resolve().parents[1] / "app"
if str(APP_DIR) not in sys.path:
    sys.path.insert(0, str(APP_DIR))

from validate_report import validate_report_payload


FEATURE_1_RAW = "1. Repo Library 的正式报告格式如何保证"
FEATURE_1 = "Repo Library 的正式报告格式如何保证"
FEATURE_2_RAW = "2、为什么报告里不要出现 file:line（直到最后一部分）"
FEATURE_2 = "为什么报告里不要出现 file:line（直到最后一部分）"


VALID_REPORT = f"""# GitHub 功能实现原理报告

## Run 1

## 第一部分：技术栈与模块语言
- 仓库: https://github.com/example/repo
- 请求 Ref: main
- 解析 Ref: main
- Commit: abc123
- 生成时间: 2026-03-10
- 主要语言/技术栈总览: Go + React + Python 三段式
- 后端: Go (Gin/SQLite)；daemon 提供 Repo Library API
- 前端: TypeScript + React；Repo Library 列表与详情页
- 其它模块: Python analyzer：抽卡、校验、搜索

## 第二部分：项目用途与核心特点
- 项目做什么用: 对真实仓库做自动分析，产出结构化报告与知识卡片。
- 典型使用场景: 选择一个仓库 + 关注点，生成可检索的“实现思路+证据定位”。
- 核心特点概览: 报告强校验、失败自动重试、抽卡与搜索统一入口。

### 风险与局限
- 校验规则过严可能导致重试次数增加。
- 证据引用格式不一致会降低卡片可用性。
- 报告结构变更需要同步更新抽卡与搜索规范化。

## 第三部分：特点实现思路

### 特点 1: 强制结构化报告
- 动机: 报告需要被程序解析，避免“看起来有内容但结构不可用”。
- 目标: 让每次分析都能产出可抽卡、可检索、可定位的稳定结构。
- 思路: 用固定模板限制标题和字段，再用脚本做机器校验。
- 取舍: 模板更严格，AI 需要额外重写成本。
- 置信度: high

### 特点 2: 校验失败自动重试
- 动机: 单次生成不稳定，需要闭环保证最终结果可用。
- 目标: 在预算内自动得到合格报告。
- 思路: 将校验错误回灌给 AI，让其完整重写直至通过。
- 取舍: 会增加耗时，并可能需要限制重试上限。
- 置信度: medium

## 第四部分：提问与解答

### 问题 1: {FEATURE_1}
- 结论: 通过固定模板 + 脚本校验 + 自动重试来保证。
- 思路: 先强约束标题与字段，再校验缺失项与违规项，失败即重写。
- 取舍: 规则越硬，模型越容易犯格式错误但最终更可用。
- 置信度: high

### 问题 2: {FEATURE_2}
- 结论: 前四部分主要是逻辑与思路；证据集中在第五部分便于抽取与检索。
- 思路: 让阅读部分更顺滑，同时确保定位信息被统一结构承载。
- 取舍: 前四部分无法直接跳到代码，需要结合第五部分。
- 置信度: medium

## 第五部分：实现定位与证据

### 特点 1: 强制结构化报告
- services/repo-analyzer/app/validate_report.py:120 [validator] - required headings and fields

### 特点 2: 校验失败自动重试
- backend/internal/repolib/report_validation.go:80 [retry_loop] - retry report generation

### 问题 1: {FEATURE_1}
- services/repo-analyzer/app/cli.py:60 [cli] - validate-report command

### 问题 2: {FEATURE_2}
- backend/internal/repolib/prompts.go:90 [prompt] - forbid file:line before part five
"""


INVALID_REPORT_HAS_EVIDENCE_EARLY = VALID_REPORT.replace(
    "- 思路: 用固定模板限制标题和字段，再用脚本做机器校验。",
    "- 思路: 用固定模板限制标题和字段（见 services/repo-analyzer/app/cli.py:12），再用脚本做机器校验。",
)


INVALID_REPORT_WRONG_EVIDENCE_TITLE = VALID_REPORT.replace(
    f"### 问题 1: {FEATURE_1}",
    f"### 问题 1: {FEATURE_1_RAW}",
)


class ValidateReportTest(unittest.TestCase):
    def test_validate_report_accepts_canonical_report(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            report_path = root / "report.md"
            output_path = root / "validation.json"
            report_path.write_text(VALID_REPORT, encoding="utf-8")

            payload = validate_report_payload(
                report_path=report_path,
                output_path=output_path,
                features=[FEATURE_1_RAW, FEATURE_2_RAW],
                repo_key="example-repo",
                snapshot_id="rp_demo",
                run_id="rr_demo",
            )

            self.assertEqual(payload["status"], "ok")
            self.assertTrue(payload["valid"], msg=str(payload.get("errors")))
            self.assertGreater(payload["card_count"], 0)

    def test_validate_report_rejects_file_refs_before_part_five(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            report_path = root / "report.md"
            output_path = root / "validation.json"
            report_path.write_text(INVALID_REPORT_HAS_EVIDENCE_EARLY, encoding="utf-8")

            payload = validate_report_payload(
                report_path=report_path,
                output_path=output_path,
                features=[FEATURE_1_RAW, FEATURE_2_RAW],
                repo_key="example-repo",
                snapshot_id="rp_demo",
                run_id="rr_demo",
            )

            self.assertFalse(payload["valid"])
            self.assertTrue(any("禁止出现 `file:line`" in item for item in payload["errors"]))

    def test_validate_report_requires_normalized_question_titles(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            report_path = root / "report.md"
            output_path = root / "validation.json"
            report_path.write_text(INVALID_REPORT_WRONG_EVIDENCE_TITLE, encoding="utf-8")

            payload = validate_report_payload(
                report_path=report_path,
                output_path=output_path,
                features=[FEATURE_1_RAW, FEATURE_2_RAW],
                repo_key="example-repo",
                snapshot_id="rp_demo",
                run_id="rr_demo",
            )

            self.assertFalse(payload["valid"])
            self.assertTrue(any("缺少 `### 问题 1" in item for item in payload["errors"]))


if __name__ == "__main__":
    unittest.main()

