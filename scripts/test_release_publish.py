import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
PUBLISH = ROOT / 'scripts/publish_release.sh'


FAKE_GH = r'''#!{python}
import json
import os
from pathlib import Path
import shutil
import sys

state_path = Path(os.environ['FAKE_GH_STATE'])
state = json.loads(state_path.read_text())
args = sys.argv[1:]

def save():
    state_path.write_text(json.dumps(state))

if args[:2] == ['release', 'view']:
    raise SystemExit(0 if state['exists'] else 1)
if args and args[0] == 'api':
    method = args[args.index('--method') + 1] if '--method' in args else 'GET'
    endpoint = next((value for value in args[1:] if value.startswith('repos/')), '')
    if '/git/ref/tags/' in endpoint:
        print('{{}}')
        raise SystemExit(0)
    if method == 'POST':
        state.update(exists=True, draft=True, release_id='4242')
        save()
        print('4242')
        raise SystemExit(0)
    if method == 'PATCH':
        state['draft'] = False
        save()
        raise SystemExit(19 if os.environ.get('FAKE_AMBIGUOUS_PUBLISH') else 0)
    if method == 'DELETE':
        state.update(exists=False, draft=False)
        save()
        raise SystemExit(0)
    if '/releases/4242' in endpoint:
        print('4242\t' + ('true' if state['draft'] else 'false'))
        raise SystemExit(0)
if args[:2] == ['release', 'upload']:
    raise SystemExit(18 if os.environ.get('FAKE_UPLOAD_FAILURE') else 0)
if args[:2] == ['release', 'download']:
    target = Path(args[args.index('--dir') + 1])
    target.mkdir(exist_ok=True)
    for source in Path(os.environ['FAKE_GH_BUNDLE']).iterdir():
        shutil.copy2(source, target / source.name)
    raise SystemExit(0)
print('unsupported fake gh invocation: ' + repr(args), file=sys.stderr)
raise SystemExit(97)
'''


class ReleasePublishTest(unittest.TestCase):
    version = '0.10.0'

    def prepare(self, root):
        bundle = root / 'bundle'
        bundle.mkdir()
        names = [
            f'zelport_{self.version}_darwin_arm64.tar.gz',
            f'zelport_{self.version}_darwin_amd64.tar.gz',
            f'zelport_{self.version}_linux_arm64.tar.gz',
            f'zelport_{self.version}_linux_amd64.tar.gz',
        ]
        sums = []
        for name in names:
            content = name.encode()
            (bundle / name).write_bytes(content)
            sums.append(f'{hashlib.sha256(content).hexdigest()}  {name}')
        (bundle / 'SHA256SUMS').write_text('\n'.join(sums) + '\n')
        fake_dir = root / 'bin'
        fake_dir.mkdir()
        fake = fake_dir / 'gh'
        fake.write_text(FAKE_GH.format(python=sys.executable))
        fake.chmod(0o755)
        state = root / 'state.json'
        state.write_text(json.dumps({'exists': False, 'draft': False}))
        env = os.environ.copy()
        env.update({
            'PATH': str(fake_dir) + os.pathsep + env['PATH'],
            'GH_REPO': 'zelport/zelport',
            'GITHUB_REF_NAME': f'v{self.version}',
            'VERSION': self.version,
            'FAKE_GH_STATE': str(state),
            'FAKE_GH_BUNDLE': str(bundle),
        })
        return bundle, state, env

    def run_publish(self, root, env):
        return subprocess.run(
            [str(PUBLISH), str(root / 'bundle')], cwd=root, env=env,
            capture_output=True, text=True, timeout=10)

    def test_interrupted_upload_removes_only_its_draft(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _, state, env = self.prepare(root)
            env['FAKE_UPLOAD_FAILURE'] = '1'
            result = self.run_publish(root, env)
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertFalse(json.loads(state.read_text())['exists'])

    def test_ambiguous_publish_response_preserves_public_release(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _, state, env = self.prepare(root)
            env['FAKE_AMBIGUOUS_PUBLISH'] = '1'
            result = self.run_publish(root, env)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            remote = json.loads(state.read_text())
            self.assertTrue(remote['exists'])
            self.assertFalse(remote['draft'])

    def test_succeeds_without_a_git_checkout_when_repo_is_explicit(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _, state, env = self.prepare(root)
            self.assertFalse((root / '.git').exists())
            result = self.run_publish(root, env)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertFalse(json.loads(state.read_text())['draft'])


if __name__ == '__main__':
    unittest.main()
