#!/usr/bin/env python3
"""Download and verify an immutable public Zelport App release."""

import argparse
import json
from pathlib import Path
import tempfile
import urllib.request

from verify_release_assets import payload_names, verify_assets


def request(url: str) -> urllib.request.Request:
    return urllib.request.Request(
        url,
        headers={
            "Accept": "application/vnd.github+json",
            "User-Agent": "zelport-release-verifier",
        },
    )


def verify_published(repository: str, version: str) -> None:
    tag = f"v{version}"
    url = f"https://api.github.com/repos/{repository}/releases/tags/{tag}"
    with urllib.request.urlopen(request(url)) as response:
        release = json.load(response)
    if release.get("tag_name") != tag or release.get("draft") or release.get("prerelease"):
        raise ValueError("release must be public, stable, and match the requested tag")
    expected = set(payload_names(version)) | {"SHA256SUMS"}
    assets = {
        asset["name"]: asset["browser_download_url"]
        for asset in release.get("assets", [])
    }
    if set(assets) != expected:
        raise ValueError("published release does not contain the exact expected asset set")
    with tempfile.TemporaryDirectory() as temporary:
        bundle = Path(temporary)
        for name, asset_url in assets.items():
            with urllib.request.urlopen(request(asset_url)) as response:
                (bundle / name).write_bytes(response.read())
        verify_assets(bundle, version, published=True)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    try:
        verify_published(args.repository, args.version)
    except (OSError, UnicodeError, ValueError) as error:
        raise SystemExit(f"published release verification failed: {error}") from error
    print(f"Verified public {args.repository} release v{args.version}")


if __name__ == "__main__":
    main()
