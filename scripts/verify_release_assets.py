#!/usr/bin/env python3
"""Verify the exact local or published Zelport App release asset set."""

import argparse
import hashlib
from pathlib import Path
import re


STABLE_SEMVER = re.compile(
    r"(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)"
)
SHA256 = re.compile(r"[0-9a-f]{64}")
PLATFORMS = ("darwin_arm64", "darwin_amd64", "linux_arm64", "linux_amd64")


def payload_names(version: str) -> tuple[str, ...]:
    if not STABLE_SEMVER.fullmatch(version):
        raise ValueError("version must be stable numeric SemVer without a v prefix")
    return (
        *(f"zelport_{version}_{platform}.tar.gz" for platform in PLATFORMS),
        f"zelport_{version}_source.tar.gz",
        "PKGBUILD",
        f"zelport-{version}.SRCINFO",
    )


def regular_file(path: Path) -> None:
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"release asset must be a regular file: {path}")


def verify_assets(bundle: Path, version: str, published: bool = False) -> None:
    payloads = payload_names(version)
    expected = set(payloads) | {"SHA256SUMS"}
    if not published:
        expected.add("Formula/zelport.rb")
    if bundle.is_symlink() or not bundle.is_dir():
        raise ValueError("release bundle must be a real directory")
    paths = list(bundle.rglob("*"))
    if any(path.is_symlink() for path in paths):
        raise ValueError("release bundle must not contain symbolic links")
    actual_files = {
        str(path.relative_to(bundle))
        for path in paths
        if not path.is_dir()
    }
    actual_directories = {
        str(path.relative_to(bundle)) for path in paths if path.is_dir()
    }
    expected_directories = set() if published else {"Formula"}
    if actual_files != expected or actual_directories != expected_directories:
        raise ValueError(
            f"release asset set differs: expected {sorted(expected)}, "
            f"found {sorted(actual_files)}"
        )
    for name in expected:
        regular_file(bundle / name)

    entries = {}
    manifest = (bundle / "SHA256SUMS").read_text(encoding="ascii")
    for line in manifest.splitlines():
        parts = line.split("  ", 1)
        if len(parts) != 2 or not SHA256.fullmatch(parts[0]) or parts[1] in entries:
            raise ValueError("SHA256SUMS has an invalid or duplicate entry")
        entries[parts[1]] = parts[0]
    if set(entries) != set(payloads):
        raise ValueError("SHA256SUMS must contain exactly one entry for every payload asset")
    for name, expected_digest in entries.items():
        digest = hashlib.sha256((bundle / name).read_bytes()).hexdigest()
        if digest != expected_digest:
            raise ValueError(f"checksum mismatch for {name}")

    recipe = (bundle / "PKGBUILD").read_text(encoding="utf-8")
    srcinfo = (bundle / f"zelport-{version}.SRCINFO").read_text(encoding="utf-8")
    if re.findall(r"(?m)^pkgver=(.+)$", recipe) != [version]:
        raise ValueError("PKGBUILD version does not match the release")
    if re.findall(r"(?m)^\s*pkgver = (.+)$", srcinfo) != [version]:
        raise ValueError(".SRCINFO version does not match the release")
    if not published:
        formula = (bundle / "Formula/zelport.rb").read_text(encoding="utf-8")
        versions = re.findall(r'^\s*version "([^"]+)"$', formula, re.MULTILINE)
        if versions != [version]:
            raise ValueError("Homebrew formula version does not match the release")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bundle", required=True, type=Path)
    parser.add_argument("--version", required=True)
    parser.add_argument("--published", action="store_true")
    args = parser.parse_args()
    try:
        verify_assets(args.bundle, args.version, args.published)
    except (OSError, UnicodeError, ValueError) as error:
        raise SystemExit(f"release verification failed: {error}") from error
    print(f"Verified Zelport App {args.version} release assets in {args.bundle}")


if __name__ == "__main__":
    main()
