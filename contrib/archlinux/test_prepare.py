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
    def source(self, root, local=False, version='0.10.0'):
        path = root / 'source.tar.gz'
        with tarfile.open(path, 'w:gz') as tar:
            files = {'go.mod': 'module github.com/zelport/zelport\ngo 1.27.0\n' + ('require github.com/zelport/zelport-sdk-go v0.0.0-local.abc\n' if local else ''), 'Makefile': 'build:\n', 'LICENSE': 'MIT', 'THIRD_PARTY_NOTICES.txt': 'notices', 'contrib/systemd/zelport.service': '[Service]\n'}
            for name, text in files.items():
                data = text.encode()
                info = tarfile.TarInfo(f'zelport-{version}/' + name)
                info.size = len(data)
                tar.addfile(info, io.BytesIO(data))
        return path

    def test_source_recipe_and_immutability(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = self.source(root)
            out = m.prepare('0.10.0', source, root / 'out')
            recipe = (out / 'PKGBUILD').read_text()
            srcinfo = (out / '.SRCINFO').read_text()
            self.assertIn(hashlib.sha256(source.read_bytes()).hexdigest(), recipe)
            self.assertIn('pkgname=zelport\n', recipe)
            self.assertIn('pkgver=0.10.0\n', recipe)
            self.assertIn('/v${_upstream_version}/zelport_${_upstream_version}_source.tar.gz', recipe)
            self.assertIn('make build', recipe)
            self.assertNotIn('make build-' + 'full', recipe)
            self.assertNotIn('@VERSION@', recipe)
            self.assertNotIn('source_x86_64', recipe)
            self.assertIn('pkgver = 0.10.0', srcinfo)
            self.assertIn(hashlib.sha256(source.read_bytes()).hexdigest(), srcinfo)
            self.assertNotIn('@VERSION@', srcinfo)
            with self.assertRaises(FileExistsError):
                m.prepare('0.10.0', source, out)

    def test_local_dependencies_are_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with self.assertRaisesRegex(ValueError, 'local'):
                m.prepare('0.10.0', self.source(root, True), root / 'out')
            self.assertFalse((root / 'out').exists())

    def test_invalid_version_and_wrong_archive_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = self.source(root)
            for version in ['v1.0.0', 'dev', '1.0.0; echo bad', '1.0.0-' + 'pre' + 'view.1', '01.0.0']:
                with self.assertRaises(ValueError):
                    m.prepare(version, self.source(root, version=version), root / 'out')
            with self.assertRaisesRegex(ValueError, 'Missing source file'):
                m.prepare('1.0.0', source, root / 'out')
            self.assertFalse((root / 'out').exists())

if __name__ == '__main__':
    unittest.main()
