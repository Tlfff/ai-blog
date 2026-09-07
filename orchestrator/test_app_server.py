import unittest

from app_server import AppServerClient


class AppServerMetadataTests(unittest.TestCase):
    def test_thread_metadata_is_remembered(self):
        client = AppServerClient()
        client._remember_thread_info(
            {
                "model": "gpt-5.6-luna",
                "modelProvider": "custom",
                "reasoningEffort": "medium",
                "serviceTier": "default",
            }
        )
        self.assertEqual(client.thread_info["model"], "gpt-5.6-luna")
        self.assertEqual(client.thread_info["model_provider"], "custom")
        self.assertEqual(client.thread_info["reasoning_effort"], "medium")


if __name__ == "__main__":
    unittest.main()
