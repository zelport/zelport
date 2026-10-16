import hashlib
import importlib.util
from pathlib import Path
import tarfile
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('release', Path(__file__).with_name('release.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseTest(unittest.TestCase):
    def test_archives_and_formula_match(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            binaries = root / 'binaries'
            binaries.mkdir()
            for platform in release.PLATFORMS:
                (binaries / platform).write_bytes(platform.encode())
            output = root / 'release'
            release.package('1.2.3', binaries, output)
            formula = (output / 'Formula/zelport.rb').read_text()
            checksums = (output / 'SHA256SUMS').read_text()
            for platform in release.PLATFORMS:
                archive = output / f'zelport_1.2.3_{platform}.tar.gz'
                digest = hashlib.sha256(archive.read_bytes()).hexdigest()
                self.assertIn(digest, formula)
                self.assertIn(f'{digest}  {archive.name}', checksums)
                self.assertIn(f'/v1.2.3/{archive.name}', formula)
                with tarfile.open(archive) as tar:
                    self.assertEqual(set(tar.getnames()), {'zelport', 'LICENSE', 'THIRD_PARTY_NOTICES.txt'})
                    self.assertEqual(tar.extractfile('zelport').read(), platform.encode())
                    self.assertEqual(tar.getmember('zelport').mode, 0o755)
            self.assertIn('run [opt_bin/"zelport"]', formula)
            self.assertNotIn('--allow-root', formula)
            self.assertNotIn('--config', formula)
            with self.assertRaises(FileExistsError):
                release.package('1.2.3', binaries, output)

    def test_missing_binary_leaves_no_partial_release(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with self.assertRaises(FileNotFoundError):
                release.package('1.2.3', root, root / 'output')
            self.assertFalse((root / 'output').exists())

    def test_invalid_version_rejected(self):
        for version in ['dev', '../1.2.3', '1.2.3\"', 'v1.2.3']:
            with self.assertRaises(ValueError):
                release.package(version, Path('/nonexistent'), Path('/nonexistent'))


if __name__ == '__main__':
    unittest.main()
