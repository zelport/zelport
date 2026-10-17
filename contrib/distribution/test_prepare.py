import hashlib
import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('prepare', Path(__file__).with_name('prepare.py'))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

class PackagingTests(unittest.TestCase):
    def source(self, root, local=False):
        path = root / 'source.tar.gz'
        with tarfile.open(path, 'w:gz') as tar:
            files = {'go.mod': 'module github.com/zelport/zelport\ngo 1.27.0\n' + ('require github.com/zelport/zelport-sdk-go v0.0.0-local.abc\n' if local else ''), 'Makefile': 'build:\n', 'LICENSE': 'MIT', 'THIRD_PARTY_NOTICES.txt': 'notices', 'contrib/systemd/zelport.service': '[Service]\n'}
            for name, text in files.items():
                data = text.encode()
                info = tarfile.TarInfo('zelport-0.1.0-preview.3/' + name)
                info.size = len(data)
                tar.addfile(info, io.BytesIO(data))
        return path

    def test_source_recipe_and_immutability(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = self.source(root)
            out = m.prepare('0.1.0-preview.3', source, root / 'out')
            recipe = (out / 'PKGBUILD').read_text()
            self.assertIn(hashlib.sha256(source.read_bytes()).hexdigest(), recipe)
            self.assertIn('pkgname=zelport\n', recipe)
            self.assertIn('pkgver=0.1.0preview.3\n', recipe)
            self.assertIn('make build', recipe)
            self.assertNotIn('make build-full', recipe)
            self.assertNotIn('@VERSION@', recipe)
            self.assertNotIn('source_x86_64', recipe)
            with self.assertRaises(FileExistsError):
                m.prepare('0.1.0-preview.3', source, out)

    def test_local_dependencies_are_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with self.assertRaisesRegex(ValueError, 'local'):
                m.prepare('0.1.0-preview.3', self.source(root, True), root / 'out')
            self.assertFalse((root / 'out').exists())

    def test_invalid_version_and_wrong_archive_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = self.source(root)
            for version in ['v1.0.0', 'dev', '1.0.0; echo bad', '1.0.0']:
                with self.assertRaises(ValueError):
                    m.prepare(version, source, root / 'out')
            self.assertFalse((root / 'out').exists())

if __name__ == '__main__':
    unittest.main()
