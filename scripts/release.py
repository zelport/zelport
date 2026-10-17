#!/usr/bin/env python3
"""Build full distributions and generate a Homebrew tap without publishing it."""
import argparse
from datetime import datetime, timezone
import hashlib
import os
from pathlib import Path
import re
import subprocess
import tempfile
import tarfile

ROOT = Path(__file__).resolve().parents[1]
PLATFORMS = ('darwin_arm64', 'darwin_amd64', 'linux_arm64', 'linux_amd64')
STABLE_SEMVER = re.compile(r'(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)')


def validate_version(version):
    if not STABLE_SEMVER.fullmatch(version):
        raise ValueError('Use a stable numeric release version such as 0.10.0, without v')


def build_command(version, built_at, output):
    validate_version(version)
    return [
        os.environ.get('GO', 'go'), 'build', '-trimpath',
        '-ldflags', f"-s -w -X main.version={version} -X 'main.builtAt={built_at}'",
        '-o', str(output), './cmd/zelport',
    ]


def package(version, binaries, output):
    validate_version(version)
    if output.exists():
        raise FileExistsError(f'Refusing to overwrite {output}')
    for platform in PLATFORMS:
        if not (binaries / platform).is_file():
            raise FileNotFoundError(binaries / platform)
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=output.parent) as tmp:
        stage = Path(tmp) / 'release'
        stage.mkdir()
        hashes = {}
        for platform in PLATFORMS:
            name = f'zelport_{version}_{platform}.tar.gz'
            with tarfile.open(stage / name, 'w:gz') as archive:
                for source, target, mode in [(binaries / platform, 'zelport', 0o755),
                                             (ROOT / 'LICENSE', 'LICENSE', 0o644),
                                             (ROOT / 'THIRD_PARTY_NOTICES.txt', 'THIRD_PARTY_NOTICES.txt', 0o644)]:
                    info = archive.gettarinfo(str(source), arcname=target)
                    info.uid = info.gid = 0
                    info.uname = info.gname = ''
                    info.mode = mode
                    with source.open('rb') as stream:
                        archive.addfile(info, stream)
            hashes[platform] = hashlib.sha256((stage / name).read_bytes()).hexdigest()
        downloads = []
        for goos, brew_os in [('darwin', 'macos'), ('linux', 'linux')]:
            downloads.append(f'  on_{brew_os} do')
            for goarch, brew_arch in [('arm64', 'arm'), ('amd64', 'intel')]:
                platform = f'{goos}_{goarch}'
                name = f'zelport_{version}_{platform}.tar.gz'
                downloads.extend([
                    f'    on_{brew_arch} do',
                    f'      url "https://github.com/zelport/zelport/releases/download/v{version}/{name}"',
                    f'      sha256 "{hashes[platform]}"',
                    '    end',
                ])
            downloads.append('  end')
        template = (ROOT / 'packaging/homebrew/zelport.rb.in').read_text()
        formula = template.replace('@VERSION@', version).replace('@DOWNLOADS@', '\n'.join(downloads))
        (stage / 'Formula').mkdir()
        (stage / 'Formula/zelport.rb').write_text(formula)
        (stage / 'SHA256SUMS').write_text(''.join(
            f'{hashes[p]}  zelport_{version}_{p}.tar.gz\n' for p in PLATFORMS))
        stage.rename(output)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    validate_version(args.version)
    if args.output.exists():
        parser.error(f'Refusing to overwrite {args.output}')
    built_at = datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S UTC')
    with tempfile.TemporaryDirectory() as tmp:
        binaries = Path(tmp)
        for platform in PLATFORMS:
            goos, goarch = platform.split('_')
            env = dict(os.environ, GOOS=goos, GOARCH=goarch, CGO_ENABLED='0', GOWORK='off')
            subprocess.run(build_command(args.version, built_at, binaries / platform),
                           cwd=ROOT, env=env, check=True)
        package(args.version, binaries, args.output.resolve())
    print(f'Release artifacts and Homebrew formula written to {args.output}')


if __name__ == '__main__':
    main()
