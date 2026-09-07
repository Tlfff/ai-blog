import tempfile
import unittest
from pathlib import Path

from workflow import ChangeSet, ContextPack, ReviewPackBuilder, ValidationStepResult


class ReviewPackTests(unittest.TestCase):
    def test_review_pack_splits_generated_files_and_extracts_relevant_sections(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        (root / "backend").mkdir()
        (root / "backend" / "功能文档.md").write_text(
            "# 登录\n无关内容\n\n# 创建文章与正文图片\n图片上传、文章创建和后台详情。\n"
        )
        (root / "docs" / "specs").mkdir(parents=True)
        (root / "docs" / "specs" / "blog-backend-tickets.md").write_text(
            "# 用户\n登录\n\n# 文章\n正文图片和创建文章使用事务。\n"
        )
        changes = ChangeSet(
            paths=("backend/api/article/article.pb.go", "backend/internal/domain/article/service.go"),
            tracked_paths=(),
            untracked_paths=(),
            fingerprint="x",
        )
        pack = ReviewPackBuilder.build(
            worktree=root,
            change_set=changes,
            context_pack=ContextPack((), 1, "父规格", "文章上下文不得跨库直写。"),
            issue_title="创建文章与正文图片",
            issue_body="图片上传、创建文章和后台详情",
            validation_steps=(ValidationStepResult("测试", "go test", 0, "ok"),),
        )
        self.assertEqual(pack.generated_paths, ("backend/api/article/article.pb.go",))
        self.assertEqual(pack.handwritten_paths, ("backend/internal/domain/article/service.go",))
        self.assertIn("创建文章与正文图片", pack.spec_excerpt)
        self.assertNotIn("# 登录", pack.spec_excerpt)
        self.assertIn("PASS", pack.validation_summary)


if __name__ == "__main__":
    unittest.main()
