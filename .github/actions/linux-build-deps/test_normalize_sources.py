from pathlib import Path
from tempfile import TemporaryDirectory
import unittest

from normalize_sources import normalize


class SourcesContract(unittest.TestCase):
    def test_all_active_formats_and_idempotence(self):
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            directory = root / "sources.list.d"
            directory.mkdir()
            files = {
                "sources.list": "deb http://azure.archive.ubuntu.com/ubuntu/ noble main\n",
                "sources.list.d/ubuntu.sources": "Types: deb\nURIs: https://azure.archive.ubuntu.com/ubuntu\nSuites: noble\n",
                "sources.list.d/updates.list": "deb http://archive.ubuntu.com/ubuntu noble-updates main\n",
                "sources.list.d/third-party.list": "deb https://packages.microsoft.com/repos/code stable main\n",
                "sources.list.d/ubuntu.sources.save": "URIs: http://azure.archive.ubuntu.com/ubuntu\n",
            }
            for name, content in files.items():
                (root / name).write_text(content)
            self.assertEqual(len(normalize(root)), 3)
            for name in list(files)[:3]:
                self.assertIn("https://archive.ubuntu.com/ubuntu", (root / name).read_text())
                self.assertNotIn("azure.archive", (root / name).read_text())
            for name in list(files)[3:]:
                self.assertEqual((root / name).read_text(), files[name])
            self.assertEqual(normalize(root), [])

    def test_minimal_configuration_and_host_boundaries(self):
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.assertEqual(normalize(root), [])
            content = "deb https://azure.archive.ubuntu.com.example/ubuntu noble main\ndeb https://archive.ubuntu.com/other noble main\n"
            (root / "sources.list").write_text(content)
            self.assertEqual(normalize(root), [])
            self.assertEqual((root / "sources.list").read_text(), content)


if __name__ == "__main__":
    unittest.main()
