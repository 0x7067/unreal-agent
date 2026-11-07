import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import prove
import verify


class StableTreeTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        subprocess.run(['git', 'init', '-q', str(self.root)], check=True)
        for d in ('tools/formal', 'docs/formal', 'pkg'):
            (self.root/d).mkdir(parents=True)
        (self.root/'pkg/a.go').write_text('package pkg\nfunc actual() bool {return true}\n')
        (self.root/'docs/formal/policy.json').write_text('{}\n')

    def run_gate(self, checking=None, compiling=None):
        with patch.object(verify, 'source_check', side_effect=checking), \
             patch.object(prove, 'check_structure', return_value=object()), \
             patch.object(prove, 'prove', side_effect=compiling):
            verify.verify(self.root, '/unused/test-bend', 5)

    def test_stable_inputs_allowed(self):
        self.run_gate()

    def test_ordinary_go_edit_during_proof_rejected(self):
        def change(*args):
            (self.root/'pkg/a.go').write_text('package pkg\nfunc actual() bool {return false}\n')
        with self.assertRaisesRegex(prove.ProofError, 'inputs changed'):
            self.run_gate(compiling=change)

    def test_coupled_go_and_source_artifact_edit_cannot_hide_behind_refresh(self):
        def change(*args):
            (self.root/'pkg/a.go').write_text('package pkg\nfunc actual() bool {return false}\n')
            (self.root/'docs/formal/SOURCE.bend').write_text('import Base\n')
            (self.root/'docs/formal/policy.json').write_text('{"changed": true}\n')
        with self.assertRaisesRegex(prove.ProofError, 'inputs changed'):
            self.run_gate(compiling=change)

    def test_new_source_file_during_check_rejected(self):
        def change(*args):
            (self.root/'pkg/new.go').write_text('package pkg\nfunc newFunction() {}\n')
        with self.assertRaisesRegex(prove.ProofError, 'inputs changed'):
            self.run_gate(compiling=change)

    def test_go_module_change_during_check_rejected(self):
        def change(*args):
            (self.root/'go.mod').write_text('module changed.invalid\n')
        with self.assertRaisesRegex(prove.ProofError, 'inputs changed'):
            self.run_gate(compiling=change)

    def test_final_source_check_failure_propagates(self):
        calls = iter([None, prove.ProofError('post-proof stale actual Go')])
        def checking(*args):
            result = next(calls)
            if result:
                raise result
        with self.assertRaisesRegex(prove.ProofError, 'post-proof stale'):
            self.run_gate(checking=checking)

    def test_symlink_input_refused(self):
        (self.root/'pkg/link.go').symlink_to(self.root/'pkg/a.go')
        with self.assertRaisesRegex(prove.ProofError, 'symlink'):
            verify.snapshot(self.root)


if __name__ == '__main__':
    unittest.main()
