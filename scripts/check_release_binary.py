#!/usr/bin/env python3
"""Verify and execute an already-packaged Zelport App release binary."""

import hashlib
import hmac
import importlib.util
from pathlib import Path
import tarfile


_release_spec = importlib.util.spec_from_file_location(
    'release', Path(__file__).with_name('release.py'))
release = importlib.util.module_from_spec(_release_spec)
_release_spec.loader.exec_module(release)

EXPECTED_MEMBERS = {
    'zelport': 0o755,
    'LICENSE': 0o644,
    'THIRD_PARTY_NOTICES.txt': 0o644,
}


def _expected_checksum(checksums: Path, archive_name: str) -> str:
    matches = []
    for line in checksums.read_text(encoding='utf-8').splitlines():
        fields = line.split()
        if len(fields) == 2 and fields[1] == archive_name:
            matches.append(fields[0])
    if len(matches) != 1:
        raise ValueError(f'SHA256SUMS must contain exactly one entry for {archive_name}')
    return matches[0]


def verify_archive(bundle: Path, version: str, platform: str,
                   destination: Path) -> Path:
    """Verify one release archive and safely extract its three regular files."""
    release.validate_version(version)
    if platform not in release.PLATFORMS:
        raise ValueError(f'unknown release platform: {platform}')
    bundle = Path(bundle)
    archive = bundle / f'zelport_{version}_{platform}.tar.gz'
    checksums = bundle / 'SHA256SUMS'
    if not checksums.is_file():
        raise FileNotFoundError(f'SHA256SUMS not found in {bundle}')
    expected_digest = _expected_checksum(checksums, archive.name)
    actual_digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    if not hmac.compare_digest(actual_digest, expected_digest):
        raise ValueError(f'checksum mismatch for {archive.name}')

    with tarfile.open(archive, 'r:gz') as source:
        members = source.getmembers()
        names = [member.name for member in members]
        if len(members) != len(EXPECTED_MEMBERS) or set(names) != set(EXPECTED_MEMBERS):
            raise ValueError(f'unexpected archive members: {names}')
        for member in members:
            if not member.isreg():
                raise ValueError(f'archive member is not a regular file: {member.name}')
            expected_mode = EXPECTED_MEMBERS[member.name]
            if member.mode & 0o777 != expected_mode:
                raise ValueError(
                    f'archive member mode for {member.name} is '
                    f'{member.mode & 0o777:o}, expected {expected_mode:o}')

        destination = Path(destination)
        destination.mkdir(parents=True, exist_ok=False)
        for member in members:
            content = source.extractfile(member)
            if content is None:
                raise ValueError(f'cannot read archive member: {member.name}')
            target = destination / member.name
            target.write_bytes(content.read())
            target.chmod(EXPECTED_MEMBERS[member.name])
    return destination / 'zelport'
