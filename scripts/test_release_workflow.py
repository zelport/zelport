from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / '.github/workflows/release.yml'


class ReleaseVerificationWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.workflow = WORKFLOW.read_text(encoding='utf-8')

    def test_events_cover_pull_requests_main_and_stable_tags(self):
        self.assertRegex(self.workflow, r'(?m)^  pull_request:$')
        self.assertRegex(self.workflow, r'(?m)^      - main$')
        self.assertIn("- 'v*.*.*'", self.workflow)
        self.assertIn(
            '[[ "$GITHUB_REF_NAME" =~ ^v(0|[1-9][0-9]*)\\.'
            '(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$ ]]', self.workflow)

    def test_matrix_uses_exact_native_runner_pairs(self):
        pairs = {
            'macos-15': 'darwin_arm64',
            'macos-15-intel': 'darwin_amd64',
            'ubuntu-24.04-arm': 'linux_arm64',
            'ubuntu-24.04': 'linux_amd64',
        }
        for runner, platform in pairs.items():
            self.assertRegex(
                self.workflow,
                rf'- runner: {re.escape(runner)}\n\s+platform: {platform}')
        self.assertNotIn('-latest', self.workflow)
        self.assertIn('fail-fast: false', self.workflow)

    def test_job_order_builds_once_before_native_execution(self):
        self.assertRegex(self.workflow, r'(?m)^  build:\n    needs: source$')
        self.assertRegex(self.workflow, r'(?m)^  native:\n    needs: build$')
        native = self.workflow.split('\n  native:', 1)[1]
        self.assertNotIn('make release', native)
        self.assertNotIn('go build', native)

    def test_artifact_is_short_lived_and_default_permissions_are_read_only(self):
        self.assertRegex(self.workflow, r'(?m)^permissions:\n  contents: read$')
        self.assertIn('name: zelport-release-${{ github.run_id }}', self.workflow)
        self.assertIn('retention-days: 1', self.workflow)
        self.assertNotIn('contents: write', self.workflow)

    def test_official_actions_are_immutably_pinned(self):
        actions = {
            'actions/checkout': '11d5960a326750d5838078e36cf38b85af677262',
            'actions/setup-go': '924ae3a1cded613372ab5595356fb5720e22ba16',
            'actions/upload-artifact': 'ea165f8d65b6e75b540449e92b4886f43607fa02',
            'actions/download-artifact': '634f93cb2916e3fdff6788551b99b062d0335ce0',
        }
        for action, commit in actions.items():
            matches = re.findall(rf'uses: {re.escape(action)}@([^ ]+)', self.workflow)
            self.assertTrue(matches, action)
            self.assertEqual(set(matches), {commit})

    def test_versions_and_native_acceptance_command_are_explicit(self):
        self.assertIn('version=0.0.0', self.workflow)
        self.assertIn('version="${GITHUB_REF_NAME#v}"', self.workflow)
        self.assertIn('echo "version=$version" >> "$GITHUB_OUTPUT"', self.workflow)
        self.assertIn('make release VERSION="$version"', self.workflow)
        self.assertIn(
            'python3 scripts/check_release_binary.py \\\n'
            '            --bundle release \\\n'
            '            --version "${{ needs.build.outputs.version }}" \\\n'
            '            --platform "${{ matrix.platform }}"',
            self.workflow)


if __name__ == '__main__':
    unittest.main()
