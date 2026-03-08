#!/usr/bin/env python3
"""Codex streaming probe.

用途：
1. 直接观察 `codex app-server` 的细粒度事件流；
2. 对比旧的 `codex exec --json` 输出；
3. 帮助判断“为什么答案是流式的，而思考看起来是一段一段的”。

使用方式：
- 先编辑本文件顶部配置区，填入 API KEY / BASE URL / PROMPT。
- 运行：`python3 scripts/debug_codex_stream_probe.py`
- 如需对比旧实现：`python3 scripts/debug_codex_stream_probe.py --mode exec-json`

说明：
- 若 `OPENAI_API_KEY` 非空且 `AUTO_LOGIN_WITH_API_KEY=True`，脚本会先执行
  `codex login --with-api-key`，把 key 写入本机 Codex 登录状态。
- `app-server` 模式最接近当前项目里新的 Codex chat 实现。
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import textwrap
import threading
import time
from pathlib import Path
from typing import Any


# ====== 配置区：你只需要改这里 ======
CODEX_BIN = "codex"
MODE = "app-server"  # 可选: app-server / exec-json
MODEL = "gpt-5-codex"
PROMPT = "请先输出几段思考过程，再给出一个结构化回答，主题是：如何优化 Codex CLI 的流式输出体验？"
SYSTEM_PROMPT = "你是一个调试演示助手。请正常思考并回答，不要省略中间过程。"
WORKSPACE = "."
OPENAI_API_KEY = ""
OPENAI_BASE_URL = ""
AUTO_LOGIN_WITH_API_KEY = True
TIMEOUT_SECONDS = 180


def now() -> str:
    return time.strftime("%H:%M:%S")


def print_banner(title: str) -> None:
    print("\n" + "=" * 20 + f" {title} " + "=" * 20)


def run_login_if_needed(codex_bin: str, env: dict[str, str]) -> None:
    api_key = env.get("OPENAI_API_KEY", "").strip()
    if not api_key or not AUTO_LOGIN_WITH_API_KEY:
        return
    print_banner("codex login")
    proc = subprocess.run(
        [codex_bin, "login", "--with-api-key"],
        input=api_key,
        text=True,
        env=env,
        capture_output=True,
    )
    print(proc.stdout.strip())
    if proc.returncode != 0:
        print(proc.stderr.strip(), file=sys.stderr)
        raise SystemExit(f"codex login failed: exit={proc.returncode}")


def build_env() -> dict[str, str]:
    env = os.environ.copy()
    if OPENAI_API_KEY.strip():
        env["OPENAI_API_KEY"] = OPENAI_API_KEY.strip()
    if OPENAI_BASE_URL.strip():
        env["OPENAI_BASE_URL"] = OPENAI_BASE_URL.strip()
    return env


def json_dumps(obj: Any) -> str:
    return json.dumps(obj, ensure_ascii=False)


def start_app_server(codex_bin: str, env: dict[str, str]) -> subprocess.Popen[str]:
    return subprocess.Popen(
        [codex_bin, "app-server", "--listen", "stdio://"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        cwd=WORKSPACE,
        env=env,
        bufsize=1,
    )


class RpcClient:
    def __init__(self, proc: subprocess.Popen[str]) -> None:
        self.proc = proc
        self._next_id = 1
        self._responses: dict[str, dict[str, Any]] = {}
        self._lock = threading.Lock()
        self._done = False
        self.notifications: list[dict[str, Any]] = []
        self.stderr_lines: list[str] = []
        self.reader = threading.Thread(target=self._read_stdout, daemon=True)
        self.reader.start()
        self.stderr_reader = threading.Thread(target=self._read_stderr, daemon=True)
        self.stderr_reader.start()

    def _read_stdout(self) -> None:
        assert self.proc.stdout is not None
        for raw in self.proc.stdout:
            line = raw.strip()
            if not line:
                continue
            try:
                msg = json.loads(line)
            except Exception:
                print(f"[{now()}] RAW {line}")
                continue
            if "id" in msg and "method" not in msg:
                with self._lock:
                    self._responses[str(msg["id"])] = msg
                continue
            self.notifications.append(msg)
            method = msg.get("method", "")
            params = msg.get("params", {})
            pretty_print_notification(method, params)
        self._done = True

    def _read_stderr(self) -> None:
        assert self.proc.stderr is not None
        for raw in self.proc.stderr:
            line = raw.rstrip("\n")
            if line:
                self.stderr_lines.append(line)
                print(f"[{now()}] STDERR {line}", file=sys.stderr)

    def request(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        req_id = self._next_id
        self._next_id += 1
        payload = {"id": req_id, "method": method, "params": params}
        assert self.proc.stdin is not None
        self.proc.stdin.write(json_dumps(payload) + "\n")
        self.proc.stdin.flush()
        deadline = time.time() + TIMEOUT_SECONDS
        while time.time() < deadline:
            with self._lock:
                if str(req_id) in self._responses:
                    return self._responses.pop(str(req_id))
            if self.proc.poll() is not None:
                raise RuntimeError(f"app-server exited early: rc={self.proc.returncode}")
            time.sleep(0.02)
        raise TimeoutError(f"request timeout: {method}")

    def notify(self, method: str, params: dict[str, Any] | None = None) -> None:
        payload: dict[str, Any] = {"method": method}
        if params is not None:
            payload["params"] = params
        assert self.proc.stdin is not None
        self.proc.stdin.write(json_dumps(payload) + "\n")
        self.proc.stdin.flush()


def pretty_print_notification(method: str, params: dict[str, Any]) -> None:
    prefix = f"[{now()}] {method}"
    if method == "item/agentMessage/delta":
        print(f"{prefix}  ANSWER += {params.get('delta', '')!r}")
        return
    if method in ("item/reasoning/summaryTextDelta", "item/reasoning/textDelta"):
        print(f"{prefix}  THINK += {params.get('delta', '')!r}")
        return
    if method == "item/plan/delta":
        print(f"{prefix}  PLAN += {params.get('delta', '')!r}")
        return
    if method == "thread/tokenUsage/updated":
        print(f"{prefix}  USAGE = {json.dumps(params.get('tokenUsage', {}), ensure_ascii=False)}")
        return
    if method == "item/completed":
        item = params.get("item", {})
        item_type = item.get("type", "")
        print(f"{prefix}  ITEM_COMPLETED type={item_type} item={json.dumps(item, ensure_ascii=False)}")
        return
    if method == "turn/completed":
        print(f"{prefix}  TURN = {json.dumps(params.get('turn', {}), ensure_ascii=False)}")
        return
    if method == "thread/started":
        print(f"{prefix}  THREAD = {json.dumps(params, ensure_ascii=False)}")
        return
    print(f"{prefix}  {json.dumps(params, ensure_ascii=False)}")


def run_app_server_mode() -> None:
    env = build_env()
    run_login_if_needed(CODEX_BIN, env)
    print_banner("start codex app-server")
    proc = start_app_server(CODEX_BIN, env)
    client = RpcClient(proc)
    try:
        init_resp = client.request(
            "initialize",
            {
                "clientInfo": {"name": "vibe_tree_probe", "title": "vibe-tree probe", "version": "0.1.0"},
                "capabilities": {"experimentalApi": True},
            },
        )
        print(f"[{now()}] initialize => {json.dumps(init_resp.get('result', {}), ensure_ascii=False)}")
        client.notify("initialized")

        thread_resp = client.request(
            "thread/start",
            {
                "model": MODEL,
                "cwd": str(Path(WORKSPACE).resolve()),
                "approvalPolicy": "never",
                "sandbox": "danger-full-access",
                "baseInstructions": SYSTEM_PROMPT,
            },
        )
        thread_id = thread_resp["result"]["thread"]["id"]
        print(f"[{now()}] thread/start => thread_id={thread_id}")

        turn_resp = client.request(
            "turn/start",
            {
                "threadId": thread_id,
                "input": [
                    {
                        "type": "text",
                        "text": PROMPT,
                        "textElements": [],
                    }
                ],
            },
        )
        print(f"[{now()}] turn/start => {json.dumps(turn_resp.get('result', {}), ensure_ascii=False)}")

        deadline = time.time() + TIMEOUT_SECONDS
        while time.time() < deadline:
            if proc.poll() is not None:
                break
            if any(note.get("method") == "turn/completed" for note in client.notifications):
                break
            time.sleep(0.05)
        else:
            raise TimeoutError("turn did not complete in time")
    finally:
        try:
            if proc.stdin:
                proc.stdin.close()
        except Exception:
            pass
        try:
            proc.terminate()
        except Exception:
            pass
        try:
            proc.wait(timeout=3)
        except Exception:
            try:
                proc.kill()
            except Exception:
                pass


def run_exec_json_mode() -> None:
    env = build_env()
    run_login_if_needed(CODEX_BIN, env)
    print_banner("start codex exec --json")
    cmd = [
        CODEX_BIN,
        "exec",
        "--json",
        "--skip-git-repo-check",
        "--dangerously-bypass-approvals-and-sandbox",
        "--model",
        MODEL,
        "-",
    ]
    proc = subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        cwd=WORKSPACE,
        env=env,
        bufsize=1,
    )
    assert proc.stdin is not None
    proc.stdin.write(f"System instructions:\n{SYSTEM_PROMPT}\n\nUser request:\n{PROMPT}")
    proc.stdin.close()
    assert proc.stdout is not None
    for raw in proc.stdout:
        line = raw.rstrip("\n")
        if not line:
            continue
        print(f"[{now()}] RAW {line}")
        try:
            obj = json.loads(line)
        except Exception:
            continue
        if obj.get("type") == "item.completed":
            item = obj.get("item", {})
            print(f"[{now()}] PARSED item.completed type={item.get('type')} text={item.get('text')!r}")
    proc.wait(timeout=TIMEOUT_SECONDS)
    print(f"[{now()}] exec exit={proc.returncode}")


def main() -> None:
    parser = argparse.ArgumentParser(description="Probe Codex streaming behavior")
    parser.add_argument("--mode", choices=["app-server", "exec-json"], default=MODE)
    args = parser.parse_args()

    print(textwrap.dedent(
        f"""
        配置摘要:
          mode={args.mode}
          model={MODEL}
          workspace={Path(WORKSPACE).resolve()}
          base_url={(OPENAI_BASE_URL or '(default)').strip()}
          api_key={'SET' if OPENAI_API_KEY.strip() else 'EMPTY'}
        """
    ).strip())

    if args.mode == "app-server":
        run_app_server_mode()
    else:
        run_exec_json_mode()


if __name__ == "__main__":
    main()
