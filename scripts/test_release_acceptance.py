import hashlib
import importlib.util
import io
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import time
import unittest


spec = importlib.util.spec_from_file_location(
    'check_release_binary', Path(__file__).with_name('check_release_binary.py'))
acceptance = importlib.util.module_from_spec(spec)
spec.loader.exec_module(acceptance)


class ReleaseArchiveTest(unittest.TestCase):
    version = '0.10.0'
    platform = 'linux_amd64'

    def make_bundle(self, root, *, members=None, checksum=True, digest=None):
        bundle = root / 'bundle'
        bundle.mkdir()
        archive = bundle / f'zelport_{self.version}_{self.platform}.tar.gz'
        if members is None:
            members = [
                ('zelport', b'binary', 0o755, tarfile.REGTYPE),
                ('LICENSE', b'license', 0o644, tarfile.REGTYPE),
                ('THIRD_PARTY_NOTICES.txt', b'notices', 0o644, tarfile.REGTYPE),
            ]
        with tarfile.open(archive, 'w:gz') as output:
            for name, content, mode, kind in members:
                info = tarfile.TarInfo(name)
                info.mode = mode
                info.type = kind
                info.size = len(content)
                output.addfile(info, io.BytesIO(content))
        if checksum:
            actual = hashlib.sha256(archive.read_bytes()).hexdigest()
            (bundle / 'SHA256SUMS').write_text(
                f'{digest or actual}  {archive.name}\n', encoding='utf-8')
        return bundle

    def verify(self, bundle, destination, platform=None):
        return acceptance.verify_archive(
            bundle, self.version, platform or self.platform, destination)

    def test_valid_archive_is_verified_and_extracted(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            binary = self.verify(self.make_bundle(root), root / 'extract')
            self.assertEqual(binary.read_bytes(), b'binary')
            self.assertEqual(binary.stat().st_mode & 0o777, 0o755)
            self.assertEqual((binary.parent / 'LICENSE').stat().st_mode & 0o777, 0o644)

    def test_rejects_checksum_mismatch(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            bundle = self.make_bundle(root, digest='0' * 64)
            with self.assertRaisesRegex(ValueError, 'checksum'):
                self.verify(bundle, root / 'extract')

    def test_rejects_missing_checksum(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            bundle = self.make_bundle(root, checksum=False)
            with self.assertRaisesRegex((FileNotFoundError, ValueError), 'SHA256SUMS'):
                self.verify(bundle, root / 'extract')

    def test_rejects_extra_or_unsafe_member(self):
        valid = [
            ('zelport', b'binary', 0o755, tarfile.REGTYPE),
            ('LICENSE', b'license', 0o644, tarfile.REGTYPE),
            ('THIRD_PARTY_NOTICES.txt', b'notices', 0o644, tarfile.REGTYPE),
        ]
        for member in [('extra', b'x', 0o644, tarfile.REGTYPE),
                       ('../zelport', b'x', 0o755, tarfile.REGTYPE)]:
            with self.subTest(member=member[0]), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                bundle = self.make_bundle(root, members=valid + [member])
                with self.assertRaisesRegex(ValueError, 'members'):
                    self.verify(bundle, root / 'extract')

    def test_rejects_wrong_member_mode(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            bundle = self.make_bundle(root, members=[
                ('zelport', b'binary', 0o644, tarfile.REGTYPE),
                ('LICENSE', b'license', 0o644, tarfile.REGTYPE),
                ('THIRD_PARTY_NOTICES.txt', b'notices', 0o644, tarfile.REGTYPE),
            ])
            with self.assertRaisesRegex(ValueError, 'mode'):
                self.verify(bundle, root / 'extract')

    def test_rejects_unknown_platform(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            bundle = self.make_bundle(root)
            with self.assertRaisesRegex(ValueError, 'platform'):
                self.verify(bundle, root / 'extract', 'plan9_mips')


class ReleaseProcessTest(unittest.TestCase):
    def executable(self, root, body):
        path = root / 'fixture.py'
        path.write_text(f'#!{sys.executable}\n{body}', encoding='utf-8')
        path.chmod(0o755)
        return path

    def assert_process_gone(self, pid_file):
        pid = int(pid_file.read_text(encoding='utf-8'))
        with self.assertRaises(ProcessLookupError):
            os.kill(pid, 0)

    def test_rejects_wrong_version_before_startup(self):
        with tempfile.TemporaryDirectory() as tmp:
            binary = self.executable(Path(tmp), '''
import sys
if '--version' in sys.argv:
    print('zelport 9.9.9 (built fixture)')
''')
            with self.assertRaisesRegex(AssertionError, 'version'):
                acceptance.run_app_acceptance(binary, '0.10.0', 'linux_amd64')

    def test_reports_early_exit_and_captured_log(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pid_file = root / 'pid'
            binary = self.executable(root, f'''
import os
import sys
if '--version' in sys.argv:
    print('zelport 0.10.0 (built fixture)')
    raise SystemExit(0)
open({str(pid_file)!r}, 'w').write(str(os.getpid()))
print('synthetic early failure', file=sys.stderr, flush=True)
raise SystemExit(17)
''')
            with self.assertRaisesRegex(AssertionError, 'synthetic early failure'):
                acceptance.run_app_acceptance(
                    binary, '0.10.0', 'linux_amd64', startup_timeout=.2)
            self.assert_process_gone(pid_file)

    def test_startup_timeout_terminates_child_and_reports_log(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pid_file = root / 'pid'
            binary = self.executable(root, f'''
import os
import sys
import time
if '--version' in sys.argv:
    print('zelport 0.10.0 (built fixture)')
    raise SystemExit(0)
open({str(pid_file)!r}, 'w').write(str(os.getpid()))
print('synthetic stalled startup', file=sys.stderr, flush=True)
while True:
    time.sleep(1)
''')
            with self.assertRaisesRegex(AssertionError, 'synthetic stalled startup'):
                acceptance.run_app_acceptance(
                    binary, '0.10.0', 'linux_amd64', startup_timeout=.2)
            self.assert_process_gone(pid_file)

    def test_ignored_sigterm_is_killed_and_reported(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            marker = root / 'ready'
            log_path = root / 'process.log'
            binary = self.executable(root, f'''
import signal
import time
from pathlib import Path
signal.signal(signal.SIGTERM, lambda *_: None)
Path({str(marker)!r}).write_text('ready')
print('synthetic ignored SIGTERM', flush=True)
while True:
    time.sleep(1)
''')
            with log_path.open('w') as log:
                process = subprocess.Popen(
                    [str(binary)], stdout=log, stderr=subprocess.STDOUT)
                for _ in range(100):
                    if marker.exists():
                        break
                    time.sleep(.01)
                else:
                    self.fail('fixture did not start')
                with self.assertRaisesRegex(AssertionError, 'synthetic ignored SIGTERM'):
                    acceptance.terminate_process(process, log_path, timeout=.1)
            self.assertIsNotNone(process.poll())


if __name__ == '__main__':
    unittest.main()
