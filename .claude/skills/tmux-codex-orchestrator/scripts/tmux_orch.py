#!/usr/bin/env python3
"""tmux-codex-orchestrator command line helper."""

from __future__ import annotations

import argparse
import json
import random
import re
import shlex
import subprocess
import sys
import textwrap
import time
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

SCRIPT_DIR = Path(__file__).resolve().parent
SKILL_DIR = SCRIPT_DIR.parent
REPO_ROOT = SCRIPT_DIR.parents[3]
ORCH_PLAN_FILE = REPO_ROOT / "ORCH_PLAN.md"
WORKTREE_ROOT = REPO_ROOT / ".worktree-tmux-orch"

STATE_DIR = SKILL_DIR / ".state"
LOG_DIR = SKILL_DIR / ".logs"
RESULT_DIR = SKILL_DIR / ".results"
REPORT_DIR = SKILL_DIR / ".reports"

DEFAULT_VERIFY_CMD = "openspec validate --all"
WORKER_TERMINAL = {"done", "failed", "blocked"}

DIRECT_EXEC_KEYWORDS = [
    "直接执行",
    "不用表格",
    "跳过表格",
    "无需审查",
    "直接开跑",
    "马上执行",
]

ANALYZE_ONLY_KEYWORDS = [
    "只分析",
    "仅分析",
    "只读",
    "不修改",
    "不要修改",
    "不进行实际修改",
    "分析分支",
]

MODIFY_KEYWORDS = [
    "开始修改",
    "允许修改",
    "可以修改",
    "实际修改",
    "动手改",
]

TABLE_COLUMNS = [
    "run_id",
    "mode",
    "worker_id",
    "task_title",
    "task_scope",
    "strategy",
    "base_branch",
    "worker_branch",
    "worktree_path",
    "verify_cmd",
    "status",
    "session_id",
    "result_ref",
    "notes",
]

SUMMARY_MARKER_BEGIN = "<<<ORCH_SUMMARY"
SUMMARY_MARKER_END = ">>>"
SUMMARY_FIELDS = [
    "status",
    "summary",
    "key_changes",
    "verify",
    "risks",
    "next_steps",
]

SAME_TASK_STRATEGIES = [
    "balanced",
    "conservative",
    "performance",
    "refactor",
    "test-heavy",
    "security-first",
    "minimal-diff",
    "creative",
]


class CmdError(RuntimeError):
    pass


@dataclass
class BranchMetric:
    commits: int
    files: int
    insertions: int
    deletions: int


def iso_now() -> str:
    return datetime.now(tz=timezone.utc).replace(microsecond=0).isoformat()


def ensure_dirs() -> None:
    for p in (STATE_DIR, LOG_DIR, RESULT_DIR, REPORT_DIR, WORKTREE_ROOT):
        p.mkdir(parents=True, exist_ok=True)


def rel(path: Path) -> str:
    try:
        return str(path.resolve().relative_to(REPO_ROOT.resolve()))
    except Exception:
        return str(path)


def run_id_now() -> str:
    return datetime.now().strftime("%Y%m%d-%H%M%S") + f"-{random.randint(0, 0xFFFF):04x}"


def slugify(text: str, fallback: str = "item", limit: int = 28) -> str:
    value = re.sub(r"[^a-zA-Z0-9]+", "-", text).strip("-").lower()
    if not value:
        value = fallback
    return value[:limit].strip("-") or fallback


def sh(cmd: list[str], cwd: Path | None = None, check: bool = True) -> subprocess.CompletedProcess[str]:
    proc = subprocess.run(
        cmd,
        cwd=str(cwd or REPO_ROOT),
        text=True,
        capture_output=True,
    )
    if check and proc.returncode != 0:
        raise CmdError(
            f"command failed ({proc.returncode}): {' '.join(cmd)}\n"
            f"stdout:\n{proc.stdout}\n"
            f"stderr:\n{proc.stderr}"
        )
    return proc


def sh_bash(command: str, cwd: Path | None = None, check: bool = True) -> subprocess.CompletedProcess[str]:
    proc = subprocess.run(
        ["bash", "-lc", command],
        cwd=str(cwd or REPO_ROOT),
        text=True,
        capture_output=True,
    )
    if check and proc.returncode != 0:
        raise CmdError(
            f"command failed ({proc.returncode}): {command}\n"
            f"stdout:\n{proc.stdout}\n"
            f"stderr:\n{proc.stderr}"
        )
    return proc


def state_path(run_id: str) -> Path:
    return STATE_DIR / f"{run_id}.json"


def load_state(run_id: str) -> dict[str, Any]:
    path = state_path(run_id)
    if not path.exists():
        raise CmdError(f"run state not found: {run_id}")
    return json.loads(path.read_text(encoding="utf-8"))


def save_state(state: dict[str, Any]) -> Path:
    state["updated_at"] = iso_now()
    path = state_path(state["run_id"])
    path.write_text(json.dumps(state, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return path


def append_event(state: dict[str, Any], kind: str, detail: dict[str, Any]) -> None:
    state.setdefault("events", []).append(
        {
            "ts": iso_now(),
            "kind": kind,
            "detail": detail,
        }
    )


def git_current_branch() -> str:
    out = sh(["git", "rev-parse", "--abbrev-ref", "HEAD"]).stdout.strip()
    if out == "HEAD":
        raise CmdError("detached HEAD is not supported")
    return out


def branch_exists(branch: str) -> bool:
    proc = subprocess.run(
        ["git", "show-ref", "--verify", "--quiet", f"refs/heads/{branch}"],
        cwd=str(REPO_ROOT),
        text=True,
        capture_output=True,
    )
    return proc.returncode == 0


def detect_direct_execution(goal: str) -> bool:
    return any(keyword in goal for keyword in DIRECT_EXEC_KEYWORDS)


def detect_analyze_only(text: str) -> bool:
    return any(keyword in text for keyword in ANALYZE_ONLY_KEYWORDS)


def detect_execution_kind(goal: str, execution_kind: str) -> str:
    if execution_kind != "auto":
        return execution_kind
    return "analyze" if detect_analyze_only(goal) else "modify"


def detect_modify_intent(text: str) -> bool:
    return any(keyword in text for keyword in MODIFY_KEYWORDS)


def parse_explicit_workers(text: str) -> int | None:
    patterns = [
        r"(\d+)\s*(?:个|名)?\s*(?:codex|CODEX|worker|workers|终端|并行)",
        r"(?:并行|parallel)\s*(\d+)",
    ]
    for pattern in patterns:
        m = re.search(pattern, text)
        if not m:
            continue
        val = int(m.group(1))
        if 1 <= val <= 32:
            return val
    return None


def detect_mode(goal: str, mode: str) -> str:
    if mode != "auto":
        return mode
    same_task_markers = ["同题", "同一个", "同样的修改", "多解", "多个方案", "same task"]
    for marker in same_task_markers:
        if marker.lower() in goal.lower():
            return "same-task"
    return "split-task"


def split_goal_tasks(goal: str) -> list[str]:
    bullet_lines: list[str] = []
    for line in goal.splitlines():
        line = line.strip()
        if not line:
            continue
        if line.startswith(("-", "*", "+")) or re.match(r"^\d+[\.)]", line):
            text = re.sub(r"^[-*+\s]+", "", line)
            text = re.sub(r"^\d+[\.)]\s*", "", text)
            text = text.strip()
            if len(text) >= 4:
                bullet_lines.append(text)
    if len(bullet_lines) >= 2:
        return bullet_lines

    parts = [p.strip() for p in re.split(r"[;；\n]+", goal) if p.strip()]
    if len(parts) >= 2:
        return parts

    return [goal.strip()]


def estimate_same_task_workers(goal: str) -> int:
    size = len(goal.strip())
    workers = 5
    if size < 80:
        workers = 4
    elif size > 260:
        workers = 7

    heavy_markers = ["重构", "架构", "跨模块", "数据库", "全链路", "端到端"]
    if any(m in goal for m in heavy_markers):
        workers += 1

    return max(3, min(workers, 8))


def decide_workers(mode: str, goal: str, tasks: list[str], explicit_workers: int | None) -> int:
    if explicit_workers:
        return max(1, min(explicit_workers, 32))
    hinted = parse_explicit_workers(goal)
    if hinted:
        return hinted
    if mode == "split-task":
        return max(1, min(len(tasks), 8))
    return estimate_same_task_workers(goal)


def default_worker_row(
    run_id: str,
    mode: str,
    execution_kind: str,
    worker_id: str,
    task_title: str,
    task_scope: str,
    strategy: str,
    base_branch: str,
) -> dict[str, str]:
    branch_slug = slugify(f"{worker_id}-{strategy if strategy != '-' else task_title}", fallback=worker_id)
    worker_branch = f"orchestrator/{run_id}/{worker_id}-{branch_slug}" if execution_kind == "modify" else "-"
    worktree_path = f".worktree-tmux-orch/{run_id}/{worker_id}" if execution_kind == "modify" else "."
    verify_cmd = DEFAULT_VERIFY_CMD if execution_kind == "modify" else "-"
    return {
        "run_id": run_id,
        "mode": mode,
        "worker_id": worker_id,
        "task_title": task_title,
        "task_scope": task_scope,
        "strategy": strategy,
        "base_branch": base_branch,
        "worker_branch": worker_branch,
        "worktree_path": worktree_path,
        "verify_cmd": verify_cmd,
        "status": "planned",
        "session_id": "last",
        "result_ref": f".codex/skills/tmux-codex-orchestrator/.results/{run_id}/{worker_id}.md",
        "notes": f"execution_kind={execution_kind}",
    }


def build_worker_rows(
    run_id: str,
    mode: str,
    execution_kind: str,
    goal: str,
    tasks: list[str],
    workers: int,
    base_branch: str,
) -> list[dict[str, str]]:
    rows: list[dict[str, str]] = []

    if mode == "same-task":
        for i in range(workers):
            worker_id = f"w{i + 1:02d}"
            strategy = SAME_TASK_STRATEGIES[i % len(SAME_TASK_STRATEGIES)]
            title = f"同题多解-{strategy}"
            scope = goal if len(goal) <= 120 else goal[:120] + "..."
            row = default_worker_row(
                run_id=run_id,
                mode=mode,
                execution_kind=execution_kind,
                worker_id=worker_id,
                task_title=title,
                task_scope=scope,
                strategy=strategy,
                base_branch=base_branch,
            )
            rows.append(row)
        return rows

    bucket_count = max(1, workers)
    buckets: list[list[str]] = [[] for _ in range(bucket_count)]
    for idx, task in enumerate(tasks):
        buckets[idx % bucket_count].append(task)

    worker_idx = 0
    for group in buckets:
        if not group:
            continue
        worker_idx += 1
        worker_id = f"w{worker_idx:02d}"
        title = group[0][:80]
        scope = " | ".join(group)
        row = default_worker_row(
            run_id=run_id,
            mode=mode,
            execution_kind=execution_kind,
            worker_id=worker_id,
            task_title=title,
            task_scope=scope,
            strategy="-",
            base_branch=base_branch,
        )
        row["notes"] = f"subtasks={len(group)}"
        rows.append(row)

    return rows


def apply_execution_kind_to_rows(state: dict[str, Any]) -> None:
    execution_kind = state.get("execution_kind", "modify")
    run_id = state["run_id"]
    for row in state.get("workers", []):
        worker_id = row.get("worker_id", "w00")
        if execution_kind == "analyze":
            row["worker_branch"] = "-"
            row["worktree_path"] = "."
            row["verify_cmd"] = "-"
            row["notes"] = append_note(row.get("notes", "-"), "execution_kind=analyze")
            continue

        # modify mode
        if not row.get("worker_branch") or row.get("worker_branch") in {"-", ""}:
            task_title = row.get("task_title", worker_id)
            strategy = row.get("strategy", "-")
            branch_slug = slugify(f"{worker_id}-{strategy if strategy != '-' else task_title}", fallback=worker_id)
            row["worker_branch"] = f"orchestrator/{run_id}/{worker_id}-{branch_slug}"
        if not row.get("worktree_path") or row.get("worktree_path") in {"-", ".", ""}:
            row["worktree_path"] = f".worktree-tmux-orch/{run_id}/{worker_id}"
        if not row.get("verify_cmd") or row.get("verify_cmd") in {"-", ""}:
            row["verify_cmd"] = DEFAULT_VERIFY_CMD


def append_note(notes: str, message: str) -> str:
    text = (notes or "").strip()
    if not text or text == "-":
        return message
    if message in text:
        return text
    return f"{text}; {message}"


def escape_cell(value: Any) -> str:
    return str(value).replace("|", "\\|").replace("\n", "<br>")


def render_plan_markdown(state: dict[str, Any]) -> str:
    lines: list[str] = []
    lines.append("# ORCH_PLAN")
    lines.append("")
    lines.append(f"- run_id: `{state['run_id']}`")
    lines.append(f"- goal: {state['goal']}")
    lines.append(f"- mode: `{state['mode']}`")
    lines.append(f"- execution_kind: `{state.get('execution_kind', 'modify')}`")
    lines.append(f"- execution_policy: `{state['execution_policy']}`")
    lines.append(f"- base_branch: `{state['base_branch']}`")
    lines.append(f"- created_at: `{state['created_at']}`")
    lines.append("")
    lines.append("| " + " | ".join(TABLE_COLUMNS) + " |")
    lines.append("| " + " | ".join(["---"] * len(TABLE_COLUMNS)) + " |")

    for row in state.get("workers", []):
        vals = [escape_cell(row.get(col, "-")) for col in TABLE_COLUMNS]
        lines.append("| " + " | ".join(vals) + " |")

    lines.append("")
    lines.append("## Notes")
    lines.append("")
    lines.append("- 默认先审查表格；命中直接执行关键词时可直接 `run`。")
    lines.append("- execution_kind=analyze 时：不创建 worktree，不进行代码写入，worker 以只读沙箱执行。")
    lines.append("- 修改计划后可执行：`tmux-orch.sh revise --run <run_id> --feedback \"...\"`。")
    lines.append(f"- 查看状态：`tmux-orch.sh status --run {state['run_id']}`。")
    lines.append(f"- 查看详情：`tmux-orch.sh inspect --run {state['run_id']}`。")
    return "\n".join(lines) + "\n"


def write_plan(state: dict[str, Any]) -> None:
    ORCH_PLAN_FILE.write_text(render_plan_markdown(state), encoding="utf-8")


def split_pipe_row(line: str) -> list[str]:
    parts = [p.strip() for p in line.strip().split("|")]
    if len(parts) < 3:
        return []
    return parts[1:-1]


def load_rows_from_plan(run_id: str) -> list[dict[str, str]]:
    if not ORCH_PLAN_FILE.exists():
        return []
    lines = ORCH_PLAN_FILE.read_text(encoding="utf-8").splitlines()

    header_idx = -1
    headers: list[str] = []
    for idx, line in enumerate(lines):
        if not line.strip().startswith("|"):
            continue
        cols = split_pipe_row(line)
        if cols[: len(TABLE_COLUMNS)] == TABLE_COLUMNS:
            header_idx = idx
            headers = cols
            break

    if header_idx < 0:
        return []

    rows: list[dict[str, str]] = []
    for line in lines[header_idx + 2 :]:
        if not line.strip().startswith("|"):
            break
        cols = split_pipe_row(line)
        if len(cols) != len(headers):
            continue
        values = {headers[i]: cols[i].replace("\\|", "|") for i in range(len(headers))}
        if values.get("run_id") == run_id:
            rows.append(values)
    return rows


def sync_state_from_plan(state: dict[str, Any], plan_rows: list[dict[str, str]]) -> None:
    if not plan_rows:
        return

    runtime_fields = {
        row["worker_id"]: {
            k: v
            for k, v in row.items()
            if k
            not in {
                *TABLE_COLUMNS,
            }
        }
        for row in state.get("workers", [])
        if row.get("worker_id")
    }

    merged: list[dict[str, Any]] = []
    for row in plan_rows:
        worker_id = row.get("worker_id", "")
        item: dict[str, Any] = {k: row.get(k, "-") for k in TABLE_COLUMNS}
        item.update(runtime_fields.get(worker_id, {}))
        merged.append(item)

    state["workers"] = merged


def ensure_tool_exists(tool: str) -> bool:
    proc = subprocess.run(["bash", "-lc", f"command -v {shlex.quote(tool)} >/dev/null"], text=True)
    return proc.returncode == 0


def worker_paths(run_id: str, worker_id: str) -> dict[str, Path]:
    return {
        "prompt": STATE_DIR / run_id / f"{worker_id}.prompt.txt",
        "script": STATE_DIR / run_id / f"{worker_id}.run.sh",
        "log": LOG_DIR / run_id / f"{worker_id}.log",
        "done": LOG_DIR / run_id / f"{worker_id}.done",
        "message": RESULT_DIR / run_id / f"{worker_id}.md",
    }


def worker_prompt(state: dict[str, Any], row: dict[str, Any]) -> str:
    mode = state["mode"]
    goal = state["goal"]
    execution_kind = state.get("execution_kind", "modify")

    extra = ""
    if mode == "same-task":
        extra = (
            "你处于同题多解模式。"
            "优先执行你负责的策略，并在保证正确性的前提下突出该策略优势。"
            f"\n策略: {row['strategy']}"
        )
    else:
        extra = "你处于任务并行模式，仅处理你负责的 task_scope。"

    if execution_kind == "analyze":
        action_constraints = textwrap.dedent(
            """
            约束:
            1) 严禁修改任何代码、文档、配置或 git 历史。
            2) 仅允许读取/分析命令，不执行写入类命令。
            3) 输出最终总结，必须包含: 发现的问题、优先级、建议方案、风险评估。
            4) 若需要修改建议，使用“建议变更”形式描述，不要实际落盘。
            5) 不要输出 Markdown 本地文件链接，不要使用 `file://` URI；引用文件时只写纯文本路径。
            """
        ).strip()
        summary_hint = textwrap.dedent(
            f"""
            最终请在回复末尾附加结构化摘要，严格使用以下格式，每个字段单行：
            {SUMMARY_MARKER_BEGIN}
            status: done|blocked|failed
            summary: 一句话总结
            key_changes: 分析型任务写“无实际修改”；可用分号分隔多点
            verify: 写“not_run”或实际检查结论
            risks: 主要风险；可用分号分隔多点
            next_steps: 建议下一步；可用分号分隔多点
            {SUMMARY_MARKER_END}
            """
        ).strip()
    else:
        action_constraints = textwrap.dedent(
            f"""
            约束:
            1) 仅在当前分支内工作，不要切到其他分支。
            2) 可修改代码与文档，并自行运行必要命令。
            3) 完成后执行 verify_cmd。
            4) 完成后提交变更，提交信息使用中文并包含明确 scope。
            5) 输出最终总结，包含: 主要改动、验证结果、风险与后续建议。
            6) 不要输出 Markdown 本地文件链接，不要使用 `file://` URI；引用文件时只写纯文本路径。
            """
        ).strip()
        summary_hint = textwrap.dedent(
            f"""
            最终请在回复末尾附加结构化摘要，严格使用以下格式，每个字段单行：
            {SUMMARY_MARKER_BEGIN}
            status: done|blocked|failed
            summary: 一句话总结
            key_changes: 主要改动；可用分号分隔多点
            verify: 验证结果；可用分号分隔多点
            risks: 风险与后续建议；可用分号分隔多点
            next_steps: 建议下一步；可用分号分隔多点
            {SUMMARY_MARKER_END}
            """
        ).strip()

    prompt = textwrap.dedent(
        f"""
        你是并行 worker {row['worker_id']}。

        全局目标:
        {goal}

        你的任务:
        - task_title: {row['task_title']}
        - task_scope: {row['task_scope']}
        - branch: {row['worker_branch']}
        - verify_cmd: {row['verify_cmd'] or DEFAULT_VERIFY_CMD}
        - execution_kind: {execution_kind}

        {action_constraints}

        {summary_hint}

        {extra}
        """
    ).strip()
    return prompt + "\n"


def write_worker_files(
    state: dict[str, Any],
    row: dict[str, Any],
    prompt_text: str,
    use_resume: bool,
) -> dict[str, Path]:
    run_id = state["run_id"]
    worker_id = row["worker_id"]
    paths = worker_paths(run_id, worker_id)

    for key in ("prompt", "script", "log", "done", "message"):
        paths[key].parent.mkdir(parents=True, exist_ok=True)

    paths["prompt"].write_text(prompt_text, encoding="utf-8")

    worktree_abs = (REPO_ROOT / row["worktree_path"]).resolve()
    execution_kind = state.get("execution_kind", "modify")

    exec_prefix = "codex exec --json"
    resume_prefix = "codex exec resume --last --json"
    if execution_kind == "analyze":
        exec_prefix = "codex exec --json --sandbox read-only"
        resume_prefix = "codex exec resume --last --json --sandbox read-only"

    cmd_start = f'{exec_prefix} -o "$MSG_FILE" "$PROMPT_TEXT" >>"$LOG_FILE" 2>&1'
    cmd_resume = (
        f'{resume_prefix} -o "$MSG_FILE" "$PROMPT_TEXT" >>"$LOG_FILE" 2>&1 '
        f'|| {exec_prefix} -o "$MSG_FILE" "$PROMPT_TEXT" >>"$LOG_FILE" 2>&1'
    )
    cmd = cmd_resume if use_resume else cmd_start

    script = textwrap.dedent(
        f"""#!/usr/bin/env bash
        set -u
        WORKTREE={shlex.quote(str(worktree_abs))}
        PROMPT_FILE={shlex.quote(str(paths['prompt']))}
        LOG_FILE={shlex.quote(str(paths['log']))}
        DONE_FILE={shlex.quote(str(paths['done']))}
        MSG_FILE={shlex.quote(str(paths['message']))}

        mkdir -p "$(dirname "$LOG_FILE")" "$(dirname "$DONE_FILE")" "$(dirname "$MSG_FILE")"
        rm -f "$DONE_FILE"
        done_written=0

        write_done() {{
          local code="$1"
          if [ "$done_written" -eq 1 ]; then
            return
          fi
          echo "$code" >"$DONE_FILE"
          done_written=1
        }}

        handle_interrupt() {{
          write_done 130
          exit 0
        }}

        trap 'write_done $?' EXIT
        trap handle_interrupt INT TERM HUP

        cd "$WORKTREE"
        PROMPT_TEXT="$(cat "$PROMPT_FILE")"

        if {cmd}; then
          rc=0
        else
          rc=$?
        fi

        write_done "$rc"
        exit 0
        """
    )
    paths["script"].write_text(script, encoding="utf-8")
    paths["script"].chmod(0o755)
    return paths


def ensure_branch_or_raise(branch: str) -> None:
    if not branch_exists(branch):
        raise CmdError(f"branch not found: {branch}")


def ensure_worker_worktree(row: dict[str, Any]) -> Path:
    base_branch = row["base_branch"].strip()
    worker_branch = row["worker_branch"].strip()
    wt_path = (REPO_ROOT / row["worktree_path"]).resolve()

    ensure_branch_or_raise(base_branch)

    if wt_path.exists() and not (wt_path / ".git").exists():
        raise CmdError(f"worktree path exists but is not git worktree: {wt_path}")

    if not wt_path.exists():
        wt_path.parent.mkdir(parents=True, exist_ok=True)
        if branch_exists(worker_branch):
            sh(["git", "worktree", "add", str(wt_path), worker_branch])
        else:
            sh(["git", "worktree", "add", "-b", worker_branch, str(wt_path), base_branch])

    return wt_path


def tmux_has_session(session_name: str) -> bool:
    proc = subprocess.run(
        ["tmux", "has-session", "-t", session_name],
        text=True,
        capture_output=True,
    )
    return proc.returncode == 0


def tmux_new_session(session_name: str) -> str:
    sh(["tmux", "new-session", "-d", "-s", session_name, "-n", "workers"])
    pane_id = sh(["tmux", "display-message", "-p", "-t", f"{session_name}:0.0", "#{pane_id}"]).stdout.strip()
    if not pane_id:
        raise CmdError("failed to create tmux pane")
    return pane_id


def tmux_new_pane(session_name: str) -> str:
    pane_id = sh(["tmux", "split-window", "-d", "-t", f"{session_name}:0", "-P", "-F", "#{pane_id}"]).stdout.strip()
    sh(["tmux", "select-layout", "-t", f"{session_name}:0", "tiled"])
    return pane_id


def tmux_pane_exists(pane_id: str) -> bool:
    if not pane_id:
        return False
    panes = sh(["tmux", "list-panes", "-a", "-F", "#{pane_id}"], check=False).stdout.splitlines()
    return pane_id in panes


def tmux_send(pane_id: str, command: str) -> None:
    sh(["tmux", "send-keys", "-t", pane_id, command, "C-m"])


def tmux_ctrl_c(pane_id: str) -> None:
    sh(["tmux", "send-keys", "-t", pane_id, "C-c"], check=False)


def tmux_kill_session(session_name: str) -> None:
    sh(["tmux", "kill-session", "-t", session_name], check=False)


def worker_is_terminal(status: str) -> bool:
    return status in WORKER_TERMINAL


def worker_exit_status(code: int) -> str:
    if code == 0:
        return "done"
    if code == 130:
        return "blocked"
    return "failed"


def refresh_worker_statuses(state: dict[str, Any]) -> None:
    session_name = state.get("session_name", "")
    session_alive = bool(session_name) and tmux_has_session(session_name)

    for row in state.get("workers", []):
        done_raw = str(row.get("done_file", "")).strip()
        done_file: Path | None = None
        if done_raw and done_raw not in {"-", "."}:
            done_file = Path(done_raw)
        pane_id = row.get("pane_id", "")
        status = row.get("status", "planned")

        if done_file and done_file.exists() and done_file.is_file():
            code_txt = done_file.read_text(encoding="utf-8").strip()
            try:
                code = int(code_txt)
            except Exception:
                code = 1
            row["status"] = worker_exit_status(code)
            row["notes"] = append_note(row.get("notes", "-"), f"exit={code}")
            continue

        if status == "running":
            if session_alive and pane_id and tmux_pane_exists(pane_id):
                row["status"] = "running"
            else:
                row["status"] = "blocked"
                row["notes"] = append_note(row.get("notes", "-"), "worker_interrupted")

    all_terminal = all(worker_is_terminal(r.get("status", "")) for r in state.get("workers", []))
    if all_terminal and session_alive:
        tmux_kill_session(session_name)
        append_event(state, "session_closed_auto", {"session": session_name})


def ensure_worker_pane(state: dict[str, Any], row: dict[str, Any]) -> str:
    session_name = state.get("session_name", "")
    pane_id = row.get("pane_id", "")

    if session_name and tmux_has_session(session_name) and pane_id and tmux_pane_exists(pane_id):
        return pane_id

    if not session_name:
        session_name = f"orch-{state['run_id']}"[:40]
        state["session_name"] = session_name

    if not tmux_has_session(session_name):
        pane_id = tmux_new_session(session_name)
    else:
        pane_id = tmux_new_pane(session_name)

    row["pane_id"] = pane_id
    return pane_id


def start_worker(
    state: dict[str, Any],
    row: dict[str, Any],
    pane_id: str,
    prompt_text: str,
    use_resume: bool,
) -> None:
    paths = write_worker_files(state, row, prompt_text=prompt_text, use_resume=use_resume)
    tmux_send(pane_id, f"bash {shlex.quote(str(paths['script']))}")

    row["pane_id"] = pane_id
    row["status"] = "running"
    row["session_id"] = "last"
    row["result_ref"] = rel(paths["message"])
    row["prompt_file"] = str(paths["prompt"])
    row["script_file"] = str(paths["script"])
    row["log_file"] = str(paths["log"])
    row["done_file"] = str(paths["done"])
    row["notes"] = append_note(row.get("notes", "-"), f"pane={pane_id}")


def row_by_worker_id(state: dict[str, Any], worker_id: str) -> dict[str, Any]:
    for row in state.get("workers", []):
        if row.get("worker_id") == worker_id:
            return row
    raise CmdError(f"worker not found: {worker_id}")


def worker_status_counter(rows: list[dict[str, Any]]) -> dict[str, int]:
    counts: dict[str, int] = {}
    for row in rows:
        s = row.get("status", "")
        counts[s] = counts.get(s, 0) + 1
    return counts


def shorten_text(value: str, limit: int = 160) -> str:
    text = " ".join(value.split())
    if len(text) <= limit:
        return text
    return text[: limit - 3].rstrip() + "..."


def sanitize_result_text(text: str) -> str:
    value = re.sub(r"\[([^\]]+)\]\((/[^)]+)\)", r"\1: \2", text)
    value = re.sub(r"file://\S+", "[local-file-uri omitted]", value)
    return value


def parse_worker_summary(content: str) -> dict[str, str]:
    text = sanitize_result_text(content).strip()
    if not text:
        return {}

    begin = text.rfind(SUMMARY_MARKER_BEGIN)
    if begin < 0:
        return {}

    tail = text[begin:].splitlines()
    if not tail or tail[0].strip() != SUMMARY_MARKER_BEGIN:
        return {}

    parsed: dict[str, str] = {}
    for line in tail[1:]:
        stripped = line.strip()
        if stripped == SUMMARY_MARKER_END:
            break
        if ":" not in stripped:
            continue
        key, value = stripped.split(":", 1)
        key = key.strip()
        if key in SUMMARY_FIELDS:
            parsed[key] = value.strip()
    return parsed


def load_worker_result_bundle(row: dict[str, Any]) -> tuple[str | None, dict[str, str]]:
    result_ref = str(row.get("result_ref", "")).strip()
    if not result_ref or result_ref == "-":
        return None, {}
    result_path = (REPO_ROOT / result_ref).resolve()
    if not result_path.exists():
        return None, {}
    content = sanitize_result_text(result_path.read_text(encoding="utf-8")).strip()
    return content, parse_worker_summary(content)


def inspect_worker_summary(row: dict[str, Any]) -> list[str]:
    lines: list[str] = []
    worker_id = row.get("worker_id", "-")
    lines.append(f"## {worker_id}")
    lines.append("")
    lines.append(f"- status: `{row.get('status', '-')}`")
    lines.append(f"- task_title: {row.get('task_title', '-')}")
    lines.append(f"- task_scope: {row.get('task_scope', '-')}")
    lines.append(f"- pane_id: `{row.get('pane_id', '-')}`")
    lines.append(f"- result_ref: `{row.get('result_ref', '-')}`")
    lines.append(f"- log_file: `{row.get('log_file', '-')}`")
    lines.append(f"- notes: {row.get('notes', '-')}")

    content, summary = load_worker_result_bundle(row)
    if summary:
        lines.append(f"- summary_status: `{summary.get('status', '-')}`")
        lines.append(f"- summary: {summary.get('summary', '-')}")
        lines.append(f"- verify: {summary.get('verify', '-')}")
        lines.append(f"- risks: {summary.get('risks', '-')}")
        lines.append(f"- next_steps: {summary.get('next_steps', '-')}")
    elif content:
        lines.append(f"- result_preview: {shorten_text(content)}")
    elif str(row.get("result_ref", "")).strip() not in {"", "-"}:
        lines.append("- result_preview: (missing or empty file)")
    else:
        lines.append("- result_preview: -")

    lines.append("")
    return lines


def build_inspect_report(state: dict[str, Any]) -> str:
    counts = worker_status_counter(state.get("workers", []))
    lines: list[str] = []
    lines.append(f"run_id={state['run_id']}")
    lines.append(f"mode={state['mode']}")
    lines.append(f"execution_kind={state.get('execution_kind', 'modify')}")
    lines.append(f"execution_policy={state.get('execution_policy', '-')}")
    lines.append(f"session={state.get('session_name', '-')}")
    lines.append(f"synth_status={state.get('synth_status', '-')}")
    lines.append(f"goal={state.get('goal', '-')}")
    lines.append("status_counts=" + ", ".join(f"{key}:{counts[key]}" for key in sorted(counts)) if counts else "status_counts=none")
    lines.append("")

    if state.get("events"):
        lines.append("recent_events:")
        for event in state["events"][-5:]:
            lines.append(f"- {event.get('ts', '-')}: {event.get('kind', '-')} {json.dumps(event.get('detail', {}), ensure_ascii=False)}")
        lines.append("")

    for row in state.get("workers", []):
        lines.extend(inspect_worker_summary(row))

    return "\n".join(lines).rstrip() + "\n"


def parse_shortstat(text: str) -> tuple[int, int]:
    ins = 0
    dels = 0
    m1 = re.search(r"(\d+)\s+insertions?", text)
    if m1:
        ins = int(m1.group(1))
    m2 = re.search(r"(\d+)\s+deletions?", text)
    if m2:
        dels = int(m2.group(1))
    return ins, dels


def branch_metric(base_branch: str, branch: str) -> BranchMetric:
    commits = int(sh(["git", "rev-list", "--count", f"{base_branch}..{branch}"]).stdout.strip() or "0")
    files = len(
        [
            l
            for l in sh(["git", "diff", "--name-only", f"{base_branch}...{branch}"]).stdout.splitlines()
            if l.strip()
        ]
    )
    shortstat = sh(["git", "diff", "--shortstat", f"{base_branch}...{branch}"]).stdout.strip()
    ins, dels = parse_shortstat(shortstat)
    return BranchMetric(commits=commits, files=files, insertions=ins, deletions=dels)


def metric_pros_cons(metric: BranchMetric) -> tuple[str, str]:
    pros: list[str] = []
    cons: list[str] = []

    if metric.files <= 12:
        pros.append("改动面可控")
    elif metric.files >= 40:
        cons.append("改动面较大")

    churn = metric.insertions + metric.deletions
    if churn <= 300:
        pros.append("代码变更量适中")
    elif churn >= 1200:
        cons.append("代码 churn 较高")

    if 1 <= metric.commits <= 4:
        pros.append("提交颗粒度较清晰")
    elif metric.commits >= 10:
        cons.append("提交数量偏多")

    if not pros:
        pros.append("覆盖了目标范围")
    if not cons:
        cons.append("需结合结果进一步人工审阅")

    return ", ".join(pros), ", ".join(cons)


def choose_same_task_baseline(state: dict[str, Any], done_rows: list[dict[str, Any]]) -> tuple[dict[str, Any], dict[str, BranchMetric]]:
    base_branch = state["base_branch"]
    metrics: dict[str, BranchMetric] = {}

    best_row = done_rows[0]
    best_score = -10**9

    for row in done_rows:
        metric = branch_metric(base_branch, row["worker_branch"])
        metrics[row["worker_id"]] = metric

        score = 0
        strategy = row.get("strategy", "")
        if strategy == "balanced":
            score += 30
        elif strategy == "conservative":
            score += 20
        elif strategy == "test-heavy":
            score += 18

        score += max(0, 15 - abs(metric.files - 12))
        score += max(0, 20 - abs((metric.insertions + metric.deletions) - 450) // 30)
        score += max(0, 8 - abs(metric.commits - 3) * 2)

        if score > best_score:
            best_score = score
            best_row = row

    return best_row, metrics


def ensure_synth_worktree(run_id: str, synth_branch: str, start_ref: str) -> Path:
    synth_path = (WORKTREE_ROOT / run_id / "synth").resolve()

    if synth_path.exists() and not (synth_path / ".git").exists():
        raise CmdError(f"synth worktree path exists but is not git worktree: {synth_path}")

    if not synth_path.exists():
        synth_path.parent.mkdir(parents=True, exist_ok=True)
        if branch_exists(synth_branch):
            sh(["git", "worktree", "add", str(synth_path), synth_branch])
        else:
            sh(["git", "worktree", "add", "-b", synth_branch, str(synth_path), start_ref])

    return synth_path


def git_merge_in_worktree(worktree: Path, branch: str) -> tuple[bool, str]:
    proc = subprocess.run(
        ["git", "merge", "--no-ff", "--no-edit", branch],
        cwd=str(worktree),
        text=True,
        capture_output=True,
    )
    if proc.returncode == 0:
        return True, proc.stdout.strip()

    subprocess.run(["git", "merge", "--abort"], cwd=str(worktree), text=True, capture_output=True)
    return False, (proc.stderr.strip() or proc.stdout.strip() or "merge failed")


def build_analysis_only_report(state: dict[str, Any]) -> Path:
    run_id = state["run_id"]
    report_path = REPORT_DIR / f"{run_id}-analysis.md"
    lines: list[str] = []

    lines.append(f"# Analysis Report: {run_id}")
    lines.append("")
    lines.append(f"- mode: `{state['mode']}`")
    lines.append("- execution_kind: `analyze`")
    lines.append(f"- goal: {state['goal']}")
    lines.append("")
    lines.append("## Worker Outputs")
    lines.append("")

    for row in state.get("workers", []):
        worker_id = row.get("worker_id", "-")
        status = row.get("status", "-")
        lines.append(f"### {worker_id}")
        lines.append("")
        lines.append(f"- status: `{status}`")
        lines.append(f"- task: {row.get('task_title', '-')}")
        lines.append(f"- scope: {row.get('task_scope', '-')}")
        lines.append("")

        result_ref = row.get("result_ref", "")
        if not result_ref or result_ref == "-":
            lines.append("- result: (empty)")
            lines.append("")
            continue

        content, summary = load_worker_result_bundle(row)
        result_path = (REPO_ROOT / result_ref).resolve()
        if not result_path.exists():
            lines.append(f"- result file missing: `{result_ref}`")
            lines.append("")
            continue

        if not content:
            lines.append(f"- result file empty: `{result_ref}`")
            lines.append("")
            continue

        if summary:
            for field in SUMMARY_FIELDS:
                if field in summary:
                    lines.append(f"- {field}: {summary[field]}")
        else:
            snippet = content[:1800]
            lines.append("```text")
            lines.append(snippet)
            if len(content) > len(snippet):
                lines.append("... (truncated)")
            lines.append("```")
        lines.append("")

    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return report_path


def build_synthesis_report(
    state: dict[str, Any],
    synth_branch: str,
    synth_worktree: Path,
    merge_actions: list[str],
    metrics: dict[str, BranchMetric],
    verify_results: list[str],
    baseline_worker_id: str | None,
) -> Path:
    run_id = state["run_id"]
    report_path = REPORT_DIR / f"{run_id}-synthesis.md"

    lines: list[str] = []
    lines.append(f"# Synthesis Report: {run_id}")
    lines.append("")
    lines.append(f"- mode: `{state['mode']}`")
    lines.append(f"- goal: {state['goal']}")
    lines.append(f"- synth_branch: `{synth_branch}`")
    lines.append(f"- synth_worktree: `{rel(synth_worktree)}`")
    if baseline_worker_id:
        lines.append(f"- baseline_worker: `{baseline_worker_id}`")
    lines.append("")

    lines.append("## Worker Comparison")
    lines.append("")
    lines.append("| worker_id | branch | strategy | commits | files | insertions | deletions | pros | cons |")
    lines.append("| --- | --- | --- | --- | --- | --- | --- | --- | --- |")

    for row in state.get("workers", []):
        if row.get("status") not in {"done", "failed", "blocked"}:
            continue
        metric = metrics.get(row["worker_id"])
        if metric is None:
            continue
        pros, cons = metric_pros_cons(metric)
        lines.append(
            "| "
            + " | ".join(
                [
                    row["worker_id"],
                    row["worker_branch"],
                    row.get("strategy", "-"),
                    str(metric.commits),
                    str(metric.files),
                    str(metric.insertions),
                    str(metric.deletions),
                    pros,
                    cons,
                ]
            )
            + " |"
        )

    lines.append("")
    lines.append("## Integration Actions")
    lines.append("")
    if merge_actions:
        for action in merge_actions:
            lines.append(f"- {action}")
    else:
        lines.append("- no merge actions")

    lines.append("")
    lines.append("## Verify Results")
    lines.append("")
    if verify_results:
        for item in verify_results:
            lines.append(f"- {item}")
    else:
        lines.append("- not executed")

    lines.append("")
    head = sh(["git", "rev-parse", "HEAD"], cwd=synth_worktree).stdout.strip()
    lines.append("## Final Output")
    lines.append("")
    lines.append(f"- final_head: `{head}`")
    lines.append(f"- preserved_worker_branches: `{len(state.get('workers', []))}`")
    lines.append("- preserved_worker_worktrees: yes")

    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return report_path


def cmd_doctor(args: argparse.Namespace) -> int:
    ensure_dirs()

    needed = ["git", "tmux", "codex"]
    missing = [tool for tool in needed if not ensure_tool_exists(tool)]

    print(f"repo_root={REPO_ROOT}")
    print(f"orch_plan={ORCH_PLAN_FILE}")
    print(f"state_dir={STATE_DIR}")
    print("missing_tools=" + (", ".join(missing) if missing else "none"))
    print("quickstart=draft -> run -> status -> inspect -> synthesize/report")
    if missing:
        return 1
    return 0


def cmd_draft(args: argparse.Namespace) -> int:
    ensure_dirs()

    goal = args.goal.strip()
    if not goal:
        raise CmdError("--goal must not be empty")

    run_id = args.run or run_id_now()
    mode = detect_mode(goal, args.mode)
    execution_kind = detect_execution_kind(goal, args.execution_kind)
    tasks = split_goal_tasks(goal)
    workers = decide_workers(mode, goal, tasks, args.workers)
    base_branch = git_current_branch()

    rows = build_worker_rows(
        run_id=run_id,
        mode=mode,
        execution_kind=execution_kind,
        goal=goal,
        tasks=tasks,
        workers=workers,
        base_branch=base_branch,
    )

    state = {
        "run_id": run_id,
        "goal": goal,
        "mode": mode,
        "execution_kind": execution_kind,
        "execution_policy": "direct" if detect_direct_execution(goal) else "review_first",
        "base_branch": base_branch,
        "session_name": f"orch-{run_id}"[:40],
        "created_at": iso_now(),
        "updated_at": iso_now(),
        "workers": rows,
        "events": [],
        "synth_branch": "",
        "synth_worktree": "",
        "synth_report": "",
        "synth_status": "-",
    }

    append_event(
        state,
        "draft",
        {
            "mode": mode,
            "execution_kind": execution_kind,
            "worker_count": len(rows),
            "execution_policy": state["execution_policy"],
        },
    )

    save_state(state)
    write_plan(state)

    print(f"run_id={run_id}")
    print(f"mode={mode}")
    print(f"execution_kind={execution_kind}")
    print(f"workers={len(rows)}")
    print(f"execution_policy={state['execution_policy']}")
    print(f"plan={ORCH_PLAN_FILE}")
    return 0


def resize_same_task_workers(state: dict[str, Any], new_count: int) -> None:
    new_count = max(1, min(new_count, 32))
    old_rows = state.get("workers", [])
    base_branch = state["base_branch"]
    run_id = state["run_id"]

    fresh_rows = build_worker_rows(
        run_id=run_id,
        mode="same-task",
        execution_kind=state.get("execution_kind", "modify"),
        goal=state["goal"],
        tasks=[state["goal"]],
        workers=new_count,
        base_branch=base_branch,
    )

    old_by_id = {row["worker_id"]: row for row in old_rows}
    merged: list[dict[str, Any]] = []
    for row in fresh_rows:
        worker_id = row["worker_id"]
        if worker_id in old_by_id:
            old = old_by_id[worker_id]
            for k in ("status", "session_id", "result_ref", "notes", "pane_id", "prompt_file", "script_file", "log_file", "done_file"):
                if k in old:
                    row[k] = old[k]
        merged.append(row)

    state["workers"] = merged


def cmd_revise(args: argparse.Namespace) -> int:
    ensure_dirs()
    state = load_state(args.run)

    plan_rows = load_rows_from_plan(args.run)
    if plan_rows:
        sync_state_from_plan(state, plan_rows)

    feedback = args.feedback.strip()
    if not feedback:
        raise CmdError("--feedback must not be empty")

    if state["mode"] == "same-task":
        count = parse_explicit_workers(feedback)
        if count:
            resize_same_task_workers(state, count)
            append_event(state, "revise_resize", {"workers": count})

    if detect_direct_execution(feedback):
        state["execution_policy"] = "direct"
    if detect_analyze_only(feedback):
        state["execution_kind"] = "analyze"
    elif detect_modify_intent(feedback):
        state["execution_kind"] = "modify"

    apply_execution_kind_to_rows(state)

    state["goal"] = state["goal"]
    append_event(state, "revise_feedback", {"feedback": feedback})
    save_state(state)
    write_plan(state)

    print(f"run_id={state['run_id']}")
    print(f"workers={len(state.get('workers', []))}")
    print(f"execution_kind={state.get('execution_kind', 'modify')}")
    print(f"execution_policy={state['execution_policy']}")
    return 0


def cmd_run(args: argparse.Namespace) -> int:
    ensure_dirs()
    state = load_state(args.run)

    plan_rows = load_rows_from_plan(args.run)
    if plan_rows:
        sync_state_from_plan(state, plan_rows)
    apply_execution_kind_to_rows(state)

    refresh_worker_statuses(state)

    session_name = state.get("session_name") or f"orch-{state['run_id']}"[:40]
    state["session_name"] = session_name

    if tmux_has_session(session_name) and not args.reuse_session:
        raise CmdError(f"tmux session already exists: {session_name} (use --reuse-session or close first)")

    first_pane = ""
    if not tmux_has_session(session_name):
        first_pane = tmux_new_session(session_name)

    launched = 0
    execution_kind = state.get("execution_kind", "modify")
    for row in state.get("workers", []):
        if row.get("status") == "done":
            continue

        if execution_kind == "modify":
            ensure_worker_worktree(row)

        if first_pane:
            pane_id = first_pane
            first_pane = ""
        elif row.get("pane_id") and tmux_pane_exists(row.get("pane_id", "")):
            pane_id = row["pane_id"]
        else:
            pane_id = tmux_new_pane(session_name)

        prompt = worker_prompt(state, row)
        start_worker(state, row, pane_id=pane_id, prompt_text=prompt, use_resume=False)
        launched += 1

    append_event(
        state,
        "run",
        {
            "session_name": session_name,
            "launched": launched,
        },
    )
    save_state(state)
    write_plan(state)

    print(f"run_id={state['run_id']}")
    print(f"session={session_name}")
    print(f"launched={launched}")
    return 0


def cmd_control(args: argparse.Namespace) -> int:
    ensure_dirs()
    state = load_state(args.run)
    apply_execution_kind_to_rows(state)
    refresh_worker_statuses(state)

    row = row_by_worker_id(state, args.worker)
    action = args.action

    if action == "stop":
        pane_id = row.get("pane_id", "")
        if pane_id and tmux_pane_exists(pane_id):
            tmux_ctrl_c(pane_id)
        row["status"] = "blocked"
        row["notes"] = append_note(row.get("notes", "-"), "manual_stop")

    elif action in {"inject", "restart"}:
        if action == "inject" and not args.prompt:
            raise CmdError("--prompt is required for action=inject")

        pane_id = ensure_worker_pane(state, row)
        if pane_id:
            tmux_ctrl_c(pane_id)

        prompt_text = args.prompt.strip() if args.prompt else worker_prompt(state, row)
        use_resume = action == "inject"
        start_worker(state, row, pane_id=pane_id, prompt_text=prompt_text, use_resume=use_resume)
        row["notes"] = append_note(row.get("notes", "-"), f"manual_{action}")

    else:
        raise CmdError(f"unsupported action: {action}")

    append_event(
        state,
        "control",
        {
            "worker": args.worker,
            "action": action,
        },
    )
    save_state(state)
    write_plan(state)

    print(f"run_id={state['run_id']}")
    print(f"worker={args.worker}")
    print(f"action={action}")
    print(f"status={row.get('status')}")
    return 0


def cmd_status(args: argparse.Namespace) -> int:
    ensure_dirs()
    state = load_state(args.run)
    apply_execution_kind_to_rows(state)
    refresh_worker_statuses(state)

    counts = worker_status_counter(state.get("workers", []))
    append_event(state, "status", {"counts": counts})
    save_state(state)
    write_plan(state)

    if args.json:
        print(json.dumps({"run_id": args.run, "status": counts, "workers": state.get("workers", [])}, ensure_ascii=False, indent=2))
        return 0

    print(f"run_id={args.run}")
    print(f"execution_kind={state.get('execution_kind', 'modify')}")
    print(f"session={state.get('session_name', '-')}")
    for k in sorted(counts):
        print(f"- {k}: {counts[k]}")
    for row in state.get("workers", []):
        print(f"  * {row['worker_id']}: {row.get('status')} ({row.get('worker_branch')} -> {row.get('result_ref')})")
    return 0


def cmd_inspect(args: argparse.Namespace) -> int:
    state = load_state(args.run)
    apply_execution_kind_to_rows(state)
    refresh_worker_statuses(state)
    save_state(state)
    write_plan(state)
    print(build_inspect_report(state))
    return 0


def cmd_synthesize(args: argparse.Namespace) -> int:
    ensure_dirs()
    state = load_state(args.run)
    apply_execution_kind_to_rows(state)
    refresh_worker_statuses(state)

    done_rows = [row for row in state.get("workers", []) if row.get("status") == "done"]
    if not done_rows:
        raise CmdError("no done workers; run status/run first")

    execution_kind = state.get("execution_kind", "modify")
    if execution_kind == "analyze":
        report = build_analysis_only_report(state)
        state["synth_branch"] = "-"
        state["synth_worktree"] = "-"
        state["synth_report"] = rel(report)
        state["synth_status"] = "analysis_ready"
        append_event(
            state,
            "synthesize_analysis_only",
            {
                "report": rel(report),
                "done_workers": len(done_rows),
            },
        )
        save_state(state)
        write_plan(state)
        print(f"run_id={state['run_id']}")
        print("execution_kind=analyze")
        print(f"report={report}")
        print("synth_status=analysis_ready")
        return 0

    mode = state["mode"]
    base_branch = state["base_branch"]
    synth_branch = args.branch or f"orchestrator/{state['run_id']}/synth"

    baseline_worker_id: str | None = None
    metrics: dict[str, BranchMetric] = {}

    if mode == "same-task":
        baseline, metrics = choose_same_task_baseline(state, done_rows)
        baseline_worker_id = baseline["worker_id"]
        start_ref = baseline["worker_branch"]
    else:
        start_ref = base_branch
        for row in done_rows:
            metrics[row["worker_id"]] = branch_metric(base_branch, row["worker_branch"])

    synth_worktree = ensure_synth_worktree(state["run_id"], synth_branch, start_ref)

    # Ensure synth branch is checked out in synth worktree.
    sh(["git", "checkout", synth_branch], cwd=synth_worktree)

    merge_actions: list[str] = []
    blocked = False

    if mode == "split-task":
        for row in done_rows:
            branch = row["worker_branch"]
            ok, detail = git_merge_in_worktree(synth_worktree, branch)
            if ok:
                merge_actions.append(f"merged `{branch}`")
            else:
                merge_actions.append(f"blocked `{branch}`: {detail.splitlines()[0]}")
                blocked = True
                break
    else:
        assert baseline_worker_id is not None
        for row in done_rows:
            if row["worker_id"] == baseline_worker_id:
                continue
            branch = row["worker_branch"]
            ok, detail = git_merge_in_worktree(synth_worktree, branch)
            if ok:
                merge_actions.append(f"absorbed `{branch}` into baseline")
            else:
                merge_actions.append(f"skipped `{branch}` due to conflict: {detail.splitlines()[0]}")

    verify_results: list[str] = []
    if not args.skip_verify and not blocked:
        verify_cmds = [
            (row.get("verify_cmd") or DEFAULT_VERIFY_CMD).strip() or DEFAULT_VERIFY_CMD
            for row in done_rows
        ]
        seen: set[str] = set()
        uniq_cmds: list[str] = []
        for cmd in verify_cmds:
            if cmd in seen:
                continue
            uniq_cmds.append(cmd)
            seen.add(cmd)

        for cmd in uniq_cmds:
            proc = sh_bash(cmd, cwd=synth_worktree, check=False)
            if proc.returncode == 0:
                verify_results.append(f"PASS `{cmd}`")
            else:
                verify_results.append(f"FAIL `{cmd}` (rc={proc.returncode})")
                blocked = True
                break

    report = build_synthesis_report(
        state=state,
        synth_branch=synth_branch,
        synth_worktree=synth_worktree,
        merge_actions=merge_actions,
        metrics=metrics,
        verify_results=verify_results,
        baseline_worker_id=baseline_worker_id,
    )

    state["synth_branch"] = synth_branch
    state["synth_worktree"] = rel(synth_worktree)
    state["synth_report"] = rel(report)
    state["synth_status"] = "blocked" if blocked else "ready"

    append_event(
        state,
        "synthesize",
        {
            "synth_branch": synth_branch,
            "synth_worktree": rel(synth_worktree),
            "status": state["synth_status"],
        },
    )

    save_state(state)
    write_plan(state)

    print(f"run_id={state['run_id']}")
    print(f"synth_branch={synth_branch}")
    print(f"synth_worktree={rel(synth_worktree)}")
    print(f"report={report}")
    print(f"synth_status={state['synth_status']}")
    return 1 if blocked else 0


def cmd_report(args: argparse.Namespace) -> int:
    state = load_state(args.run)
    apply_execution_kind_to_rows(state)
    refresh_worker_statuses(state)
    save_state(state)
    write_plan(state)

    report = state.get("synth_report", "")
    if report:
        report_path = (REPO_ROOT / report).resolve()
        if report_path.exists():
            print(report_path.read_text(encoding="utf-8"))
            return 0

    counts = worker_status_counter(state.get("workers", []))
    print(f"run_id={state['run_id']}")
    print(f"mode={state['mode']}")
    print(f"execution_kind={state.get('execution_kind', 'modify')}")
    print(f"goal={state['goal']}")
    print(f"synth_status={state.get('synth_status', '-')}")
    print("worker_status:")
    for k in sorted(counts):
        print(f"- {k}: {counts[k]}")
    for row in state.get("workers", []):
        content, summary = load_worker_result_bundle(row)
        if summary:
            print(f"* {row['worker_id']}: {summary.get('summary', '-')}")
        elif content:
            print(f"* {row['worker_id']}: {shorten_text(content, 120)}")
    return 0


def cmd_close(args: argparse.Namespace) -> int:
    state = load_state(args.run)
    apply_execution_kind_to_rows(state)
    session_name = state.get("session_name", "")
    if session_name and tmux_has_session(session_name):
        interrupted = 0
        for row in state.get("workers", []):
            if row.get("status") != "running":
                continue
            pane_id = str(row.get("pane_id", "")).strip()
            if pane_id and tmux_pane_exists(pane_id):
                tmux_ctrl_c(pane_id)
                interrupted += 1

        if interrupted:
            time.sleep(0.8)

        tmux_kill_session(session_name)
        refresh_worker_statuses(state)
        append_event(
            state,
            "session_closed_manual",
            {
                "session": session_name,
                "interrupted_workers": interrupted,
            },
        )
        save_state(state)
        write_plan(state)
        print(f"closed_session={session_name}")
        return 0

    print("no_active_session")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="tmux-codex-orchestrator helper")
    sub = parser.add_subparsers(dest="command", required=True)

    p_doctor = sub.add_parser("doctor")
    p_doctor.set_defaults(func=cmd_doctor)

    p_draft = sub.add_parser("draft")
    p_draft.add_argument("--goal", required=True)
    p_draft.add_argument("--mode", default="auto", choices=["auto", "same-task", "split-task"])
    p_draft.add_argument("--execution-kind", default="auto", choices=["auto", "modify", "analyze"])
    p_draft.add_argument("--workers", type=int)
    p_draft.add_argument("--run")
    p_draft.set_defaults(func=cmd_draft)

    p_revise = sub.add_parser("revise")
    p_revise.add_argument("--run", required=True)
    p_revise.add_argument("--feedback", required=True)
    p_revise.set_defaults(func=cmd_revise)

    p_run = sub.add_parser("run")
    p_run.add_argument("--run", required=True)
    p_run.add_argument("--reuse-session", action="store_true")
    p_run.set_defaults(func=cmd_run)

    p_control = sub.add_parser("control")
    p_control.add_argument("--run", required=True)
    p_control.add_argument("--worker", required=True)
    p_control.add_argument("--action", required=True, choices=["stop", "inject", "restart"])
    p_control.add_argument("--prompt")
    p_control.set_defaults(func=cmd_control)

    p_status = sub.add_parser("status")
    p_status.add_argument("--run", required=True)
    p_status.add_argument("--json", action="store_true")
    p_status.set_defaults(func=cmd_status)

    p_inspect = sub.add_parser("inspect")
    p_inspect.add_argument("--run", required=True)
    p_inspect.set_defaults(func=cmd_inspect)

    p_synth = sub.add_parser("synthesize")
    p_synth.add_argument("--run", required=True)
    p_synth.add_argument("--branch")
    p_synth.add_argument("--skip-verify", action="store_true")
    p_synth.set_defaults(func=cmd_synthesize)

    p_report = sub.add_parser("report")
    p_report.add_argument("--run", required=True)
    p_report.set_defaults(func=cmd_report)

    p_close = sub.add_parser("close")
    p_close.add_argument("--run", required=True)
    p_close.set_defaults(func=cmd_close)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    try:
        ensure_dirs()
        return int(args.func(args))
    except CmdError as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
