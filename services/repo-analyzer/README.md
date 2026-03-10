# Repo Analyzer

Repo Library 的 Python 命令式引擎，提供单一 CLI 入口并复用现有 `github-feature-analyzer` 脚本。

## Commands

- `prepare`：准备 snapshot/source 与 `code_index.json`，为后续 AI 聊天生成 `report.md` 预留路径。
- `pipeline`：顺序执行 `ingest`、`extract-cards` 和搜索索引刷新。
- `ingest`：准备存储目录、抓取仓库、构建代码索引、渲染 `report.md`。
- `extract-cards`：把 `report.md` 与可选 `subagent_results.json` 抽取成 cards/evidence JSON。
- `validate-report`：校验正式报告的标题层级、feature 映射、表格要求、证据格式，以及是否至少能抽出 1 张卡片。
- `search`：同步 snapshot 语料、刷新向量索引，或执行归一化搜索查询。

## Example

```bash
python3 services/repo-analyzer/app/cli.py pipeline \
  --repo-url https://github.com/octocat/Hello-World \
  --ref main \
  --feature "routing" \
  --storage-root /tmp/repo-library \
  --run-id demo-run \
  --snapshot-dir /tmp/repo-library/snapshots/demo-run \
  --output /tmp/repo-library/pipeline.json
```

输出 JSON 会包含稳定字段：`repo_path`、`snapshot_path`、`run_path`、`report_path`、`resolved_ref`、`commit_sha`、`card_count`、`evidence_count` 与 `search_refresh`。当提供 `--snapshot-dir` 时，源码、artifacts、report 与派生文件会直接写入该绝对路径。

`prepare` 命令会返回同一组路径字段，但 `report_path` 仅表示后续 AI 生成报告的目标位置，此时 `report_ready=false`。
