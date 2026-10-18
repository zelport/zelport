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
            release.package('0.10.0', binaries, output)
            formula = (output / 'Formula/zelport.rb').read_text()
            checksums = (output / 'SHA256SUMS').read_text().splitlines()
            expected_checksums = []
            for platform in release.PLATFORMS:
                archive = output / f'zelport_0.10.0_{platform}.tar.gz'
                digest = hashlib.sha256(archive.read_bytes()).hexdigest()
                self.assertIn(digest, formula)
                expected_checksums.append(f'{digest}  {archive.name}')
                self.assertIn(f'/v0.10.0/{archive.name}', formula)
                with tarfile.open(archive) as tar:
                    self.assertEqual(set(tar.getnames()), {'zelport', 'LICENSE', 'THIRD_PARTY_NOTICES.txt'})
                    self.assertEqual(tar.extractfile('zelport').read(), platform.encode())
                    self.assertEqual(tar.getmember('zelport').mode, 0o755)
                    self.assertEqual(tar.getmember('LICENSE').mode, 0o644)
                    self.assertEqual(tar.getmember('THIRD_PARTY_NOTICES.txt').mode, 0o644)
            source = output / 'zelport_0.10.0_source.tar.gz'
            recipe = output / 'PKGBUILD'
            srcinfo = output / 'zelport-0.10.0.SRCINFO'
            source_digest = hashlib.sha256(source.read_bytes()).hexdigest()
            for asset in [source, recipe, srcinfo]:
                expected_checksums.append(
                    f'{hashlib.sha256(asset.read_bytes()).hexdigest()}  {asset.name}')
            with tarfile.open(source) as tar:
                self.assertIn('zelport-0.10.0/go.mod', tar.getnames())
                self.assertIn('zelport-0.10.0/contrib/systemd/zelport.service', tar.getnames())
            self.assertIn('pkgver=0.10.0', recipe.read_text())
            self.assertIn(source_digest, recipe.read_text())
            self.assertIn('pkgver = 0.10.0', srcinfo.read_text())
            self.assertIn(source_digest, srcinfo.read_text())
            self.assertEqual(checksums, expected_checksums)
            self.assertIn('version "0.10.0"', formula)
            self.assertNotIn('revision ', formula)
            self.assertIn('run [opt_bin/"zelport"]', formula)
            self.assertNotIn('--allow-root', formula)
            self.assertNotIn('--config', formula)
            with self.assertRaises(FileExistsError):
                release.package('0.10.0', binaries, output)

    def test_release_build_uses_standard_composition(self):
        command = release.build_command('0.10.0', '2026-10-17 12:00:00 UTC', Path('/tmp/zelport'))
        self.assertEqual(command[-1], './cmd/zelport')
        self.assertNotIn('-tags', command)
        self.assertIn('-X main.version=0.10.0', command[4])

    def test_missing_binary_leaves_no_partial_release(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with self.assertRaises(FileNotFoundError):
                release.package('1.2.3', root, root / 'output')
            self.assertFalse((root / 'output').exists())

    def test_invalid_version_rejected(self):
        for version in ['dev', '../1.2.3', '1.2.3\"', 'v1.2.3', '1.2.3-rc.1', '1.2.3-preview.1', '01.2.3']:
            with self.assertRaises(ValueError):
                release.package(version, Path('/nonexistent'), Path('/nonexistent'))


if __name__ == '__main__':
    unittest.main()
