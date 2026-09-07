"""真实 MySQL ROW Binlog、Redis 进度和 Meilisearch 文档集成测试。"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path
from urllib import error, request

RUN_INTEGRATION = os.getenv("MEILISYNC_INTEGRATION") == "1"


@unittest.skipUnless(RUN_INTEGRATION, "set MEILISYNC_INTEGRATION=1 to run Docker integration")
class MeilisyncIntegrationTest(unittest.TestCase):
    """验证应用实际单行写入产生的三类 ROW 事件和公开搜索契约。"""

    @classmethod
    def setUpClass(cls) -> None:
        suffix = str(os.getpid())
        cls.mysql = f"ai-blog-search-mysql-{suffix}"
        cls.redis = f"ai-blog-search-redis-{suffix}"
        cls.meili = f"ai-blog-search-meili-{suffix}"
        cls.runner: subprocess.Popen[str] | None = None
        cls.config_file: str | None = None
        try:
            cls._run("docker", "run", "-d", "--name", cls.mysql, "-e", "MYSQL_ROOT_PASSWORD=root", "-e", "MYSQL_DATABASE=blog", "-p", "127.0.0.1::3306", "mysql:8.0", "--server-id=2140014", "--log-bin=mysql-bin", "--binlog-format=ROW")
            cls._run("docker", "run", "-d", "--name", cls.redis, "-p", "127.0.0.1::6379", "redis:7-alpine")
            cls._run("docker", "run", "-d", "--name", cls.meili, "-e", "MEILI_ENV=development", "-p", "127.0.0.1::7700", "getmeili/meilisearch:v1.53.1")
            cls.mysql_port = cls._port(cls.mysql, "3306/tcp")
            cls.redis_port = cls._port(cls.redis, "6379/tcp")
            cls.meili_port = cls._port(cls.meili, "7700/tcp")
            cls._wait(lambda: cls._run("docker", "exec", cls.mysql, "mysql", "-uroot", "-proot", "blog", "-e", "SELECT 1", check=False).returncode == 0)
            cls._wait(lambda: cls._http("GET", "/health").get("status") == "available")
            cls._mysql(
                """
CREATE TABLE articles (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  author_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
  title VARCHAR(255) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  tags VARCHAR(255) NOT NULL DEFAULT '',
  status TINYINT NOT NULL,
  view_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
  like_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
  comment_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_time DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_time DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)
) DEFAULT CHARSET=utf8mb4;
"""
            )
            config = f"""debug: false
progress:
  type: redis
  dsn: redis://127.0.0.1:{cls.redis_port}/0
  key: blog:meilisync:articles:binlog-position
source:
  type: mysql
  server_id: {2141000 + os.getpid() % 1000}
  host: 127.0.0.1
  port: {cls.mysql_port}
  user: root
  password: root
  database: blog
  charset: utf8mb4
meilisearch:
  api_url: http://127.0.0.1:{cls.meili_port}
  api_key: ""
  insert_size: 500
  insert_interval: 1
sync:
  - table: articles
    index: articles
    full: false
    plugins:
      - meilisync_plugin.articles.ArticlePlugin
"""
            handle = tempfile.NamedTemporaryFile("w", suffix=".yml", delete=False, encoding="utf-8")
            handle.write(config)
            handle.close()
            cls.config_file = handle.name
            env = os.environ | {"MEILISEARCH_URL": f"http://127.0.0.1:{cls.meili_port}", "MEILISEARCH_API_KEY": "", "PYTHONDONTWRITEBYTECODE": "1"}
            cls._run(sys.executable, "-m", "meilisync_plugin.index_settings", env=env)
            cls.runner = subprocess.Popen(
                [sys.executable, "-m", "meilisync_plugin.runner", "-c", cls.config_file, "start"],
                cwd=Path(__file__).parents[1],
                env=env,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
            )
            cls._wait(lambda: "master_log_position" in cls._run("docker", "exec", cls.redis, "redis-cli", "HGETALL", "blog:meilisync:articles:binlog-position").stdout)
        except Exception:
            cls.tearDownClass()
            raise

    @classmethod
    def tearDownClass(cls) -> None:
        if cls.runner is not None:
            cls.runner.terminate()
            try:
                cls.runner.wait(timeout=5)
            except subprocess.TimeoutExpired:
                cls.runner.kill()
        if cls.config_file:
            Path(cls.config_file).unlink(missing_ok=True)
        subprocess.run(["docker", "rm", "-f", cls.mysql, cls.redis, cls.meili], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)

    def test_incremental_sync_and_public_search(self) -> None:
        self._mysql("INSERT INTO articles (id,title,content,tags,status) VALUES (1,'搜索入门','# 正文关键词\\n```go\\nsecret()\\n```\\n![图](image://1)','Go,中文,go',3),(2,'隐藏草稿','草稿正文','私有',2);")
        self._wait(lambda: {item["id"] for item in self._http("GET", "/indexes/articles/documents")["results"]} == {1, 2})
        documents = {item["id"]: item for item in self._http("GET", "/indexes/articles/documents")["results"]}
        self.assertEqual("sousuorumen", documents[1]["title_pinyin"])
        self.assertEqual("ssrm", documents[1]["title_initials"])
        self.assertEqual("正文关键词", documents[1]["content_plain"])
        self.assertEqual("Go 中文", documents[1]["tags"])
        self.assertEqual(2, documents[2]["status"])

        pinyin_result = self._http("POST", "/indexes/articles/search", {"q": "sousuo", "filter": "status = 3"})
        self.assertEqual([1], [hit["id"] for hit in pinyin_result["hits"]])
        result = self._http("POST", "/indexes/articles/search", {"q": "搜索", "filter": "status = 3", "attributesToHighlight": ["title"], "attributesToCrop": ["content_plain:50"], "highlightPreTag": "<em>", "highlightPostTag": "</em>"})
        self.assertEqual([1], [hit["id"] for hit in result["hits"]])
        self.assertIn("<em>", result["hits"][0]["_formatted"]["title"])

        self._mysql("UPDATE articles SET title=CASE id WHEN 1 THEN '搜索实战' ELSE title END, content=CASE id WHEN 1 THEN '更新正文摘要' ELSE content END, status=CASE id WHEN 2 THEN 1 ELSE status END WHERE id IN (1,2);")
        self._wait(lambda: self._http("GET", "/indexes/articles/documents/1").get("title") == "搜索实战" and self._http("GET", "/indexes/articles/documents/2").get("status") == 1)
        self.assertEqual([], self._http("POST", "/indexes/articles/search", {"q": "草稿", "filter": "status = 3"})["hits"])

        self._mysql("DELETE FROM articles WHERE id IN (1,2);")
        self._wait(self._documents_deleted)
        progress = self._run("docker", "exec", self.redis, "redis-cli", "HGETALL", "blog:meilisync:articles:binlog-position").stdout
        self.assertIn("master_log_position", progress)

    @classmethod
    def _documents_deleted(cls) -> bool:
        for document_id in (1, 2):
            try:
                cls._http("GET", f"/indexes/articles/documents/{document_id}")
            except error.HTTPError as exc:
                if exc.code == 404:
                    continue
            return False
        return True

    @classmethod
    def _mysql(cls, sql: str) -> None:
        subprocess.run(["docker", "exec", "-i", cls.mysql, "mysql", "--default-character-set=utf8mb4", "-uroot", "-proot", "blog"], input=sql, text=True, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    @classmethod
    def _port(cls, container: str, internal: str) -> int:
        output = cls._run("docker", "port", container, internal).stdout.splitlines()[0]
        return int(output.rsplit(":", 1)[1])

    @classmethod
    def _http(cls, method: str, path: str, payload: dict | None = None) -> dict:
        body = None if payload is None else json.dumps(payload).encode()
        req = request.Request(f"http://127.0.0.1:{cls.meili_port}{path}", data=body, method=method, headers={"Content-Type": "application/json"})
        with request.urlopen(req, timeout=5) as response:
            return json.load(response)

    @classmethod
    def _wait(cls, predicate, timeout: float = 40) -> None:
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            try:
                if predicate():
                    return
            except (error.URLError, error.HTTPError, KeyError, json.JSONDecodeError):
                pass
            time.sleep(0.5)
        runner_output = ""
        if cls.runner is not None and cls.runner.poll() is not None and cls.runner.stdout is not None:
            runner_output = cls.runner.stdout.read()
        raise AssertionError(f"condition timed out; runner output: {runner_output}")

    @staticmethod
    def _run(*args: str, check: bool = True, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
        return subprocess.run(args, check=check, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)


if __name__ == "__main__":
    unittest.main()
