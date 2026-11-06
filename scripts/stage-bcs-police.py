#!/usr/bin/env python3
"""Stage the unmodified MiPa BMW 1.1 fallback from its extracted original archive.

Usage: python scripts/stage-bcs-police.py EXTRACTED_ARCHIVE EMPTY_STAGING_DIRECTORY
This assembles files for a future release; it never edits a user's AI lists.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil

MODEL = Path('Vehicles/MIPA/MIPA_AI_BMW_F31')
SOURCE = 'https://reboot.omsi-webdisk.de/file/2476-ki-fahrzeug-bmw-3er-touring-zivil-und-polizeivariante/'
ARCHIVE_SHA256 = '818298bc6085d0a37d5bc0ca1c25ea2ee1d3c2df70b11e01ceec070620883498'


def stage(source: Path, destination: Path) -> int:
    source = source.resolve(strict=True)
    destination = destination.resolve()
    if destination.exists() and (not destination.is_dir() or any(destination.iterdir())):
        raise ValueError('Use a new or empty staging directory; installed models are never overwritten.')
    if source == destination or source in destination.parents or destination in source.parents:
        raise ValueError('Source and staging directories must be separate.')
    model = source / 'OMSI' / MODEL
    for name in ['MIPA_AI_BMW_F31.ovh', 'MIPA_AI_BMW_F31_pol.ovh', 'MIPA_AI_BMW_F31_pol_active.ovh']:
        if not (model / name).is_file():
            raise ValueError(f'Original archive definition missing: {name}')
    files = [(p, MODEL / p.relative_to(model)) for p in sorted(model.rglob('*')) if p.is_file()]
    attribution = Path('ThirdParty/BCS/MiPa_BMW_F31')
    for name in ['ReadMe_DE.txt', 'ReadMe_EN.txt']:
        p = source / name
        if not p.is_file():
            raise ValueError(f'Original author README missing: {name}')
        files.append((p, attribution / name))
    if any(p.is_symlink() or not p.resolve().is_relative_to(source) for p, _ in files):
        raise ValueError('Archive contents must be regular files within the extraction directory.')
    manifest = {
        'author': 'MiPa', 'version': '1.1', 'source': SOURCE,
        'license': 'WTFPL-2.0, as stated on the original publisher entry',
        'inspected_original_archive_sha256': ARCHIVE_SHA256,
        'archive_hash_verified_by_this_script': False,
        'files': {},
        'dependency_not_bundled': 'Kennz_DtNeu font for registration plates',
        'fire_engine': 'Not bundled; install the original RomanianWorks package separately.',
    }
    # Validate every source before writing anything to the staging directory.
    for original, relative in files:
        manifest['files'][relative.as_posix()] = hashlib.sha256(original.read_bytes()).hexdigest()
    for original, relative in files:
        target = destination / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(original, target)
        if hashlib.sha256(target.read_bytes()).hexdigest() != manifest['files'][relative.as_posix()]:
            raise OSError(f'Staged content differs: {relative}')
    (destination / attribution / 'SOURCE.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
    return len(files)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source', type=Path)
    parser.add_argument('destination', type=Path)
    args = parser.parse_args()
    try:
        count = stage(args.source, args.destination)
    except (OSError, ValueError) as error:
        parser.exit(1, f'Could not stage police fallback: {error}\n')
    print(f'Staged {count} unmodified BMW files with provenance. No AI lists changed.')


if __name__ == '__main__':
    main()
