#!/usr/bin/env python3
"""Re-extract actual Go AST every invocation; refresh SOURCE artifacts only."""
import argparse
import difflib
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
DEST = Path('docs/formal/implementation/enums')

MODULE = 'github.com/unreallabsai/unreal-agent'


def _regular_local_path(root, relative):
    """Reject symlink/traversal paths, including the selected root ancestry."""
    root = Path(root).absolute()
    relative = Path(relative)
    if relative.is_absolute() or '..' in relative.parts:
        raise RuntimeError(f'nonlocal source path: {relative}')
    for current in (*reversed(root.parents), root):
        if current.is_symlink():
            raise RuntimeError(f'symlink source root ancestry: {current}')
    current = root
    for part in relative.parts:
        current = current / part
        if current.is_symlink():
            raise RuntimeError(f'symlink source path: {current}')
    return current


def canonical_go_environment():
    # Call only AFTER checking the actual invocation environment; normalization
    # is not a substitute for rejecting unsupported runtime resolution.
    return {**os.environ, 'GOTOOLCHAIN': 'local', 'GOWORK': 'off', 'GOFLAGS': ''}


def check_environment(root):
    """Validate the supported root-module Go resolution context; return hashes.

    This is a prerequisite for source-linked verification, not a digest-only
    approximation of module resolution. Active workspaces and ANY effective
    GOFLAGS are outside scope (including flags persisted through go env -w).
    """
    root = Path(root).absolute()
    mod = _regular_local_path(root, 'go.mod')
    if not mod.is_file():
        raise RuntimeError('root go.mod is required for source-linked checking')
    run = subprocess.run(['go', 'env', '-json', 'GOFLAGS', 'GOWORK', 'GO111MODULE', 'GOMOD'], cwd=root,
                         capture_output=True, text=True, timeout=30,
                         env={**os.environ, 'GOTOOLCHAIN': 'local'})
    if run.returncode:
        raise RuntimeError('cannot inspect actual Go environment: ' + run.stderr)
    effective = json.loads(run.stdout)
    if effective.get('GO111MODULE', '') not in ('', 'on'):
        raise RuntimeError('module-disabled Go context unsupported')
    if effective.get('GOFLAGS', ''):
        raise RuntimeError('nonempty effective GOFLAGS unsupported (including overlays)')
    if effective.get('GOWORK', '') not in ('', 'off'):
        raise RuntimeError('active Go workspace unsupported: ' + effective['GOWORK'])
    if Path(effective.get('GOMOD', '')).resolve() != mod.resolve():
        raise RuntimeError('actual Go module is not the selected root go.mod')
    # Go parses its own module syntax; no handwritten go.mod line parser.
    run = subprocess.run(['go', 'mod', 'edit', '-json'], cwd=root,
                         capture_output=True, text=True, timeout=30,
                         env=canonical_go_environment())
    if run.returncode:
        raise RuntimeError('cannot parse root go.mod: ' + run.stderr)
    metadata = json.loads(run.stdout)
    if (metadata.get('Module') or {}).get('Path') != MODULE:
        raise RuntimeError('unsupported root module path (expected ' + MODULE + ')')
    for requirement in metadata.get('Require') or []:
        path = requirement['Path']
        if path == MODULE or path.startswith(MODULE + '/'):
            raise RuntimeError('root-module subpath require unsupported: ' + path)
    for replacement in metadata.get('Replace') or []:
        path = replacement['Old']['Path']
        if path == MODULE or path.startswith(MODULE + '/'):
            raise RuntimeError('root-module replacement unsupported: ' + path)
    return {'go.mod': hashlib.sha256(mod.read_bytes()).hexdigest()}


def check_source_context(root, source_paths):
    """Reject nested modules along ACTUAL selected source/dependency ancestry.

    Unrelated nested modules (TUI/API generator) are deliberately unaffected.
    Paths must be root-relative actual source filenames, not package names.
    """
    root = Path(root).absolute()
    for relative in source_paths:
        source = _regular_local_path(root, relative)
        if not source.is_file():
            raise RuntimeError(f'missing actual source file: {relative}')
        current = source.parent
        while current != root:
            nested = current / 'go.mod'
            if nested.exists() or nested.is_symlink():
                raise RuntimeError(f'nested Go module on selected source path: {nested}')
            current = current.parent


def byte_list(values):
    return '[' + ', '.join(f'{n}n' for n in values) + ']'

def expression(node):
    kind = node['kind']
    if kind == 'bool':
        return 'M.Truth{' + ('True{}' if node.get('value') else 'False{}') + '}'
    if kind == 'eq':
        slot = 'Second' if node.get('index', 0) else 'First'
        return f'M.Same{{M.{slot}{{}}, {byte_list(node.get("bytes", []))}}}'
    names = {'not': 'Neg', 'and': 'Both', 'or': 'Either', 'if': 'Branch'}
    return 'M.' + names[kind] + '{' + ', '.join(map(expression, node['args'])) + '}'

def artifacts(root):
    module_metadata = check_environment(root)
    run = subprocess.run(['go', 'run', 'tools/formal/gosource/extract.go', str(root)],
                         cwd=ROOT, capture_output=True, text=True, timeout=90,
                         env=canonical_go_environment())
    if run.returncode:
        raise RuntimeError(run.stderr)
    records = json.loads(run.stdout)
    check_source_context(root, {path for record in records
                                for path in [record['file'], *record['dependencies']]})
    for record in records:
        record['module_metadata_sha256'] = module_metadata
        record['source_sha256'] = hashlib.sha256((root / record['file']).read_bytes()).hexdigest()
        record['dependency_sha256'] = {
            path: hashlib.sha256((root / path).read_bytes()).hexdigest()
            for path in record['dependencies']
        }
        record['law'] = record['id'] + '_matches_policy'
        record['classification'] = 'source-linked-restricted-semantics'
    text = ('# Generated from actual restricted Go AST. DO NOT EDIT.\n'
            '# Explicit refresh changes SOURCE ONLY, never the independent policy.\n'
            'import Base\nimport ./semantics.bend as M\n\n')
    for record in records:
        name = record['id']
        text += f'# {record["file"]}: {record["name"]}; inputs {record["inputs"]}\n'
        text += f'def {name}_ast() -> M.Expr:\n  {expression(record["ast"])}\n\n'
        args = '+x: List<&2, Nat>'
        if len(record['inputs']) == 2:
            args += ', +y: List<&2, Nat>'
        y = 'y' if len(record['inputs']) == 2 else '[]'
        text += f'def {name}({args}) -> Bool:\n  M.eval({name}_ast(), x, {y})\n\n'
    return {'SOURCE.bend': text, 'source_ast.json': json.dumps(records, indent=2) + '\n'}

def check(root, refresh=False):
    # Never follow symlinked generated artifacts (including parent directories).
    for name in ('SOURCE.bend', 'source_ast.json'):
        current = root
        if current.is_symlink():
            raise RuntimeError('symlink source root')
        for part in (DEST / name).parts:
            current = current / part
            if current.is_symlink():
                raise RuntimeError(f'symlink artifact path: {current}')
    outputs = artifacts(root)
    drift = []
    for name, expected in outputs.items():
        path = root / DEST / name
        actual = path.read_text() if path.exists() else ''
        if actual == expected:
            continue
        if refresh:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(expected)
        else:
            drift.append(name)
            print(''.join(difflib.unified_diff(actual.splitlines(True), expected.splitlines(True),
                  fromfile=str(path), tofile='fresh actual Go AST')), file=sys.stderr)
    if drift:
        raise RuntimeError('source extraction drift: ' + ', '.join(drift) +
                           '; review Go changes and independent laws before explicit --refresh')
    return len(outputs)

def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--root', type=Path, default=ROOT, help='source/artifact tree (mutation tests)')
    ap.add_argument('--refresh', action='store_true', help='update SOURCE only, never policy/laws/proofs')
    args = ap.parse_args()
    try:
        count = check(args.root.absolute(), args.refresh)
    except (RuntimeError, OSError, ValueError, subprocess.TimeoutExpired) as exc:
        print(f'gosource: {exc}', file=sys.stderr)
        return 1
    print(f'gosource: actual Go AST matches {count} source artifacts (11 predicates)')
    return 0

if __name__ == '__main__':
    sys.exit(main())
