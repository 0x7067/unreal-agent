import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import completion


class GoInventoryTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workspace = tempfile.TemporaryDirectory()
        cls.binary = Path(cls.workspace.name) / 'inventory'
        subprocess.run(['go', 'build', '-o', str(cls.binary),
                        str(completion.ROOT/'tools/formal/inventory/main.go')],
                       check=True, env={**os.environ, 'GOTOOLCHAIN': 'local'}, timeout=120)

    @classmethod
    def tearDownClass(cls):
        cls.workspace.cleanup()

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        subprocess.run(['git', 'init', '-q', str(self.root)], check=True)

    def write(self, path, contents):
        destination = self.root/path
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text(contents)
        return destination

    def inventory(self):
        run = subprocess.run([str(self.binary), '--root', str(self.root)], capture_output=True, text=True)
        self.assertEqual(run.returncode, 0, run.stderr)
        return json.loads(run.stdout)

    def test_named_functions_methods_and_repeated_init(self):
        self.write('pkg/a.go', 'package pkg\ntype Node struct{}\nfunc top() {}\nfunc(n *Node) run() {}\nfunc init() {}\nfunc init() {}\n')
        source = self.inventory()['sources'][0]
        self.assertEqual([f['id'] for f in source['functions']], [
            'pkg/a.go::top', 'pkg/a.go::(*Node).run', 'pkg/a.go::init', 'pkg/a.go::init#2'])
        self.assertTrue(all(len(f['body_sha256']) == 64 for f in source['functions']))

    def test_all_build_variants_included_not_only_host(self):
        self.write('pkg/a_linux.go', '//go:build linux\n\npackage pkg\nfunc platform() {}\n')
        self.write('pkg/a_windows.go', '//go:build windows\n\npackage pkg\nfunc platform() {}\n')
        sources = self.inventory()['sources']
        self.assertEqual(len(sources), 2)
        self.assertEqual(sources[1]['build_constraints'], ['//go:build windows'])

    def test_tests_and_ignored_cache_not_counted(self):
        self.write('.gitignore', '/cache/\n')
        self.write('pkg/a.go', 'package pkg\nfunc real() {}\n')
        self.write('pkg/a_test.go', 'package pkg\nfunc testOnly() {}\n')
        self.write('cache/ignored.go', 'package ignored\nfunc notOurs() {}\n')
        self.assertEqual(len(self.inventory()['sources']), 1)

    def test_body_changes_are_detected(self):
        self.write('pkg/a.go', 'package pkg\nfunc changed() bool {return true}\n')
        old = self.inventory()['sources'][0]['functions'][0]['body_sha256']
        self.write('pkg/a.go', 'package pkg\nfunc changed() bool {return false}\n')
        self.assertNotEqual(old, self.inventory()['sources'][0]['functions'][0]['body_sha256'])

    def test_parser_error_refuses_inventory(self):
        self.write('pkg/a.go', 'package pkg\nfunc invalid( {\n')
        result = subprocess.run([str(self.binary), '--root', str(self.root)], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('parse', result.stderr)

    def test_symlink_source_refused(self):
        original = self.write('pkg/a.go', 'package pkg\n')
        (original.parent/'b.go').symlink_to(original)
        result = subprocess.run([str(self.binary), '--root', str(self.root)], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('symlink', result.stderr)

    def test_completion_reports_unproved_actual_declarations(self):
        self.write('pkg/a.go', 'package pkg\nfunc unproved() {}\n')
        report = completion.inspect(self.root)
        self.assertEqual(report['named_function_declarations'], 1)
        self.assertEqual(report['unresolved'], ['pkg/a.go::unproved'])
        self.assertFalse(report['complete'])
        self.assertEqual(report['whole_implementation_refinement_proofs'], 0)

    def test_historical_manifest_is_not_an_active_source_contract(self):
        self.write('pkg/a.go', 'package pkg\nfunc actual() {}\n')
        self.write('tools/formal/models/history/enums/source_ast.json', json.dumps([{
            'file': 'deleted/feature.go', 'name': 'historical', 'receiver': '',
            'classification': 'source-linked-restricted-semantics'}]))
        report = completion.inspect(self.root)
        self.assertEqual(report['registered_restricted_semantics_contracts'], 0)
        self.assertEqual(report['unresolved'], ['pkg/a.go::actual'])
        self.assertFalse(report['complete'])

    def test_missing_source_contract_declaration_refused(self):
        self.write('pkg/a.go', 'package pkg\nfunc actual() {}\n')
        self.write('tools/formal/models/implementation/enums/source_ast.json', json.dumps([{
            'file':'pkg/a.go', 'name':'imaginary', 'receiver':'',
            'classification':'source-linked-restricted-semantics'}]))
        with self.assertRaisesRegex(ValueError, 'missing/duplicate'):
            completion.inspect(self.root)

    def test_registered_contract_is_not_whole_implementation_proof(self):
        self.write('pkg/a.go', 'package pkg\nfunc actual() {}\n')
        self.write('tools/formal/models/implementation/enums/source_ast.json', json.dumps([{
            'file':'pkg/a.go', 'name':'actual', 'receiver':'',
            'classification':'source-linked-restricted-semantics'}]))
        report = completion.inspect(self.root)
        self.assertEqual(report['registered_restricted_semantics_contracts'], 1)
        self.assertEqual(report['declarations_without_source_linked_contract'], 0)
        self.assertFalse(report['complete'])


if __name__ == '__main__':
    unittest.main()
