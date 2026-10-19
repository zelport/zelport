from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / '.github/workflows/release.yml'
MODULE_BUILD_SCRIPT = ROOT / 'scripts/check-module-builds.sh'
PUBLISH_SCRIPT = ROOT / 'scripts/publish_release.sh'


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
        self.assertIn('run: make verify', self.workflow)
        self.assertRegex(self.workflow, r'(?m)^  native:\n    needs: build$')
        native = self.workflow.split('\n  native:', 1)[1]
        self.assertNotIn('make release-assets', native)
        self.assertNotIn('go build', native)

    def test_artifact_is_short_lived_and_default_permissions_are_read_only(self):
        self.assertRegex(self.workflow, r'(?m)^permissions:\n  contents: read$')
        self.assertIn('name: zelport-release-${{ github.run_id }}', self.workflow)
        self.assertIn('retention-days: 1', self.workflow)

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
        self.assertIn('make release-assets VERSION="$version"', self.workflow)
        self.assertIn('make verify-release-assets VERSION="$version"', self.workflow)
        self.assertIn('path: dist/${{ steps.version.outputs.version }}', self.workflow)
        self.assertIn(
            'python3 scripts/check_release_binary.py \\\n'
            '            --bundle release \\\n'
            '            --version "${{ needs.build.outputs.version }}" \\\n'
            '            --platform "${{ matrix.platform }}"',
            self.workflow)

    def test_module_boundary_check_uses_only_standard_search_tools(self):
        script = MODULE_BUILD_SCRIPT.read_text(encoding='utf-8')
        self.assertNotRegex(script, r'(?m)\brg\b')
        self.assertIn('grep -Eq', script)


class ReleasePublicationWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.workflow = WORKFLOW.read_text(encoding='utf-8')

    def test_publish_is_tag_only_and_waits_for_build_and_native_jobs(self):
        self.assertRegex(
            self.workflow,
            r"(?m)^  publish:\n    if: github\.ref_type == 'tag'\n"
            r"    needs: \[build, native\]$")

    def test_write_permission_is_local_to_publish_job(self):
        self.assertEqual(self.workflow.count('contents: write'), 1)
        publish = self.workflow.split('\n  publish:', 1)[1]
        self.assertRegex(publish, r'(?m)^    permissions:\n      contents: write$')

    def test_publish_has_explicit_repository_identity(self):
        publish = self.workflow.split('\n  publish:', 1)[1]
        self.assertIn('GH_REPO: ${{ github.repository }}', publish)
        self.assertIn('scripts/publish_release.sh release', publish)

    def test_existing_release_is_refused_before_draft_creation(self):
        publish = PUBLISH_SCRIPT.read_text(encoding='utf-8')
        preflight = publish.index('gh release view "$GITHUB_REF_NAME"')
        create = publish.index('--method POST')
        self.assertLess(preflight, create)
        self.assertIn('refusing to replace existing release', publish)
        self.assertIn('-F draft=true', publish)
        self.assertIn('-F generate_release_notes=true', publish)
        self.assertIn('/git/ref/tags/$GITHUB_REF_NAME', publish)

    def test_draft_is_downloaded_and_verified_before_becoming_public(self):
        publish = PUBLISH_SCRIPT.read_text(encoding='utf-8')
        download = publish.index('gh release download "$GITHUB_REF_NAME"')
        checksum = publish.index('sha256sum --check SHA256SUMS')
        compare = publish.index('cmp "$asset" "published/${asset##*/}"')
        make_public = publish.index('--method PATCH')
        self.assertLess(download, checksum)
        self.assertLess(checksum, compare)
        self.assertLess(compare, make_public)
        self.assertIn('"$bundle/zelport_${VERSION}_source.tar.gz"', publish)
        self.assertIn('"$bundle/PKGBUILD"', publish)
        self.assertIn('"$bundle/zelport-${VERSION}.SRCINFO"', publish)
        self.assertIn('= 8', publish)

    def test_failure_cleanup_deletes_only_the_draft_created_by_this_job(self):
        publish = PUBLISH_SCRIPT.read_text(encoding='utf-8')
        self.assertIn('release_id=', publish)
        self.assertIn('cleanup_allowed=true', publish)
        self.assertIn('if [[ "$remote" == "$release_id"', publish)
        self.assertIn('--method DELETE', publish)
        self.assertIn('trap cleanup EXIT', publish)
        self.assertIn('cleanup_allowed=false', publish)


if __name__ == '__main__':
    unittest.main()
