"""Installer integration tests with isolated homes and fake system operations."""
import hashlib
import io
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class InstallTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.home = Path(self.tmp.name)
        self.assets = self.home / 'assets'
        self.assets.mkdir()
        self.payload = b'#!/bin/sh\necho "zelport 0.1.0-preview.5 (built fixture)"\n'
        self.archive = self.assets / 'zelport_0.1.0-preview.5_linux_amd64.tar.gz'
        with tarfile.open(self.archive, 'w:gz') as tar:
            member = tarfile.TarInfo('zelport')
            member.mode = 0o755
            member.size = len(self.payload)
            tar.addfile(member, io.BytesIO(self.payload))
            for name in ['LICENSE', 'THIRD_PARTY_NOTICES.txt']:
                member = tarfile.TarInfo(name)
                member.size = 7
                tar.addfile(member, io.BytesIO(b'license'))
        self.write_checksum()
        (self.assets / 'PKGBUILD').write_bytes((ROOT / 'contrib/archlinux/PKGBUILD').read_bytes())
        self.config = self.home / '.config/zelport/config.json'
        self.config.parent.mkdir(parents=True)
        self.config.write_text('preserve this configuration')

    def write_checksum(self):
        digest = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        (self.assets / 'SHA256SUMS').write_text(digest + '  ' + self.archive.name + '\n')

    def run_installer(self, args=(), os_name='Linux', distro='debian', arch='x86_64',
                      session=True, active=False, collision=False, uid=1000, start_ok=True,
                      other_install=False, readonly_cache=False, package_version="", enabled=False, package_complete=True):
        self.log = self.home / 'operations'
        program = r'''
source "$INSTALLER"
uname() { if [[ ${1:-} == -m ]]; then printf '%s\n' "$TEST_ARCH"; else printf '%s\n' "$TEST_OS"; fi; }
id() { printf '%s\n' "$TEST_UID"; }
detect_distro() { printf '%s\n' "$TEST_DISTRO"; }
fetch() { printf 'fetch %s\n' "$1" >> "$TEST_LOG"; cp "$TEST_ASSETS/${1##*/}" "$2"; }
systemctl() {
    printf 'systemctl %s\n' "$*" >> "$TEST_LOG"
    case "$*" in
        *show-environment*) [[ $TEST_SESSION == yes ]] ;;
        *FragmentPath*) return 0 ;;
        *is-active*) [[ $TEST_ACTIVE == yes ]] ;;
        *is-enabled*) [[ $TEST_ENABLED == yes ]] ;;
        *start*|*restart*) [[ $TEST_START_OK == yes ]] || return 1; TEST_ACTIVE=yes ;;
        *) return 0 ;;
    esac
}
ss() { if [[ $TEST_COLLISION == yes ]]; then echo 'LISTEN 127.0.0.1:8080'; fi; }
pacman() {
    [[ -n $TEST_PACKAGE_VERSION ]] || return 1
    if [[ ${1:-} == -Qk ]]; then [[ $TEST_PACKAGE_COMPLETE == yes ]]; return; fi
    printf 'zelport %s\n' "$TEST_PACKAGE_VERSION"
}
makepkg() {
    printf 'makepkg %s\n' "$*" >> "$TEST_LOG"
    if [[ $TEST_READONLY_CACHE == yes ]]; then
        printf 'build-dir %s\n' "$PWD" >> "$TEST_LOG"
        mkdir -p src/go/pkg/mod/example.test/dependency
        printf 'module source\n' > src/go/pkg/mod/example.test/dependency/source.go
        chmod 444 src/go/pkg/mod/example.test/dependency/source.go
        chmod 555 src/go/pkg/mod/example.test/dependency
        ln -s "$TEST_ASSETS" outside-link
    fi
}
sleep() { :; }
main "$@"
'''
        env = {**os.environ, 'HOME': str(self.home), 'XDG_CONFIG_HOME': str(self.home / '.config'),
               'XDG_DATA_HOME': str(self.home / '.local/share'),
               'PATH': str(self.home / '.local/bin') + ':/usr/bin:/bin:/usr/sbin:/sbin',
               'INSTALLER': str(ROOT / 'install.sh'), 'TEST_ASSETS': str(self.assets),
               'TEST_LOG': str(self.log), 'TEST_ARCH': arch, 'TEST_OS': os_name,
               'TEST_DISTRO': distro, 'TEST_UID': str(uid),
               'TEST_SESSION': 'yes' if session else 'no', 'TEST_ACTIVE': 'yes' if active else 'no',
               'TEST_COLLISION': 'yes' if collision else 'no', 'TEST_START_OK': 'yes' if start_ok else 'no',
               'TEST_READONLY_CACHE': 'yes' if readonly_cache else 'no',
               'TEST_PACKAGE_VERSION': package_version,
               'TEST_ENABLED': 'yes' if enabled else 'no',
               'TEST_PACKAGE_COMPLETE': 'yes' if package_complete else 'no'}
        if other_install:
            other = self.home / 'other/bin'
            other.mkdir(parents=True)
            binary = other / 'zelport'
            binary.write_text('#!/bin/sh\nexit 0\n')
            binary.chmod(0o755)
            env['PATH'] = str(other) + ':' + env['PATH']
        result = subprocess.run(['bash', '-c', program, 'installer', *args], env=env,
                                text=True, capture_output=True)
        self.assertEqual(self.config.read_text(), 'preserve this configuration')
        return result

    def ops(self):
        return self.log.read_text() if self.log.exists() else ''

    def test_platform_guidance_has_no_side_effects(self):
        for platform, message in [('Darwin', 'brew install zelport/tap/zelport'), ('MINGW64_NT', 'coming soon')]:
            with self.subTest(platform=platform):
                result = self.run_installer(os_name=platform)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn(message, result.stdout)
                self.assertEqual(self.ops(), '')

    def test_root_and_unsupported_arch_fail_before_download(self):
        for kw in [{'uid': 0}, {'arch': 'riscv64'}]:
            result = self.run_installer(**kw)
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn('fetch', self.ops())

    def test_binary_install_and_user_service(self):
        result = self.run_installer()
        self.assertEqual(result.returncode, 0, result.stderr)
        binary = self.home / '.local/bin/zelport'
        self.assertEqual(binary.read_bytes(), self.payload)
        self.assertTrue(os.access(binary, os.X_OK))
        self.assertEqual((self.home / '.local/share/zelport/THIRD_PARTY_NOTICES.txt').read_text(), 'license')
        unit = self.home / '.config/systemd/user/zelport.service'
        self.assertIn('ExecStart=%h/.local/bin/zelport', unit.read_text())
        self.assertIn('systemctl --user start zelport.service', self.ops())
        self.assertIn('systemctl --user enable zelport.service', self.ops())

    def test_no_systemd_installs_without_service(self):
        result = self.run_installer(session=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.home / '.local/bin/zelport').exists())
        self.assertFalse((self.home / '.config/systemd/user').exists())
        self.assertNotIn('start zelport', self.ops())
        self.assertIn('manually', result.stdout)

    def test_no_start_does_not_touch_systemd(self):
        result = self.run_installer(args=['--no-start'])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('systemctl', self.ops())

    def test_checksum_failure_preserves_existing_binary(self):
        binary = self.home / '.local/bin/zelport'
        binary.parent.mkdir(parents=True)
        binary.write_bytes(b'previous binary')
        (self.assets / 'SHA256SUMS').write_text('0' * 64 + '  ' + self.archive.name + '\n')
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(binary.read_bytes(), b'previous binary')
        self.assertNotIn('start zelport', self.ops())

    def test_symlink_binary_is_not_overwritten(self):
        target = self.home / 'other-program'
        target.write_text('do not overwrite')
        binary = self.home / '.local/bin/zelport'
        binary.parent.mkdir(parents=True)
        binary.symlink_to(target)
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(target.read_text(), 'do not overwrite')
        self.assertTrue(binary.is_symlink())

    def test_custom_unit_is_preserved(self):
        unit = self.home / '.config/systemd/user/zelport.service'
        unit.parent.mkdir(parents=True)
        unit.write_text('custom unit')
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(unit.read_text(), 'custom unit')
        self.assertFalse((self.home / '.local/bin/zelport').exists())

    def test_collision_does_not_start_or_stop_any_service(self):
        result = self.run_installer(collision=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('start zelport', self.ops())
        self.assertNotIn('stop zelport', self.ops())
        self.assertIn('8080', result.stderr)

    def test_update_restarts_managed_service(self):
        result = self.run_installer(active=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('systemctl --user restart zelport.service', self.ops())

    def test_failed_new_service_is_stopped_not_enabled(self):
        result = self.run_installer(start_ok=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('systemctl --user stop zelport.service', self.ops())
        self.assertNotIn('systemctl --user enable', self.ops())

    def test_arch_uses_verified_makepkg_recipe(self):
        result = self.run_installer(distro='arch', args=['--no-start'])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('makepkg --syncdeps --install', self.ops())
        self.assertFalse((self.home / '.local/bin/zelport').exists())

    def test_modified_arch_recipe_is_never_executed(self):
        (self.assets / 'PKGBUILD').write_text('exit 0\n')
        result = self.run_installer(distro='arch', args=['--no-start'])
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('makepkg ', self.ops())

    def test_arch_cleanup_removes_readonly_go_cache_without_following_symlinks(self):
        self.assets.chmod(0o555)
        try:
            result = self.run_installer(distro='arch', args=['--no-start'], readonly_cache=True)
            build_dir = Path(next(line.removeprefix('build-dir ') for line in self.ops().splitlines()
                                  if line.startswith('build-dir ')))
            # Ensure a failing regression run does not leave its own fixture behind.
            self.addCleanup(shutil.rmtree, build_dir, ignore_errors=True)
            self.addCleanup(subprocess.run, ['chmod', '-R', 'u+w', str(build_dir)], capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertNotIn('Permission denied', result.stderr)
            self.assertFalse(build_dir.exists())
            self.assertEqual(self.assets.stat().st_mode & 0o777, 0o555)
            self.assertTrue(self.archive.is_file())
        finally:
            self.assets.chmod(0o755)

    def test_cleanup_preserves_original_failure_status(self):
        build_dir = self.home / 'failed-build'
        build_dir.mkdir()
        result = subprocess.run(['bash', '-c', '''
source "$1"
set -euo pipefail
work_dir=$2
install_tmp=''
trap cleanup EXIT
exit 23
''', 'cleanup-test', str(ROOT / 'install.sh'), str(build_dir)], text=True, capture_output=True)
        self.assertEqual(result.returncode, 23, result.stderr)
        self.assertFalse(build_dir.exists())

    def test_missing_binary_member_is_rejected(self):
        with tarfile.open(self.archive, 'w:gz') as tar:
            member = tarfile.TarInfo('../outside'); member.size = 1
            tar.addfile(member, io.BytesIO(b'x'))
        self.write_checksum()
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.home / '.local/bin/zelport').exists())
        self.assertFalse((self.home / 'outside').exists())

    def test_unknown_option_does_not_install(self):
        result = self.run_installer(args=['--unknown'])
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.ops(), '')

    def test_missing_checksum_does_not_install(self):
        (self.assets / 'SHA256SUMS').write_text('')
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.home / '.local/bin/zelport').exists())

    def test_repeated_install_preserves_configuration_and_unit(self):
        first = self.run_installer()
        self.assertEqual(first.returncode, 0, first.stderr)
        unit = self.home / '.config/systemd/user/zelport.service'
        original = unit.read_bytes()
        second = self.run_installer(active=True)
        self.assertEqual(second.returncode, 0, second.stderr)
        self.assertEqual(unit.read_bytes(), original)

    def test_repeat_binary_skips_download_replacement_and_restart(self):
        self.assertEqual(self.run_installer().returncode, 0)
        binary = self.home / '.local/bin/zelport'
        unit = self.home / '.config/systemd/user/zelport.service'
        before = (binary.stat().st_mtime_ns, unit.stat().st_mtime_ns)
        self.log.unlink()
        result = self.run_installer(active=True, enabled=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('systemctl --user enable ', self.ops())
        self.assertNotIn('fetch ', self.ops())
        self.assertNotIn('restart ', self.ops())
        self.assertNotIn('daemon-reload', self.ops())
        self.assertEqual(before, (binary.stat().st_mtime_ns, unit.stat().st_mtime_ns))

    def test_current_binary_can_start_missing_service(self):
        self.assertEqual(self.run_installer(args=['--no-start']).returncode, 0)
        self.log.unlink()
        result = self.run_installer()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('fetch ', self.ops())
        self.assertIn('systemctl --user start zelport.service', self.ops())
        self.assertTrue((self.home / '.config/systemd/user/zelport.service').exists())

    def test_current_arch_package_skips_build_and_restart(self):
        result = self.run_installer(distro='arch', package_version='0.1.0preview.5-1', active=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('makepkg ', self.ops())
        self.assertNotIn('restart ', self.ops())

    def test_incomplete_arch_package_is_reinstalled(self):
        result = self.run_installer(distro='arch', package_version='0.1.0preview.5-1',
                                    package_complete=False, args=['--no-start'])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('makepkg ', self.ops())

    def test_old_binary_is_replaced_and_service_restarted(self):
        binary = self.home / '.local/bin/zelport'
        binary.parent.mkdir(parents=True)
        binary.write_text('#!/bin/sh\necho "zelport 0.1.0-preview.4 (built fixture)"\n')
        binary.chmod(0o755)
        result = self.run_installer(active=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(binary.read_bytes(), self.payload)
        self.assertIn('systemctl --user restart zelport.service', self.ops())

    def test_older_arch_package_is_upgraded(self):
        result = self.run_installer(distro='arch', package_version='0.1.0preview.4-1', active=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('makepkg ', self.ops())
        self.assertIn('restart ', self.ops())

    def test_arm64_uses_arm64_asset(self):
        self.archive = self.archive.rename(self.assets / 'zelport_0.1.0-preview.5_linux_arm64.tar.gz')
        self.write_checksum()
        result = self.run_installer(arch='aarch64', args=['--no-start'])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('linux_arm64.tar.gz', self.ops())

    def test_another_installation_blocks_duplicate(self):
        result = self.run_installer(other_install=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('original installer', result.stderr)
        self.assertNotIn('fetch', self.ops())
        self.assertFalse((self.home / '.local/bin/zelport').exists())

    def test_direct_and_pipe_help(self):
        direct = subprocess.run(['bash', str(ROOT / 'install.sh'), '--help'], text=True, capture_output=True)
        piped = subprocess.run(['bash', '-s', '--', '--help'], input=(ROOT / 'install.sh').read_text(), text=True, capture_output=True)
        self.assertEqual(direct.returncode, 0, direct.stderr)
        self.assertEqual(piped.returncode, 0, piped.stderr)
        self.assertIn('Usage:', piped.stdout)
        self.assertEqual(direct.stdout, piped.stdout)


if __name__ == '__main__':
    unittest.main()
