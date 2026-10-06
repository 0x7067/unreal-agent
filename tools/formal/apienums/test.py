#!/usr/bin/env python3
"""stdlib fail-closed frontend and real-Bend semantic mutation regression tests."""
from contextlib import redirect_stderr
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('apienums_check', Path(__file__).with_name('check.py'))
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)
ROOT = checker.ROOT
DEST = checker.DEST
FILE = Path('internal/openaiapi/types.gen.go')
ENV = dict(os.environ, GOTOOLCHAIN='local', GOPROXY='off', GOSUMDB='off', BEND_NO_UPDATE='1')


class APIEnums(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.build_dir = tempfile.TemporaryDirectory(prefix='apienums binary ')
        cls.binary = Path(cls.build_dir.name) / 'extract'
        subprocess.run(['go', 'build', '-o', str(cls.binary), './tools/formal/apienums/extract.go'],
                       cwd=ROOT, env=ENV, capture_output=True, text=True, check=True, timeout=120)
        cls.bend = Path(os.environ.get('BEND', str(Path.home() / '.bend/bin/bend')))

    @classmethod
    def tearDownClass(cls):
        cls.build_dir.cleanup()

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='apienums mutation ')
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        shutil.copyfile(ROOT / "go.mod", self.root / "go.mod")
        for pkg in ('openaiapi', 'anthropicapi'):
            directory = self.root / 'internal' / pkg
            directory.mkdir(parents=True)
            for name in ('types.gen.go', 'generate.go'):
                shutil.copyfile(ROOT / directory.relative_to(self.root) / name, directory / name)
        shutil.copytree(ROOT / DEST, self.root / DEST)
        semantics = Path('tools/formal/models/implementation/enums/semantics.bend')
        (self.root / semantics).parent.mkdir(parents=True)
        shutil.copyfile(ROOT / semantics, self.root / semantics)

    def extract(self, root=None):
        result = subprocess.run([str(self.binary), str(root or self.root)], env=ENV,
                                capture_output=True, text=True, timeout=60)
        if result.returncode:
            raise RuntimeError(result.stderr)
        return json.loads(result.stdout)

    def check(self, refresh=False):
        with patch.object(checker, 'extract', self.extract), redirect_stderr(io.StringIO()):
            return checker.check(self.root, refresh)

    def replace(self, old, new, count=1):
        path = self.root / FILE
        text = path.read_text()
        self.assertIn(old, text)
        path.write_text(text.replace(old, new, count))

    def bend_check(self, succeeds):
        if not self.bend.is_file():
            self.skipTest('pinned Bend unavailable; frontend mutation checks still run')
        result = subprocess.run([str(self.bend), str(self.root / DEST / 'PROOF.bend'), '--check-only'],
                                env=ENV, capture_output=True, text=True, timeout=120)
        output = result.stdout + result.stderr
        if succeeds:
            self.assertEqual(result.returncode, 0, output[-8000:])
            self.assertIn('All terms check.', output)
        else:
            self.assertTrue(result.returncode != 0 or 'All terms check.' not in output,
                            'semantically wrong source unexpectedly proved')
            self.assertTrue(any(word in output.lower() for word in ('mismatch', 'fail', 'error')),
                            output[-8000:])

    def test_inventory_and_real_proof(self):
        self.assertEqual(self.check(), 476)
        data = self.extract()
        self.assertEqual(len(data['policy']), 476)
        self.assertEqual(sum(r['file'] == str(FILE) for r in data['records']), 279)
        self.assertTrue(all(r['classification'] == 'source-linked-restricted-semantics'
                            for r in data['records']))
        self.bend_check(True)

    def test_default_true_freshness_then_refreshed_bend_failure(self):
        protected = {name: hashlib.sha256((self.root / DEST / name).read_bytes()).digest()
                     for name in ('policy.json', 'POLICY.bend', 'LAWS.bend', 'PROOF.bend')}
        self.replace('default:\n\t\treturn false', 'default:\n\t\treturn true')
        with self.assertRaisesRegex(RuntimeError, 'source extraction drift'):
            self.check()
        self.assertEqual(self.check(True), 476)
        for name, digest in protected.items():
            self.assertEqual(hashlib.sha256((self.root / DEST / name).read_bytes()).digest(), digest)
        self.bend_check(False)

    def test_member_false_freshness_then_refreshed_bend_failure(self):
        self.replace('case AdditionalToolsTypeAdditionalTools:\n\t\treturn true',
                     'case AdditionalToolsTypeAdditionalTools:\n\t\treturn false')
        with self.assertRaisesRegex(RuntimeError, 'source extraction drift'):
            self.check()
        self.check(True)
        self.bend_check(False)

    def test_missing_case_after_refresh_fails_bend(self):
        self.replace('\tcase ApplyPatchCallOutputStatusCompleted:\n\t\treturn true\n', '')
        self.check(True)
        self.bend_check(False)

    def test_changed_constant_cannot_refresh_policy(self):
        self.replace('AdditionalToolsType = "additional_tools"', 'AdditionalToolsType = "changed"')
        with self.assertRaisesRegex(RuntimeError, 'DECLARATION POLICY MISMATCH'):
            self.check(True)

    def test_new_declared_value_not_in_method_cannot_refresh_policy(self):
        self.replace('AdditionalToolsTypeAdditionalTools AdditionalToolsType = "additional_tools"',
                     'AdditionalToolsTypeAdditionalTools AdditionalToolsType = "additional_tools"\n'
                     '\tNewIndependentValue AdditionalToolsType = "not_in_method"')
        with self.assertRaisesRegex(RuntimeError, 'DECLARATION POLICY MISMATCH'):
            self.check(True)

    def test_removed_constant_rejected(self):
        self.replace('AdditionalToolsTypeAdditionalTools AdditionalToolsType = "additional_tools"', '')
        with self.assertRaisesRegex(RuntimeError, 'undefined: AdditionalToolsTypeAdditionalTools'):
            self.check(True)

    def test_case_with_wrong_named_string_type_rejected(self):
        self.replace('case AdditionalToolsTypeAdditionalTools:',
                     'case AdditionalToolsItemParamTypeAdditionalTools:')
        with self.assertRaisesRegex(RuntimeError, 'mismatched types'):
            self.extract()

    def test_changed_type_rejected(self):
        self.replace('type AdditionalToolsType string', 'type AdditionalToolsType int')
        with self.assertRaisesRegex(RuntimeError, 'defined directly over string'):
            self.check(True)

    def test_alias_receiver_rejected(self):
        self.replace('type AdditionalToolsType string', 'type AdditionalToolsType = string')
        with self.assertRaisesRegex(RuntimeError, 'no aliases'):
            self.extract()

    def test_constant_expression_resolution_and_raw_bytes(self):
        # Not text matching: equivalent constant expressions decode identically.
        self.replace('AdditionalToolsType = "additional_tools"',
                     'AdditionalToolsType = ("additional_" + `tools`)')
        self.assertEqual(self.check(True), 476)
        self.replace('AdditionalToolsType = ("additional_" + `tools`)',
                     'AdditionalToolsType = "\\x00\\xffé"')
        data = self.extract()
        self.assertEqual(data['policy'][0]['constants'][0]['bytes'], [0, 255, 195, 169])
        self.assertEqual(data['records'][0]['ast']['args'][0]['bytes'], [0, 255, 195, 169])
        with self.assertRaisesRegex(RuntimeError, 'DECLARATION POLICY MISMATCH'):
            self.check(True)

    def test_policy_bend_tampering_rejected(self):
        path = self.root / DEST / 'POLICY.bend'
        path.write_text(path.read_text().replace('97n', '98n', 1))
        with self.assertRaisesRegex(RuntimeError, 'POLICY.bend does not match'):
            self.check(True)

    def test_unsupported_go_ast_not_silently_dropped(self):
        original = (self.root / FILE).read_text()
        mutations = [
            ('func (e AdditionalToolsType) Valid() bool {',
             'func (e *AdditionalToolsType) Valid() bool {'),
            ('func (e AdditionalToolsType) Valid() bool {',
             'func (e AdditionalToolsType) Valid() (result bool) {'),
            ('func (e AdditionalToolsType) Valid() bool {',
             'func (e AdditionalToolsType) Valid() bool { e = "";'),
            ('switch e {', 'switch e := e; e {'),
            ('switch e {', 'switch AdditionalToolsType("") {'),
            ('case AdditionalToolsTypeAdditionalTools:\n\t\treturn true',
             'case AdditionalToolsTypeAdditionalTools:\n\t\tfallthrough'),
            ('case AdditionalToolsTypeAdditionalTools:\n\t\treturn true',
             'case AdditionalToolsTypeAdditionalTools:\n\t\treturn len(e) > 0'),
            ('case AdditionalToolsTypeAdditionalTools:', 'case "additional_tools":'),
            ('default:\n\t\treturn false', 'default:\n\t\tpanic("not total")'),
            ('type AdditionalToolsType string', 'type AdditionalToolsType []byte'),
        ]
        for old, new in mutations:
            with self.subTest(mutation=new):
                (self.root / FILE).write_text(original.replace(old, new, 1))
                with self.assertRaises(RuntimeError):
                    self.extract()
        (self.root / FILE).write_text(original)

    def test_predeclared_shadowing_rejected_even_after_refresh(self):
        path = self.root / FILE
        original = path.read_text()
        for declaration in ('var true = false', 'var false = true',
                            'type bool = string', 'type string = int'):
            with self.subTest(declaration=declaration):
                path.write_text(original + '\n' + declaration + '\n')
                with self.assertRaisesRegex(RuntimeError, 'predeclared .* shadowed'):
                    self.check(True)
        path.write_text(original)

    def test_sibling_predeclared_shadowing_rejected_even_after_refresh(self):
        sibling = self.root / FILE.parent / 'shadow.go'
        for declaration in ('var true = false', 'var false = true',
                            'type bool = string', 'type string = int',
                            'func true() bool { return false }'):
            with self.subTest(declaration=declaration):
                sibling.write_text('package openaiapi\n' + declaration + '\n')
                with self.assertRaisesRegex(RuntimeError, 'predeclared .*shadowed'):
                    self.check(True)

    def test_new_sibling_dependency_requires_freshness_review(self):
        sibling = self.root / FILE.parent / 'harmless.go'
        sibling.write_text('package openaiapi\nvar unimportant = 0\n')
        with self.assertRaisesRegex(RuntimeError, 'source extraction drift'):
            self.check()
        self.check(True)
        manifest = json.loads((self.root / DEST / 'source_ast.json').read_text())
        dependencies = manifest[0]['dependencies']
        self.assertIn(str(sibling.relative_to(self.root)), {d['file'] for d in dependencies})
        self.assertTrue(all(len(d['source_sha256']) == 64 for d in dependencies))
        sibling.write_text('package openaiapi\nvar unimportant = 1\n')
        with self.assertRaisesRegex(RuntimeError, 'source extraction drift'):
            self.check()

    def test_sibling_compiler_directives_and_imports_rejected(self):
        sibling = self.root / FILE.parent / 'extra.go'
        for statement in ('//go:linkname danger target\nfunc danger() {}',
                          '//go:noinline\nfunc other() {}',
                          'import . "strings"', 'import string "strings"'):
            with self.subTest(statement=statement):
                sibling.write_text('package openaiapi\n' + statement + '\n')
                with self.assertRaisesRegex(RuntimeError, 'compiler directives|unsupported import'):
                    self.check(True)

    def test_valid_in_new_file_explicitly_rejected(self):
        (self.root / FILE.parent / 'extra.go').write_text(
            'package openaiapi\ntype Extra string\nfunc (e Extra) Valid() bool { return true }\n')
        with self.assertRaisesRegex(RuntimeError, 'outside inventoried'):
            self.extract()

    def test_deleted_method_inventory_explicitly_rejected(self):
        self.replace('func (e AdditionalToolsType) Valid()', 'func (e AdditionalToolsType) Changed()')
        # Constants for an unselected type can fail restricted typechecking first;
        # either way no method silently disappears from the evidence.
        with self.assertRaises(RuntimeError):
            self.check(True)

    def test_nested_module_boundaries_rejected(self):
        for relative in ('internal/go.mod', 'internal/openaiapi/go.mod',
                         'internal/anthropicapi/go.mod'):
            with self.subTest(relative=relative):
                path = self.root / relative
                path.write_text('module attacker.invalid/submodule\ngo 1.27\n')
                with self.assertRaisesRegex(RuntimeError, 'nested (Go )?module'):
                    self.check(True)
                path.unlink()

    def test_noncanonical_root_module_rejected(self):
        (self.root / 'go.mod').write_text('module attacker.invalid/root\ngo 1.27\n')
        with self.assertRaises(RuntimeError):
            self.check(True)

    def test_root_module_metadata_requires_freshness_review(self):
        path = self.root / 'go.mod'
        path.write_text(path.read_text() + '\n// metadata reviewed independently\n')
        with self.assertRaisesRegex(RuntimeError, 'source extraction drift'):
            self.check()
        self.check(True)
        manifest = json.loads((self.root / DEST / 'source_ast.json').read_text())
        self.assertEqual(manifest[0]['module_metadata_sha256']['go.mod'],
                         hashlib.sha256(path.read_bytes()).hexdigest())

    def test_overlay_workspace_environment_rejected(self):
        for name, value in (('GOFLAGS', '-overlay=/tmp/substitution.json'),
                            ('GOWORK', '/tmp/substitution.work')):
            with self.subTest(name=name), patch.dict(os.environ, {name: value}):
                with self.assertRaises(RuntimeError):
                    self.check(True)

    def test_symlink_source_rejected(self):
        path = self.root / FILE
        original = path.read_text()
        target = self.root / 'outside.go'
        target.write_text(original)
        path.unlink()
        path.symlink_to(target)
        with self.assertRaisesRegex(RuntimeError, 'symlink/escape'):
            self.extract()

    def test_symlink_artifact_rejected(self):
        path = self.root / DEST / 'source_ast.json'
        other = self.root / 'other.json'
        other.write_text(path.read_text())
        path.unlink()
        path.symlink_to(other)
        with self.assertRaisesRegex(RuntimeError, 'symlink/escape'):
            self.check(True)


if __name__ == '__main__':
    unittest.main()
