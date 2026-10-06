#!/usr/bin/env python3
"""Review ledger freshness, NOT a verifier or proof of implementation equivalence."""
import argparse
import hashlib
import json
import re
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
LEDGER = Path('tools/formal/models/coverage.json')
STATES = {'unproved', 'model-only', 'model-with-tests'}


def packages(root):
    """Include tracked and new nonignored Go production files, across modules."""
    result = subprocess.run(
        ['git', '-C', str(root), 'ls-files', '--cached', '--others',
         '--exclude-standard', '-z'], check=True, capture_output=True)
    found = {}
    for name in sorted(set(result.stdout.decode().split('\0'))):
        if not name.endswith('.go') or name.endswith('_test.go'):
            continue
        path = root / name
        if path.is_symlink() or not path.is_file():
            raise ValueError(f'production source missing or symlinked: {name}')
        if not path.resolve().is_relative_to(root.resolve()):
            raise ValueError(f'production source escapes root: {name}')
        found.setdefault(str(Path(name).parent), []).append(name)
    return found


def digest(root, names):
    h = hashlib.sha256()
    for name in names:
        h.update(name.encode() + b'\0')
        h.update(hashlib.sha256((root / name).read_bytes()).digest())
    return h.hexdigest()


def validate(root, ledger):
    if ledger.get('schema') != 1 or ledger.get('implementation_proven') is not False:
        raise ValueError('ledger must explicitly state implementation_proven=false')
    actual = packages(root)
    entries = ledger['packages']
    if set(actual) != set(entries):
        raise ValueError(f'package inventory drift: new={sorted(set(actual)-set(entries))}, '
                         f'removed={sorted(set(entries)-set(actual))}')
    for package, names in actual.items():
        entry = entries[package]
        if entry['status'] not in STATES:
            raise ValueError(f'{package}: invalid coverage status (no Go refinement established)')
        if entry['source_sha256'] != digest(root, names):
            raise ValueError(f'{package}: source changed; review laws/tests/gaps, then refresh ledger')
        if not entry.get('gap', '').strip():
            raise ValueError(f'{package}: missing residual obligation')
        models = entry.get('models', [])
        if (entry['status'] == 'unproved') != (not models):
            raise ValueError(f'{package}: status/model mismatch')
        for model in models:
            path = root / model
            if model not in ledger['models'] or not path.is_file():
                raise ValueError(f'{package}: unknown/missing model {model}')
    discovered = {str(p.relative_to(root)) for p in (root/'tools/formal/models').rglob('LAWS.bend')}
    if discovered != set(ledger['models']):
        raise ValueError('law inventory differs from coverage model inventory')
    legacy_mapped = []
    for model, entry in ledger['models'].items():
        if not entry.get('scope') or not entry.get('gaps'):
            raise ValueError(f'{model}: missing model scope/gaps')
        manifest = entry.get('source_contract_manifest')
        if manifest:
            if entry.get('trust_classification') != 'source-linked-restricted-semantics':
                raise ValueError(f'{model}: unknown source proof trust classification')
            law_names = set(re.findall(r'^law ([A-Za-z0-9_]+):', (root/model).read_text(), re.M))
            records = json.loads((root/manifest).read_text())
            seen = set()
            for record in records:
                key = (record['file'], record.get('receiver', ''), record['name'])
                if key in seen or record.get('law') not in law_names:
                    raise ValueError(f'{model}: duplicate or unbound source contract {key}')
                seen.add(key)
                if record.get('classification') != entry['trust_classification']:
                    raise ValueError(f'{model}: source contract classification mismatch')
                if record['source_sha256'] != hashlib.sha256((root/record['file']).read_bytes()).hexdigest():
                    raise ValueError(f'{model}: source contract hash drift (AST re-extraction required)')
            if not seen:
                raise ValueError(f'{model}: empty source contract inventory')
        for legacy in entry.get('legacy', []):
            legacy_mapped.append(legacy)
            if not (root / legacy).is_file():
                raise ValueError(f'{model}: missing retained legacy evidence {legacy}')
    legacy_actual = {str(p.relative_to(root)) for suffix in ('*.lean', '*.tla')
                     for p in (root/'tools/formal/models').rglob(suffix)
                     if '.lake' not in p.relative_to(root).parts}
    if set(legacy_mapped) != legacy_actual or len(legacy_mapped) != len(set(legacy_mapped)):
        raise ValueError('legacy model/invariant source inventory drift or duplicate mapping')
    print(f'coverage: {len(actual)} Go package directories inventoried; '
          f'{sum(e["status"] != "unproved" for e in entries.values())} have model links; '
          '0 implementation-refinement proofs (not fully verified)')


def refresh(root, ledger):
    """Explicit review action; preserves classifications, never promotes coverage."""
    old = ledger['packages']
    ledger['packages'] = {
        package: {
            **old.get(package, {
                'status': 'unproved', 'models': [],
                'gap': 'No Bend implementation-refinement proof; laws and correspondence remain to be developed.'}),
            'source_sha256': digest(root, names),
        }
        for package, names in sorted(packages(root).items())
    }
    (root / LEDGER).write_text(json.dumps(ledger, indent=2) + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--refresh', action='store_true', help='after reviewing changed source, acknowledge hashes; does not prove anything')
    parser.add_argument('--require-complete', action='store_true', help='fail unless full implementation coverage is established (currently unavailable)')
    args = parser.parse_args()
    try:
        ledger = json.loads((ROOT / LEDGER).read_text())
        if args.refresh:
            refresh(ROOT, ledger)
        validate(ROOT, ledger)
        if args.require_complete:
            raise ValueError('complete implementation verification is NOT established; see coverage.json residual obligations')
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print(f'coverage: FAIL: {error}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
