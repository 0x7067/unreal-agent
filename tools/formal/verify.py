#!/usr/bin/env python3
"""Combined source freshness + Bend gate on one stable project tree.

Unlike prove.py (compiler/structure checks), this is the canonical source-bound
verification entry point used by `make prove`. It is not a filesystem sandbox,
verified compiler, or whole-Go proof.
"""
from __future__ import annotations

import argparse
import hashlib
from pathlib import Path
import subprocess
import sys

import prove

ROOT = Path(__file__).resolve().parents[2]


def snapshot(root: Path) -> dict[str, str]:
    tracked = subprocess.run(
        ['git', '-C', str(root), 'ls-files', '--cached', '--others', '--exclude-standard', '-z'],
        capture_output=True, text=True, check=True, timeout=30)
    names = {name for name in tracked.stdout.split('\0')
             if name and (name.endswith('.go') or Path(name).name in ('go.mod', 'go.sum', 'go.work', 'go.work.sum'))}
    for directory in ('tools/formal', 'docs/formal'):
        names.update(str(path.relative_to(root)) for path in (root/directory).rglob('*')
                     if path.suffix in ('.py', '.go', '.bend', '.json', '.sh')
                     and '__pycache__' not in path.parts and '.lake' not in path.parts)
    names.update(name for name in ('LAWS.bend', 'PROOF.bend', 'Makefile') if (root/name).exists())
    result = {}
    for name in sorted(names):
        path = root/name
        if not path.is_file() or path.is_symlink() or path.resolve() != path.absolute():
            raise prove.ProofError(f'verification input missing or symlinked: {name}')
        result[name] = hashlib.sha256(path.read_bytes()).hexdigest()
    return result


def source_check(root: Path, timeout: float) -> None:
    for script in ('tools/formal/gosource/check.py', 'tools/formal/apienums/check.py'):
        result = subprocess.run([sys.executable, str(root/script), '--root', str(root)],
                                cwd=root, stdin=subprocess.DEVNULL, text=True,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                timeout=timeout, check=False)
        if result.returncode:
            raise prove.ProofError(f'source-linked contract freshness failed:\n{result.stdout}{result.stderr}')
        print(result.stdout.strip(), flush=True)


def verify(root: Path, bend: str, timeout: float) -> None:
    before = snapshot(root)
    source_check(root, timeout)
    project = prove.check_structure(root)
    prove.prove(project, bend, timeout)
    source_check(root, timeout)
    after = snapshot(root)
    if before != after:
        changed = sorted(name for name in before.keys() | after.keys() if before.get(name) != after.get(name))
        raise prove.ProofError('verification inputs changed during checking; rerun on a stable tree: ' + ', '.join(changed))
    print('verify: source-bound Bend proofs checked on a stable tree; restricted frontend remains trusted; NOT a whole-Go proof')


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bend', help='Bend executable; otherwise BEND/PATH/~/.bend/bin/bend')
    parser.add_argument('--timeout', type=prove.positive_timeout, default=120.0)
    args = parser.parse_args(argv)
    try:
        verify(ROOT, prove.find_bend(args.bend), args.timeout)
    except (prove.ProofError, ValueError, OSError, subprocess.SubprocessError) as error:
        print(f'verify: FAIL: {error}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
