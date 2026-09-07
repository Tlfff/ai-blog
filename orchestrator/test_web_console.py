import unittest
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

from web_console import change_stats, normalize


@dataclass
class Example:
    path: Path


class WebConsoleTests(unittest.TestCase):
    def test_normalize_serializes_pipeline_values(self):
        value = normalize(
            {
                "config": Example(Path("/tmp/issue-5")),
                "started": datetime(2026, 9, 4, tzinfo=timezone.utc),
                "paths": (Path("a.go"),),
            }
        )
        self.assertEqual(value["config"]["path"], "/tmp/issue-5")
        self.assertEqual(value["paths"], ["a.go"])
        self.assertTrue(value["started"].startswith("2026-09-04"))

    def test_missing_historical_worktree_has_safe_empty_stats(self):
        self.assertEqual(
            change_stats("/path/that/does/not/exist"),
            {"files": 0, "untracked": 0, "additions": 0, "deletions": 0},
        )


if __name__ == "__main__":
    unittest.main()
