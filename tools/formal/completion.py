#!/usr/bin/env python3
"""Report implementation-proof obligations from actual Go syntax, never from a claim flag.

This inventory does NOT prove functions, type initialization, concurrency or I/O.
Source-linked restricted-subset contracts are separately counted, never promoted
into unconditional whole-package refinement proofs.
"""
from __future__ import annotations

import argparse
from collections import Counter
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]


def inspect(root: Path) -> dict:
    inventory = subprocess.run(
        ['go', 'run', str(ROOT / 'tools/formal/inventory/main.go'), '--root', str(root)],
        cwd=ROOT, env={**os.environ, "GOTOOLCHAIN": "local"}, capture_output=True, text=True, check=True, timeout=120)
    data = json.loads(inventory.stdout)
    if data.get('schema') != 1 or not data.get('sources'):
        raise ValueError('empty or unsupported Go source inventory')
    functions = {f['id']: f for source in data['sources'] for f in source['functions']}
    contracts = []
    for claimed in sorted((root/'tools/formal/models/implementation').glob('*/source_ast.json')):
        records = json.loads(claimed.read_text())
        if not isinstance(records, list):
            raise ValueError(f'unsupported source-linked inventory: {claimed}')
        contracts.extend(records)
    if not isinstance(contracts, list):
        raise ValueError('unsupported source-linked contract inventory')
    linked = set()
    for contract in contracts:
        receiver = contract.get('receiver', '')
        symbol = f'({receiver}).' if receiver else ''
        key = f'{contract["file"]}::{symbol}{contract["name"]}'
        if key not in functions or key in linked:
            raise ValueError(f'missing/duplicate source declaration in source contract: {key}')
        if contract.get('classification') != 'source-linked-restricted-semantics':
            raise ValueError(f'unknown contract trust classification: {key}')
        linked.add(key)
    by_package = Counter(str(Path(f['file']).parent) for f in functions.values())
    return {
        'schema': 1,
        'toolchain': data['toolchain'],
        'production_source_files': len(data['sources']),
        'named_function_declarations': len(functions),
        'registered_restricted_semantics_contracts': len(linked),
        'declarations_without_source_linked_contract': len(functions.keys() - linked),
        'whole_implementation_refinement_proofs': 0,
        'complete': False,
        'packages': dict(sorted(by_package.items())),
        'unresolved': sorted(functions.keys() - linked),
        'additional_obligations': [
            'Prove or explicitly trust the Go-subset extraction/semantics bridge; restricted contracts do not prove the Go compiler.',
            'Specify and prove all remaining function bodies, including nested closures and build variants.',
            'Verify package initialization, globals, type/layout/interface behavior and dependency boundaries.',
            'Connect actor/channel/heap/cancellation behavior to the abstract protocol proofs.',
            'Discharge filesystem/process/network/crash-durability assumptions or state them in the trusted environment contract.',
        ],
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=ROOT)
    parser.add_argument('--json', action='store_true', help='print full live inventory and unresolved declarations')
    parser.add_argument('--require-complete', action='store_true', help='refuse success while implementation proof obligations remain')
    args = parser.parse_args(argv)
    try:
        report = inspect(args.root.resolve())
        if args.json:
            print(json.dumps(report, indent=2))
        else:
            print(f'completion: {report["production_source_files"]} production Go files; '
                  f'{report["named_function_declarations"]} named function/method declarations; '
                  f'{report["registered_restricted_semantics_contracts"]} registered restricted-semantics contracts; '
                  f'{report["declarations_without_source_linked_contract"]} declarations without a source-linked contract')
            print('completion: NOT complete; no whole-implementation refinement proof. '
                  'Registered contracts require source freshness + Bend checking; this report is not a verifier.')
        if args.require_complete and not report['complete']:
            print('completion: FAIL: unresolved implementation obligations remain; '
                  'see --json for the actual declarations and semantic/environment gaps', file=sys.stderr)
            return 1
        return 0
    except (ValueError, KeyError, OSError, subprocess.SubprocessError) as error:
        print(f'completion: FAIL: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
