#!/usr/bin/env python3
"""Actual-source negative regressions (temp trees only), not proof evidence."""
import contextlib
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
from unittest import mock

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('gosource_check', HERE / 'check.py')
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)
ROOT = checker.ROOT
COMPONENT = checker.DEST
BEND = os.environ.get('BEND', str(Path.home() / '.bend/bin/bend'))


class ExtractionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='gosource-negative-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        shutil.copy2(ROOT / "go.mod", self.root / "go.mod")
        self.records = json.loads((ROOT / COMPONENT / 'source_ast.json').read_text())
        # Packages read by the actual AST extractor, not handwritten fixtures.
        dirs = {str(Path(r['file']).parent) for r in self.records}
        dirs |= {str(Path(p).parent) for r in self.records for p in r['dependencies']}
        for directory in dirs:
            dest = self.root / directory
            dest.mkdir(parents=True)
            for path in (ROOT / directory).glob('*.go'):
                if not path.name.endswith('_test.go'):
                    shutil.copy2(path, dest / path.name)
        shutil.copytree(ROOT / COMPONENT, self.root / COMPONENT)

    def assert_rejected(self, reason=None):
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(RuntimeError) as raised:
                checker.check(self.root)
        if reason:
            self.assertIn(reason, str(raised.exception))

    def replace(self, file, old, new):
        path = self.root / file
        text = path.read_text()
        self.assertIn(old, text)
        path.write_text(text.replace(old, new, 1))

    def mutate_predicate(self, record):
        path = self.root / record['file']
        text = path.read_text()
        header = (f'func {record["name"]}(' if not record['receiver']
                  else 'func (effort ReasoningEffort) Valid(')
        start = text.index(header)
        opening = text.index('{', start)
        depth = 1
        end = opening + 1
        while depth:
            depth += (text[end] == '{') - (text[end] == '}')
            end += 1
        body = text[opening:end]
        if 'return true' in body:
            body = body.replace('return true', 'return false', 1)
        else:
            self.assertIn('==', body)
            body = body.replace('==', '!=', 1)
        path.write_text(text[:opening] + body + text[end:])

    def refresh_hashes_only(self):
        path = self.root / COMPONENT / 'source_ast.json'
        records = json.loads(path.read_text())
        for record in records:
            record['source_sha256'] = hashlib.sha256((self.root / record['file']).read_bytes()).hexdigest()
            record['dependency_sha256'] = {
                name: hashlib.sha256((self.root / name).read_bytes()).hexdigest()
                for name in record['dependencies']
            }
        path.write_text(json.dumps(records, indent=2) + '\n')

    def test_current_actual_sources_are_fresh(self):
        checker.check(self.root)

    def test_all_eleven_actual_predicates_mutated_even_with_refreshed_hashes(self):
        for record in self.records:
            with self.subTest(predicate=record['id']):
                path = self.root / record['file']
                original = path.read_bytes()
                original_manifest = (self.root / COMPONENT / 'source_ast.json').read_bytes()
                self.mutate_predicate(record)
                self.refresh_hashes_only()
                self.assert_rejected('source extraction drift')
                path.write_bytes(original)
                (self.root / COMPONENT / 'source_ast.json').write_bytes(original_manifest)

    def test_explicit_refresh_never_changes_independent_policy_and_proof_fails(self):
        if not Path(BEND).is_file():
            self.skipTest('Bend binary unavailable; source negative tests still run')
        protected = {name: (self.root / COMPONENT / name).read_bytes()
                     for name in ('POLICY.bend', 'LAWS.bend', 'PROOF.bend')}
        for record in self.records:
            with self.subTest(predicate=record['id']):
                path = self.root / record['file']
                original = path.read_bytes()
                self.mutate_predicate(record)
                checker.check(self.root, refresh=True)
                for name, content in protected.items():
                    self.assertEqual(content, (self.root / COMPONENT / name).read_bytes())
                result = subprocess.run([BEND, str(self.root / COMPONENT / 'PROOF.bend'), '--check-only'],
                                        capture_output=True, text=True, timeout=60,
                                        env={**os.environ, 'BEND_NO_TELEMETRY': '1'})
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                path.write_bytes(original)
                checker.check(self.root, refresh=True)

    def test_total_if_else_is_extracted_from_actual_modified_source(self):
        record = self.records[0]
        path = self.root / record['file']
        text = path.read_text()
        old = """switch status {
	case StatusCompleted, StatusFailed, StatusCanceled:
		return true
	default:
		return false
	}"""
        new = """if status == StatusCompleted {
            return true
        } else if status != StatusFailed && !(status == StatusCanceled) {
            return false
        } else {
            return true
        }"""
        self.assertIn(old, text)
        path.write_text(text.replace(old, new, 1))
        data = checker.artifacts(self.root)
        records = json.loads(data['source_ast.json'])
        ast = records[0]['ast']
        self.assertEqual(ast['kind'], 'if')
        self.assertEqual(ast['args'][2]['kind'], 'if')
        self.assertEqual(ast['args'][2]['args'][0]['kind'], 'and')
        self.assert_rejected('source extraction drift')

    def test_arbitrary_literal_bytes_not_unicode_scalar_sampling(self):
        self.replace('harness/operation/operation.go', 'Status = "completed"',
                     r'Status = "\xff\x00é"')
        records = json.loads(checker.artifacts(self.root)['source_ast.json'])
        self.assertEqual(records[0]['ast']['args'][0]['args'][0]['bytes'], [255, 0, 195, 169])
        self.assert_rejected('source extraction drift')

    def test_nontotal_if_rejected(self):
        self.replace(self.records[0]['file'], 'return true',
                     'if status == StatusCompleted { return true }')
        self.assert_rejected('non-total block')

    def test_root_module_identity_rejected_even_refresh(self):
        self.replace('go.mod', 'module github.com/unreallabsai/unreal-agent', 'module example.org/fake')
        with self.assertRaisesRegex(RuntimeError, 'unsupported root module path'):
            checker.check(self.root, refresh=True)

    def test_root_self_subpath_override_rejected_even_refresh(self):
        path = self.root / 'go.mod'
        with path.open('a') as out:
            out.write('\nrequire github.com/unreallabsai/unreal-agent/harness/operation v1.0.0\n'
                      'replace github.com/unreallabsai/unreal-agent/harness/operation => ./override\n')
        override = self.root / 'override'
        override.mkdir()
        (override / 'go.mod').write_text('module github.com/unreallabsai/unreal-agent/harness/operation\n')
        (override / 'status.go').write_text('package operation\ntype Status string\nconst StatusCompleted Status = "not-completed"\n')
        (self.root / 'harness/operation/go.mod').write_text('module github.com/unreallabsai/unreal-agent/harness/operation\n')
        with self.assertRaisesRegex(RuntimeError, 'root-module'):
            checker.check(self.root, refresh=True)

    def test_even_currently_unused_root_subpath_replace_rejected(self):
        with (self.root / 'go.mod').open('a') as out:
            out.write('\nreplace github.com/unreallabsai/unreal-agent/harness/operation => ./override\n')
        with self.assertRaisesRegex(RuntimeError, 'root-module replacement'):
            checker.check(self.root, refresh=True)

    def test_root_subpath_require_rejected(self):
        with (self.root / 'go.mod').open('a') as out:
            out.write('\nrequire github.com/unreallabsai/unreal-agent/harness/operation v1.0.0\n')
        self.assert_rejected('root-module subpath require')

    def test_nested_module_rejected_on_dependency(self):
        (self.root / 'harness/primitives/go.mod').write_text('module example.org/primitives\n')
        with self.assertRaisesRegex(RuntimeError, 'nested Go module'):
            checker.check(self.root, refresh=True)

    def test_nested_module_rejected_on_target_ancestor(self):
        (self.root / 'cmd/go.mod').write_text('module example.org/cmd\n')
        self.assert_rejected('nested Go module')

    def test_gopath_instead_of_module_mode_rejected(self):
        with mock.patch.dict(os.environ, {'GO111MODULE': 'off'}):
            with self.assertRaisesRegex(RuntimeError, 'module-disabled'):
                checker.check(self.root, refresh=True)

    def test_effective_overlay_goflags_rejected(self):
        overlay = self.root / 'overlay.json'
        overlay.write_text('{"Replace": {}}')
        with mock.patch.dict(os.environ, {'GOFLAGS': f'-overlay={overlay}'}):
            with self.assertRaisesRegex(RuntimeError, 'effective GOFLAGS'):
                checker.check(self.root, refresh=True)

    def test_persisted_overlay_goflags_rejected(self):
        overlay = self.root / 'overlay.json'
        overlay.write_text('{"Replace": {}}')
        goenv = self.root / 'goenv'
        goenv.write_text(f'GOFLAGS=-overlay={overlay}\n')
        with mock.patch.dict(os.environ, {'GOENV': str(goenv)}):
            os.environ.pop('GOFLAGS', None)
            self.assert_rejected('effective GOFLAGS')

    def test_auto_workspace_ancestor_rejected(self):
        ancestor = self.root / 'workspace'
        ancestor.mkdir()
        selected = ancestor / 'nested-root'
        selected.mkdir()
        shutil.copy2(ROOT / 'go.mod', selected / 'go.mod')
        (ancestor / 'go.work').write_text('go 1.27.0\nuse ./nested-root\n')
        with mock.patch.dict(os.environ, {'GOWORK': ''}):
            with self.assertRaisesRegex(RuntimeError, 'active Go workspace'):
                checker.check_environment(selected)

    def test_module_metadata_provenance_changes_with_reviewed_metadata(self):
        before = checker.check_environment(self.root)
        with (self.root / 'go.mod').open('a') as out:
            out.write('\n// reviewed harmless metadata edit\n')
        after = checker.check_environment(self.root)
        self.assertNotEqual(before, after)
        self.assert_rejected('source extraction drift')

    def test_call_is_rejected(self):
        self.replace(self.records[0]['file'], 'return true', 'return externalEffect()')
        self.assert_rejected('unsupported expression')

    def test_assignment_side_effect_rejected(self):
        self.replace(self.records[0]['file'], 'return true', 'status = StatusFailed; return true')
        self.assert_rejected('unsupported statement')

    def test_pointer_parameter_rejected(self):
        self.replace(self.records[0]['file'], 'func localOperationFinished(status Status)',
                     'func localOperationFinished(status *Status)')
        self.assert_rejected('unsupported name')

    def test_pointer_projection_rejected(self):
        self.replace('cmd/internal/workerruntime/workload_registry.go',
                     'func terminalOperation(op operation.Operation)',
                     'func terminalOperation(op *operation.Operation)')
        self.assert_rejected('unsupported name')

    def test_build_constraint_rejected(self):
        path = self.root / self.records[0]['file']
        path.write_text('//go:build linux\n\n' + path.read_text())
        self.assert_rejected('build constraints unsupported')

    def test_dependency_build_constraint_rejected(self):
        path = self.root / 'harness/operation/operation.go'
        path.write_text('//go:build linux\n\n' + path.read_text())
        self.assert_rejected('build constraints unsupported')

    def test_pointer_projected_status_field_rejected(self):
        self.replace('harness/operation/operation.go', 'Status          Status',
                     'Status          *Status')
        self.assert_rejected('unsupported name')

    def test_enum_alias_rejected(self):
        self.replace('harness/operation/operation.go', 'type Status string', 'type Status = string')
        self.assert_rejected('alias/generic')

    def test_enum_cycle_rejected(self):
        self.replace('harness/operation/operation.go', 'type Status string', 'type Status Status')
        self.assert_rejected('directly have string')

    def test_nonliteral_constant_rejected(self):
        self.replace('harness/operation/operation.go', 'Status = "completed"', 'Status = "com" + "pleted"')
        self.assert_rejected('only literal string constants')

    def test_parameter_shadowing_constant_rejected(self):
        self.replace('harness/operation/remote_job_handler.go',
                     'func validRemoteJobStatusTransition(current Status, updated Status)',
                     'func validRemoteJobStatusTransition(current Status, StatusAwaiting Status)')
        self.assert_rejected()

    def test_source_symlink_rejected(self):
        path = self.root / self.records[0]['file']
        content = path.read_bytes()
        path.unlink()
        outside = self.root / 'outside.go'
        outside.write_bytes(content)
        path.symlink_to(outside)
        self.assert_rejected('symlink source path')

    def test_dependency_directory_symlink_rejected(self):
        path = self.root / 'harness/primitives'
        dest = self.root / 'primitives-copy'
        path.rename(dest)
        path.symlink_to(dest, target_is_directory=True)
        self.assert_rejected('symlink source path')

    def test_artifact_symlink_rejected(self):
        path = self.root / COMPONENT / 'SOURCE.bend'
        path.unlink()
        path.symlink_to(ROOT / COMPONENT / 'SOURCE.bend')
        self.assert_rejected('symlink artifact path')


if __name__ == '__main__':
    unittest.main()
