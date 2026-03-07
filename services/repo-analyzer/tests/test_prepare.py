import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

APP_DIR = Path(__file__).resolve().parents[1] / "app"
if str(APP_DIR) not in sys.path:
    sys.path.insert(0, str(APP_DIR))

from cli import build_parser
from ingest import run_prepare


class PrepareCommandTest(unittest.TestCase):
    def test_build_parser_accepts_prepare_without_feature(self) -> None:
        parser = build_parser()
        args = parser.parse_args(
            [
                "prepare",
                "--repo-url",
                "https://github.com/octocat/Hello-World",
                "--storage-root",
                "/tmp/repo-library",
                "--output",
                "/tmp/repo-library/prepare.json",
            ]
        )

        self.assertEqual(args.command, "prepare")
        self.assertEqual(args.ref, "main")
        self.assertFalse(hasattr(args, "feature"))

    def test_run_prepare_creates_snapshot_and_code_index_without_report(self) -> None:
        def fake_run_json_command(cmd: list[str], **_: object) -> dict[str, object]:
            command_name = Path(cmd[1]).name
            if command_name == "fetch_repo.py":
                source_dir = Path(cmd[cmd.index("--source-dir") + 1])
                (source_dir / "README.md").write_text("# demo\n", encoding="utf-8")
                return {"resolved_ref": "main", "commit_sha": "abcdef1234567890"}
            if command_name == "build_code_index.py":
                output_path = Path(cmd[cmd.index("--output") + 1])
                output_path.write_text(json.dumps({"files": 1}), encoding="utf-8")
                return {"indexed_files": 1}
            raise AssertionError(f"unexpected external command: {cmd}")

        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            storage_root = root / "storage"

            with patch("ingest.analyzer_script", side_effect=lambda name: Path(f"/mock/{name}")):
                with patch("ingest.run_json_command", side_effect=fake_run_json_command):
                    payload = run_prepare(
                        repo_url="https://github.com/octocat/Hello-World",
                        ref="main",
                        storage_root=str(storage_root),
                        run_id="demo-run",
                    )

            self.assertEqual(payload["status"], "ok")
            self.assertEqual(payload["command"], "prepare")
            self.assertFalse(payload["report_ready"])
            self.assertEqual(payload["run"]["status"], "prepared")
            self.assertFalse(Path(payload["report_path"]).exists())
            self.assertTrue(Path(payload["snapshot"]["source_dir"]).exists())
            self.assertTrue(Path(payload["snapshot"]["code_index_path"]).exists())
            self.assertEqual(payload["prepare"]["report"]["status"], "pending")


if __name__ == "__main__":
    unittest.main()
