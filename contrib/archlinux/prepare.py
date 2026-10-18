#!/usr/bin/env python3
"""Generate a source-based Arch recipe from a reviewed GitHub source archive."""
import argparse
import hashlib
from pathlib import Path
import re
import tarfile

ROOT = Path(__file__).resolve().parents[2]


def prepare(version, source_archive, output):
    if not re.fullmatch(r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)', version):
        raise ValueError('Use a stable numeric version such as 0.10.0, without v')
    prefix = f'zelport-{version}/'
    with tarfile.open(source_archive, 'r:gz') as archive:
        for name in ['go.mod', 'Makefile', 'LICENSE', 'THIRD_PARTY_NOTICES.txt', 'contrib/systemd/zelport.service']:
            try:
                member = archive.getmember(prefix + name)
            except KeyError as error:
                raise ValueError(f'Missing source file: {prefix}{name}') from error
            if not member.isfile():
                raise ValueError(f'Expected a regular source file: {name}')
            if name == 'go.mod':
                module = archive.extractfile(member).read().decode()
                if 'v0.0.0-local.' in module or re.search(r'(?m)^replace\s', module):
                    raise ValueError('Publish dependencies first; local pins or replacements are not allowed')
    digest = hashlib.sha256(Path(source_archive).read_bytes()).hexdigest()
    recipe = (ROOT / 'contrib/archlinux/PKGBUILD.in').read_text()
    recipe = recipe.replace('@VERSION@', version).replace('@PKGVER@', version).replace('@SHA256@', digest)
    srcinfo = (ROOT / 'contrib/archlinux/SRCINFO.in').read_text()
    srcinfo = srcinfo.replace('@VERSION@', version).replace('@SHA256@', digest)
    output = Path(output)
    output.mkdir(parents=True, exist_ok=False)
    (output / 'PKGBUILD').write_text(recipe)
    (output / '.SRCINFO').write_text(srcinfo)
    return output


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--source-archive', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    try:
        print(prepare(args.version, args.source_archive, args.output))
    except (ValueError, OSError, tarfile.TarError) as error:
        parser.exit(1, str(error) + '\n')


if __name__ == '__main__':
    main()
