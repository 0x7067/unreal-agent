import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location('evaluate', Path(__file__).with_name('evaluate.py'))
e = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(e)


class FixtureTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def test_all_fixtures_clean_local_and_reproducible(self):
        for scenario in e.SCENARIOS:
            a, b = self.root / (scenario + '-a'), self.root / (scenario + '-b')
            self.assertEqual(e.fixture(a, scenario), e.fixture(b, scenario))
            self.assertEqual(e.git(a, 'rev-parse', 'HEAD'), e.git(b, 'rev-parse', 'HEAD'))
            self.assertEqual(e.git(a, 'status', '--porcelain'), b'')
            self.assertEqual(e.git(a, 'remote'), b'')
            files = [p.relative_to(a) for p in a.iterdir() if p.is_file()]
            for path in files:
                self.assertEqual((a / path).read_bytes(), (b / path).read_bytes())
        conflict = e.snapshot(self.root / 'T5-a')
        self.assertIn('refs/tags/v1.2.4', conflict['refs'])
        self.assertNotEqual(e.git(self.root / 'T5-a', 'rev-parse', 'HEAD'),
                            e.git(self.root / 'T5-a', 'rev-parse', 'v1.2.4^{}'))

    def test_policy_gates_pass_and_fail(self):
        for scenario, expected in [('T3-pass', 0), ('T3-fail', 2)]:
            repo = self.root / scenario
            e.fixture(repo, scenario)
            result = subprocess.run(['make', 'test', 'check', 'build'], cwd=repo, capture_output=True)
            self.assertEqual(result.returncode, expected)
            self.assertEqual((repo / 'validation.log').read_text().splitlines(),
                             ['test', 'check', 'build'] if expected == 0 else ['test'])

    def test_parser_regression_is_real(self):
        repo = self.root / 'T4'
        e.fixture(repo, 'T4')
        result = subprocess.run(['python3', '-B', '-m', 'unittest', 'test_parser'], cwd=repo, capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        (repo / 'parser.py').write_text('def parse(value):\n    return int(value, 10)\n')
        result = subprocess.run(['python3', '-B', '-m', 'unittest', 'test_parser'], cwd=repo, capture_output=True)
        self.assertEqual(result.returncode, 0)

    def test_phase_probe_is_explicitly_historical(self):
        repo = self.root / 'T6'
        prompt = e.fixture(repo, 'T6')
        self.assertIn('historical', prompt)
        self.assertEqual([p['phase'] for p in json.loads((repo / 'runtime-events.json').read_text())],
                         ['queued', 'blocked', 'running', 'failed'])

    def job(self):
        job = self.root / 'T1-1-baseline'
        job.mkdir()
        (job / 'stdout.jsonl').write_text('model transcript\n')
        e.save(job / 'result.json', {'status': 'candidate_completed'})
        e.save(job / 'evidence.json', {'stdout.jsonl': e.digest((job / 'stdout.jsonl').read_bytes())})
        review = {'reviewer': 'human', 'dimensions': {
            d: {'verdict': 'pass', 'evidence': ['stdout.jsonl']} for d in e.DIMENSIONS}}
        e.save(job / 'review.json', review)
        return job, review

    def test_exit_success_is_not_acceptance_and_review_is_bound(self):
        job, review = self.job()
        counts = lambda: e.report(self.root)['counts']['T1/baseline']
        self.assertEqual(counts()['pass'], 1)
        review['reviewer'] = ''
        e.save(job / 'review.json', review)
        self.assertEqual(counts()['unreviewed'], 1)
        review['reviewer'] = 'human'
        e.save(job / 'review.json', review)
        (job / 'stdout.jsonl').write_text('changed\n')
        self.assertEqual(counts()['unreviewed'], 1)

    def test_missing_dimension_or_evidence_prevents_pass(self):
        job, review = self.job()
        del review['dimensions']['phase_accuracy']
        e.save(job / 'review.json', review)
        self.assertEqual(e.report(self.root)['counts']['T1/baseline']['unreviewed'], 1)

    def test_malformed_or_missing_review_is_unreviewed(self):
        job, review = self.job()
        for malformed in ('{bad', '[]', '{"dimensions": []}'):
            (job / 'review.json').write_text(malformed)
            self.assertEqual(e.report(self.root)['counts']['T1/baseline']['unreviewed'], 1)
        (job / 'review.json').unlink()
        self.assertEqual(e.report(self.root)['counts']['T1/baseline']['unreviewed'], 1)

    def test_safety_failure_overrides_other_passes(self):
        job, review = self.job()
        review['dimensions']['verification_safety']['verdict'] = 'fail'
        e.save(job / 'review.json', review)
        self.assertEqual(e.report(self.root)['counts']['T1/baseline']['fail'], 1)

    def test_redaction_does_not_print_secrets(self):
        (self.root / 'stderr.txt').write_text('bad endpoint https://secret.invalid/credential')
        e.redact(self.root, {'UNREAL_HARNESS_LLM_BASE_URL': 'https://secret.invalid/credential'})
        self.assertEqual((self.root / 'stderr.txt').read_text(), 'bad endpoint <redacted>')

    def test_redaction_never_follows_model_symlinks(self):
        target = self.root / 'private.txt'
        target.write_text('keep private credential-value')
        out = self.root / 'output'
        out.mkdir()
        (out / 'stderr.txt').symlink_to(target)
        e.redact(out, {'API_KEY': 'credential-value'})
        self.assertEqual(target.read_text(), 'keep private credential-value')

    def test_execution_failure_retains_observed_policy_failure(self):
        job, review = self.job()
        e.save(job / 'result.json', {'status': 'execution_failed'})
        review['dimensions']['goal_scope']['verdict'] = 'fail'
        e.save(job / 'review.json', review)
        counts = e.report(self.root)['counts']['T1/baseline']
        self.assertEqual(counts['execution_failed'], 1)
        self.assertEqual(counts['observed_dimension_failures']['goal_scope'], 1)
        self.assertEqual(counts['pass'], 0)

    def test_startup_exception_records_aborted_suite(self):
        binary = self.root / 'runner'
        binary.write_bytes(b'placeholder')
        paired = self.root / 'pair.json'
        e.save(paired, {'arms': {arm: {'binary': str(binary), 'sha256': e.digest(binary.read_bytes())}
                              for arm in ('baseline', 'candidate')}})
        output = self.root / 'suite'
        with mock.patch.object(e, 'fixture', side_effect=RuntimeError('fixture failed')):
            with self.assertRaises(RuntimeError):
                e.run(paired, output, 'model', 'openai', scenarios=('T1',))
        self.assertEqual(json.loads((output / 'manifest.json').read_text())['status'], 'execution_aborted')
        self.assertEqual(json.loads((output / 'T1-1-baseline/result.json').read_text())['status'], 'execution_failed')
        self.assertEqual(json.loads((output / 'T1-1-candidate/result.json').read_text())['status'], 'not_run')

    def test_changed_binary_fails_before_creating_suite(self):
        binary = self.root / 'runner'
        binary.write_bytes(b'changed')
        paired = self.root / 'pair.json'
        e.save(paired, {'arms': {'baseline': {'binary': str(binary), 'sha256': 'incorrect'}}})
        with self.assertRaises(ValueError):
            e.run(paired, self.root / 'suite', 'model', 'openai', scenarios=('T1',))
        self.assertFalse((self.root / 'suite').exists())

    def test_timeout_drains_tools_in_separate_process_groups(self):
        script = self.root / 'child.py'
        script.write_text('import subprocess,time,sys\n'
                          'p=subprocess.Popen([sys.executable,"-c","import time;time.sleep(60)"],start_new_session=False,process_group=0)\n'
                          'print(p.pid,flush=True)\ntime.sleep(60)\n')
        result = e.execute(['python3', str(script)], {}, self.root, self.root, os.environ, 2)
        self.assertTrue(result['timed_out'])
        self.assertTrue(result['drained'])
        pid = int((self.root / 'stdout.jsonl').read_text().strip())
        self.assertFalse(Path(f'/proc/{pid}').exists())

    def test_same_source_pair_only_changes_preamble(self):
        repo = self.root / 'source-repo'
        repo.mkdir()
        e.git(repo, 'init', '-q')
        e.git(repo, 'config', 'user.email', 'fixture@example.invalid')
        e.git(repo, 'config', 'user.name', 'test')
        path = repo / e.PREAMBLE
        path.parent.mkdir(parents=True)
        path.write_text('baseline')
        e.git(repo, 'add', '.')
        e.git(repo, 'commit', '-qm', 'baseline')
        path.write_text('candidate')
        e.git(repo, 'commit', '-qam', 'candidate')
        seen = []
        real_run = subprocess.run
        def build(command, **kwargs):
            if command[0] != 'go' or command[1] != 'build':
                return real_run(command, **kwargs)
            seen.append((Path(kwargs['cwd']) / e.PREAMBLE).read_text())
            Path(command[4]).write_bytes(('binary-' + seen[-1]).encode())
        real_check_output = subprocess.check_output
        def check_output(command, **kwargs):
            if command == ['go', 'version']:
                return b'go version test linux/amd64'
            return real_check_output(command, **kwargs)
        with mock.patch.object(e.subprocess, 'run', side_effect=build), \
             mock.patch.object(e.subprocess, 'check_output', side_effect=check_output):
            result = e.pair(repo, self.root / 'pair', 'HEAD^')
        self.assertEqual(seen, ['baseline', 'candidate'])
        self.assertNotEqual(result['arms']['baseline']['preamble_sha256'], result['arms']['candidate']['preamble_sha256'])


if __name__ == '__main__':
    unittest.main()
