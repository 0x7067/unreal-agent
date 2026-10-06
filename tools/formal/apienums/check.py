#!/usr/bin/env python3
"""Check 476 actual Go ASTs against frozen declaration policy; refresh SOURCE only."""
import argparse
import difflib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
DEST = Path('tools/formal/models/implementation/api-enums')
COUNTS = {'internal/openaiapi/types.gen.go': 279,
          'internal/anthropicapi/types.gen.go': 197}


def safe(root, rel):
    path = root / rel
    if path.resolve() != path.absolute():
        raise RuntimeError(f'symlink/escape not admitted: {path}')
    return path


def check_environment(root):
    # Share the same canonical module/workspace/overlay policy as gosource.
    # Import trusted checker code from this checkout, never from --root fixtures.
    path = ROOT / 'tools/formal/gosource/check.py'
    spec = importlib.util.spec_from_file_location('apienums_gosource_environment', path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    metadata = module.check_environment(root)
    module.check_source_context(root, list(COUNTS))
    for relative in ('internal/go.mod', 'internal/openaiapi/go.mod',
                     'internal/anthropicapi/go.mod'):
        nested = root / relative
        if nested.exists() or nested.is_symlink():
            raise RuntimeError(f'nested module not admitted for API enum sources: {relative}')
    return metadata


def byte_list(values):
    return '[' + ', '.join(f'{n}n' for n in values) + ']'


def expression(node):
    kind = node['kind']
    if kind == 'bool':
        return 'M.Truth{' + ('True{}' if node.get('value') else 'False{}') + '}'
    if kind == 'eq':
        return f'M.Same{{M.First{{}}, {byte_list(node.get("bytes", []))}}}'
    names = {'or': 'Either', 'if': 'Branch'}
    return 'M.' + names[kind] + '{' + ', '.join(map(expression, node['args'])) + '}'


def source_text(records):
    text = ('# Actual restricted Go AST. Generated; explicit --refresh updates SOURCE only.\n'
            'import Base\nimport ../enums/semantics.bend as M\n\n')
    for r in records:
        text += f'# {r["file"]}: ({r["receiver"]}).Valid\n'
        text += f'def {r["id"]}_ast() -> M.Expr:\n  {expression(r["ast"])}\n\n'
        text += (f'def {r["id"]}(+x: List<&2, Nat>) -> Bool:\n'
                 f'  M.eval({r["id"]}_ast(), x, [])\n\n')
    return text


def policy_text(policies):
    text = ('# FROZEN declaration-derived policy. Independent of Valid method bodies.\n'
            '# Policy edits require explicit human review; --refresh NEVER changes this file.\n'
            'import Base\nimport ./helpers.bend as H\n\n')
    for p in policies:
        values = '[' + ', '.join(byte_list(c['bytes']) for c in p['constants']) + ']'
        text += (f'def {p["id"]}_constants() -> List<&2, List<&2, Nat>>:\n  {values}\n\n'
                 f'def {p["id"]}(+x: List<&2, Nat>) -> Bool:\n'
                 f'  H.members({p["id"]}_constants(), x)\n\n')
    return text


def extract(root):
    env = dict(os.environ, GOTOOLCHAIN='local', GOWORK='off', GOFLAGS='',
               GOPROXY='off', GOSUMDB='off')
    run = subprocess.run(['go', 'run', 'tools/formal/apienums/extract.go', str(root)],
                         cwd=ROOT, env=env, capture_output=True, text=True, timeout=120)
    if run.returncode:
        raise RuntimeError(run.stderr.strip() or 'Go AST extraction failed')
    return json.loads(run.stdout)


def check(root, refresh=False):
    root = root.absolute()
    if root.resolve() != root:
        raise RuntimeError('symlink root not admitted')
    module_metadata = check_environment(root)
    data = extract(root)
    records = data['records']
    for record in records:
        record['module_metadata_sha256'] = module_metadata
    counts = {file: sum(r['file'] == file for r in records) for file in COUNTS}
    if counts != COUNTS or len(records) != sum(COUNTS.values()):
        raise RuntimeError(f'explicit inventory mismatch: expected {COUNTS}, found {counts}, total {len(records)}')
    if len({r['id'] for r in records}) != len(records):
        raise RuntimeError('duplicate method IDs')
    frozen = json.loads(safe(root, DEST / 'policy.json').read_text())
    if data['policy'] != frozen:
        raise RuntimeError('DECLARATION POLICY MISMATCH: enum types/constants changed; '
                           '--refresh cannot change independent policy; explicit human policy review required')
    if safe(root, DEST / 'POLICY.bend').read_text() != policy_text(frozen):
        raise RuntimeError('POLICY.bend does not match frozen independent policy.json')
    outputs = {'SOURCE.bend': source_text(records),
               'source_ast.json': json.dumps(records, indent=2) + '\n'}
    drift = []
    for name, expected in outputs.items():
        path = safe(root, DEST / name)
        actual = path.read_text() if path.exists() else ''
        if actual == expected:
            continue
        if refresh:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(expected)
        else:
            drift.append(name)
            # Avoid megabytes of diagnostic output from whole-file source digests.
            delta = ''.join(difflib.unified_diff(actual.splitlines(True), expected.splitlines(True),
                            fromfile=str(path), tofile='fresh actual Go AST'))
            print(delta[:12000], file=sys.stderr)
    if drift:
        raise RuntimeError('source extraction drift: ' + ', '.join(drift) +
                           '; review changes then --refresh (SOURCE only); Bend must still prove frozen laws')
    return len(records)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--root', type=Path, default=ROOT)
    ap.add_argument('--refresh', action='store_true', help='update SOURCE artifacts only, never policy/laws/proofs')
    args = ap.parse_args()
    try:
        count = check(args.root, args.refresh)
    except (RuntimeError, OSError, ValueError, subprocess.TimeoutExpired) as exc:
        print(f'apienums: {exc}', file=sys.stderr)
        return 1
    print(f'apienums: {count} source-linked restricted Valid methods, independent declaration policy unchanged')
    return 0


if __name__ == '__main__':
    sys.exit(main())
