#!/usr/bin/env bash
set -euo pipefail

bundle=${1:?usage: publish_release.sh BUNDLE}
: "${GH_REPO:?GH_REPO is required}"
: "${GITHUB_REF_NAME:?GITHUB_REF_NAME is required}"
: "${VERSION:?VERSION is required}"

test "$GITHUB_REF_NAME" = "v$VERSION"
assets=(
  "$bundle/zelport_${VERSION}_darwin_arm64.tar.gz"
  "$bundle/zelport_${VERSION}_darwin_amd64.tar.gz"
  "$bundle/zelport_${VERSION}_linux_arm64.tar.gz"
  "$bundle/zelport_${VERSION}_linux_amd64.tar.gz"
  "$bundle/zelport_${VERSION}_source.tar.gz"
  "$bundle/PKGBUILD"
  "$bundle/zelport-${VERSION}.SRCINFO"
  "$bundle/SHA256SUMS"
)
for asset in "${assets[@]}"; do
  test -f "$asset"
done

release_id=
cleanup_allowed=true
cleanup() {
  status=$?
  trap - EXIT
  if [[ -n "$release_id" && "$cleanup_allowed" == true ]]; then
    remote=$(gh api "repos/$GH_REPO/releases/$release_id" \
      --jq '[.id, .draft] | @tsv' 2>/dev/null || true)
    if [[ "$remote" == "$release_id"$'\ttrue' ]]; then
      gh api --method DELETE "repos/$GH_REPO/releases/$release_id" || true
    fi
  fi
  exit "$status"
}
trap cleanup EXIT

if gh release view "$GITHUB_REF_NAME" >/dev/null 2>&1; then
  echo "refusing to replace existing release $GITHUB_REF_NAME" >&2
  exit 1
fi
gh api "repos/$GH_REPO/git/ref/tags/$GITHUB_REF_NAME" >/dev/null

release_id=$(gh api --method POST "repos/$GH_REPO/releases" \
  -f tag_name="$GITHUB_REF_NAME" \
  -f name="Zelport App $GITHUB_REF_NAME" \
  -F draft=true \
  -F generate_release_notes=true \
  --jq .id)
test -n "$release_id"
gh release upload "$GITHUB_REF_NAME" "${assets[@]}"

mkdir published
gh release download "$GITHUB_REF_NAME" --dir published
test "$(find published -type f | wc -l | tr -d ' ')" = 8
(cd published && sha256sum --check SHA256SUMS)
for asset in "${assets[@]}"; do
  cmp "$asset" "published/${asset##*/}"
done

# Once publication is attempted, cleanup may not delete the release until its
# remote state is known: the PATCH can succeed remotely while its response fails.
cleanup_allowed=false
publish_status=0
gh api --method PATCH "repos/$GH_REPO/releases/$release_id" \
  -F draft=false --silent || publish_status=$?
remote=$(gh api "repos/$GH_REPO/releases/$release_id" \
  --jq '[.id, .draft] | @tsv')
if [[ "$remote" == "$release_id"$'\tfalse' ]]; then
  trap - EXIT
  exit 0
fi
cleanup_allowed=true
echo "release $GITHUB_REF_NAME did not become public" >&2
if [[ "$publish_status" -eq 0 ]]; then
  publish_status=1
fi
exit "$publish_status"
