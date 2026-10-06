import contextlib
import copy
import io
import hashlib
import json
import subprocess
import tempfile
import unittest
from pathlib import Path

import check_coverage as coverage


class CoverageTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        subprocess.run(['git', 'init', '-q', str(self.root)], check=True)
        (self.root / 'pkg').mkdir()
        (self.root / 'pkg/a.go').write_text('package pkg\n')
        model = 'tools/formal/models/example/LAWS.bend'
        (self.root / model).parent.mkdir(parents=True)
        (self.root / model).write_text('import Base\n')
        self.ledger = {
            'schema': 1, 'implementation_proven': False,
            'models': {model: {'scope': 'model only', 'gaps': 'no refinement', 'legacy': []}},
            'packages': {'pkg': {'status': 'unproved', 'models': [], 'gap': 'not proved',
                'source_sha256': coverage.digest(self.root, ['pkg/a.go'])}}}

    def check(self, ledger=None):
        with contextlib.redirect_stdout(io.StringIO()):
            coverage.validate(self.root, self.ledger if ledger is None else ledger)

    def test_current_unproved_inventory_passes_without_claiming_proof(self):
        self.check()

    def test_edited_source_rejected(self):
        (self.root / 'pkg/a.go').write_text('package pkg\n// edited\n')
        with self.assertRaisesRegex(ValueError, 'source changed'):
            self.check()

    def test_untracked_new_package_rejected(self):
        (self.root / 'new').mkdir()
        (self.root / 'new/a.go').write_text('package new\n')
        with self.assertRaisesRegex(ValueError, 'inventory drift'):
            self.check()

    def test_added_file_same_package_rejected(self):
        (self.root / 'pkg/b.go').write_text('package pkg\n')
        with self.assertRaisesRegex(ValueError, 'source changed'):
            self.check()

    def test_tests_do_not_count_as_implementation(self):
        (self.root / 'pkg/a_test.go').write_text('package pkg\n')
        self.check()

    def test_cannot_promote_to_implementation_proven(self):
        ledger = copy.deepcopy(self.ledger)
        ledger['implementation_proven'] = True
        with self.assertRaisesRegex(ValueError, 'implementation_proven=false'):
            self.check(ledger)
        ledger['implementation_proven'] = False
        ledger['packages']['pkg']['status'] = 'proven'
        with self.assertRaisesRegex(ValueError, 'invalid coverage status'):
            self.check(ledger)

    def test_unindexed_law_rejected(self):
        (self.root / 'tools/formal/models/other').mkdir()
        (self.root / 'tools/formal/models/other/LAWS.bend').write_text('import Base\n')
        with self.assertRaisesRegex(ValueError, 'law inventory'):
            self.check()

    def test_refresh_never_promotes_proof_status(self):
        (self.root / 'pkg/a.go').write_text('package pkg\n// acknowledged edit\n')
        coverage.refresh(self.root, self.ledger)
        self.assertEqual(self.ledger['packages']['pkg']['status'], 'unproved')
        self.assertIs(self.ledger['implementation_proven'], False)
        self.check()

    def test_refresh_new_package_defaults_to_unproved(self):
        (self.root / 'new').mkdir()
        (self.root / 'new/a.go').write_text('package new\n')
        coverage.refresh(self.root, self.ledger)
        self.assertEqual(self.ledger['packages']['new']['status'], 'unproved')
        self.assertEqual(self.ledger['packages']['new']['models'], [])
        self.check()

    def test_unmapped_legacy_source_rejected(self):
        (self.root / 'tools/formal/models/example/Old.tla').write_text('---- MODULE Old ----\n')
        with self.assertRaisesRegex(ValueError, 'legacy model/invariant source inventory'):
            self.check()

    def source_contract(self):
        model = 'tools/formal/models/example/LAWS.bend'
        (self.root/model).write_text('import Base\nlaw contract: Unit\n')
        manifest = 'tools/formal/models/example/source_ast.json'
        self.ledger['models'][model].update(
            source_contract_manifest=manifest,
            trust_classification='source-linked-restricted-semantics')
        record = {'file': 'pkg/a.go', 'name': 'predicate', 'receiver': '',
                  'classification': 'source-linked-restricted-semantics', 'law': 'contract',
                  'source_sha256': hashlib.sha256((self.root/'pkg/a.go').read_bytes()).hexdigest()}
        return manifest, record

    def test_source_contract_must_bind_public_law(self):
        manifest, record = self.source_contract()
        record['law'] = 'not_declared'
        (self.root/manifest).write_text(json.dumps([record]))
        with self.assertRaisesRegex(ValueError, 'unbound source contract'):
            self.check()

    def test_source_contract_trust_cannot_be_promoted(self):
        manifest, record = self.source_contract()
        record['classification'] = 'fully-proved-Go'
        (self.root/manifest).write_text(json.dumps([record]))
        with self.assertRaisesRegex(ValueError, 'classification mismatch'):
            self.check()

    def test_source_contract_hash_must_match_actual_file(self):
        manifest, record = self.source_contract()
        record['source_sha256'] = '0'*64
        (self.root/manifest).write_text(json.dumps([record]))
        with self.assertRaisesRegex(ValueError, 'source contract hash drift'):
            self.check()

    def test_missing_gap_rejected(self):
        self.ledger['packages']['pkg']['gap'] = ''
        with self.assertRaisesRegex(ValueError, 'residual obligation'):
            self.check()

    def test_symlinked_production_file_rejected(self):
        (self.root / 'pkg/b.go').symlink_to(self.root / 'pkg/a.go')
        with self.assertRaisesRegex(ValueError, 'symlinked'):
            self.check()


if __name__ == '__main__':
    unittest.main()
