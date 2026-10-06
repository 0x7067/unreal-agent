"""Product-removal scope regression: preserve historic conclusions, not claims."""
import hashlib
import json
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
HISTORY = ROOT / 'tools/formal/models/history'
ACTIVE = ROOT / 'tools/formal/models/implementation/enums'


def declarations(path, header):
    """Compare declarations ignoring comments/blank lines, not conclusions."""
    text = re.sub(r'#[^\n]*', '', path.read_text())
    return {m.group(1): ''.join(m.group(2).split()) for m in re.finditer(
        header + r'([A-Za-z0-9_]+)(.*?)(?=\n(?:law |def )|\Z)', text, re.S)}


class FormalScopeTests(unittest.TestCase):
    def test_intact_original_component(self):
        expected = json.loads((HISTORY / 'enums-sha256.json').read_text())
        self.assertEqual(set(expected), {
            'LAWS.bend', 'PROOF.bend', 'POLICY.bend',
            'semantics.bend', 'SOURCE.bend', 'source_ast.json'})
        for name, digest in expected.items():
            self.assertEqual(hashlib.sha256((HISTORY/'enums'/name).read_bytes()).hexdigest(), digest, name)
        records = json.loads((HISTORY/'enums/source_ast.json').read_text())
        self.assertEqual(len(records), 11)
        self.assertEqual(len(declarations(HISTORY/'enums/LAWS.bend', r'law ')), 33)

    def test_surviving_law_conclusions_and_proof_terms_unchanged(self):
        for name, header, count in [('LAWS.bend', r'law ', 27),
                                    ('PROOF.bend', r'def Laws\.', 27)]:
            old = declarations(HISTORY/'enums'/name, header)
            current = declarations(ACTIVE/name, header)
            self.assertEqual(len(current), count)
            for symbol, declaration in current.items():
                self.assertEqual(declaration, old[symbol], symbol)
        for name in ('POLICY.bend', 'semantics.bend'):
            self.assertEqual((ACTIVE/name).read_bytes(), (HISTORY/'enums'/name).read_bytes())

    def test_registered_active_contracts_are_six_plus_476(self):
        records = json.loads((ACTIVE/'source_ast.json').read_text())
        self.assertEqual({r['id'] for r in records}, {
            'local_finished', 'primitive_completed', 'remote_transition',
            'coordinator_terminal', 'stored_terminal', 'reasoning_valid'})
        api = ROOT/'tools/formal/models/implementation/api-enums/source_ast.json'
        self.assertEqual(len(json.loads(api.read_text())), 476)
        old = {r['id']: r for r in json.loads((HISTORY/'enums/source_ast.json').read_text())}
        for record in records:
            for field in ('ast', 'inputs', 'dependencies', 'file', 'name', 'receiver', 'law', 'classification'):
                self.assertEqual(record[field], old[record['id']][field], (record['id'], field))


if __name__ == '__main__':
    unittest.main()
