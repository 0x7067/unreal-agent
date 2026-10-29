import subprocess
import tempfile
import unittest
from pathlib import Path

from harness_harbor.agent import session_export


class SessionExportTest(unittest.TestCase):
    def run_export(self, runner_status: int, sessions: Path) -> tuple[int, Path]:
        """Run the export tail after a fake runner; return its exit and store."""
        script = (
            'store="$(mktemp -d)" || exit 1; '
            'printf %s "$store"; '
            ': > "$store/a.session.jsonl"; '
            f"(exit {runner_status}); " + session_export(str(sessions))
        )
        result = subprocess.run(["sh", "-c", script], capture_output=True, text=True)
        return result.returncode, Path(result.stdout)

    def test_copies_sessions_and_removes_store(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            sessions = Path(directory) / "sessions"
            status, store = self.run_export(0, sessions)
            self.assertEqual(status, 0)
            self.assertTrue((sessions / "a.session.jsonl").exists())
            self.assertFalse(store.exists())

    def test_keeps_runner_failure_and_removes_store(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            status, store = self.run_export(3, Path(directory) / "sessions")
            self.assertEqual(status, 3)
            self.assertFalse(store.exists())

    def test_reports_failed_export_after_runner_success(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            blocker = Path(directory) / "file"
            blocker.write_text("")
            status, store = self.run_export(0, blocker / "sessions")
            self.assertNotEqual(status, 0)
            self.assertFalse(store.exists())


if __name__ == "__main__":
    unittest.main()
