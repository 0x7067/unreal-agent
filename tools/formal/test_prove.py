"""stdlib regression tests; real Bend tests skip unless the pinned binary exists."""

from contextlib import redirect_stderr, redirect_stdout
import io
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

# Works both with unittest discovery and direct execution, without installation.
sys.path.insert(0, str(Path(__file__).resolve().parent))
import prove


class Fixture(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="formal test ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.component = self.root / "docs/formal/example/bend"
        self.component.mkdir(parents=True)
        self.write("LAWS.bend", "import ./docs/formal/example/bend/LAWS.bend as ExampleLaws\n")
        self.write("PROOF.bend", "import ./LAWS.bend as Laws\nimport ./docs/formal/example/bend/PROOF.bend as ExampleProof\n")
        self.write("docs/formal/example/bend/LAWS.bend", "import Base\nlaw reflexive:\n  {True{} == True{} : Bool}\n")
        self.write("docs/formal/example/bend/PROOF.bend", "import Base\nimport ./LAWS.bend as Claims\ndef Claims.reflexive():\n  {==}\n")

    def write(self, relative, text):
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def append(self, relative, text):
        path = self.root / relative
        path.write_text(path.read_text() + text, encoding="utf-8")

    def structure(self):
        return prove.check_structure(self.root)

    def rejected(self, pattern=None):
        return self.assertRaisesRegex(prove.ProofError, pattern or ".")


class StructureTests(Fixture):
    def test_valid_aggregate_alias_and_component(self):
        project = self.structure()
        self.assertEqual(project.components, [self.component])
        self.assertEqual(project.targets, [self.component / "PROOF.bend", self.root / "PROOF.bend"])

    def test_offline_never_invokes_or_finds_bend(self):
        output = io.StringIO()
        with patch.object(prove, "find_bend", side_effect=AssertionError("must be offline")), patch.object(subprocess, "run", side_effect=AssertionError("must not execute")), redirect_stdout(output):
            self.assertEqual(prove.main(["--root", str(self.root), "--check-structure", "--bend", "/missing"]), 0)
        self.assertIn("proofs NOT checked", output.getvalue())

    def test_missing_root_proof_import(self):
        self.write("PROOF.bend", "import ./LAWS.bend as Laws\n")
        with self.rejected("root PROOF.bend missing direct import"):
            self.structure()

    def test_missing_root_law_import(self):
        self.write("LAWS.bend", "# empty index\n")
        with self.rejected("root LAWS.bend missing direct import"):
            self.structure()

    def test_missing_root_sibling_law_import(self):
        self.write("PROOF.bend", "import ./docs/formal/example/bend/PROOF.bend as Proof\n")
        with self.rejected("directly import sibling"):
            self.structure()

    def test_missing_component_sibling_law_import(self):
        self.write("docs/formal/example/bend/PROOF.bend", "def Claims.reflexive():\n  {==}\n")
        with self.rejected("directly import sibling"):
            self.structure()

    def test_missing_sibling_proof(self):
        (self.component / "PROOF.bend").unlink()
        with self.rejected():
            self.structure()

    def test_orphan_component_without_root_imports(self):
        self.write("docs/formal/new/bend/LAWS.bend", "import Base\nlaw added:\n  {True{} == True{} : Bool}\n")
        self.write("docs/formal/new/bend/PROOF.bend", "import ./LAWS.bend as New\ndef New.added():\n  {==}\n")
        with self.rejected("missing direct import"):
            self.structure()

    def test_new_orphan_laws_without_proof(self):
        self.write("docs/formal/new/LAWS.bend", "law added:\n  Unit\n")
        with self.rejected("requires a sibling"):
            self.structure()

    def test_orphan_helper_is_not_silently_unchecked(self):
        self.write("docs/formal/unreferenced.bend", "import Base\ndef broken() -> Bool:\n  3n\n")
        with self.rejected("not reachable"):
            self.structure()

    def test_laws_outside_pair_rejected(self):
        self.write("docs/formal/example/bend/helper.bend", "import Base\nlaw hidden:\n  {True{} == True{} : Bool}\ndef hidden():\n  {==}\n")
        self.append("docs/formal/example/bend/PROOF.bend", "import ./helper.bend as Helper\n")
        with self.rejected("must be in paired LAWS"):
            self.structure()

    def test_missing_law_implementation(self):
        self.append("docs/formal/example/bend/LAWS.bend", "law newly_added:\n  {True{} == True{} : Bool}\n")
        with self.rejected("missing law implementations.*newly_added"):
            self.structure()

    def test_wrong_alias_implementation(self):
        self.write("docs/formal/example/bend/PROOF.bend", "import ./LAWS.bend as Claims\ndef Wrong.reflexive():\n  {==}\n")
        with self.rejected("missing law implementations"):
            self.structure()

    def test_commented_or_string_def_cannot_fill_law(self):
        for fake in ['# def Claims.reflexive():', 'def text() -> String:\n  "def Claims.reflexive():"']:
            with self.subTest(fake=fake):
                self.write("docs/formal/example/bend/PROOF.bend", "import ./LAWS.bend as Claims\n" + fake + "\n")
                with self.rejected("missing law implementations"):
                    self.structure()

    def test_empty_component_rejected(self):
        self.write("docs/formal/example/bend/LAWS.bend", "import Base\n")
        with self.rejected("no law declarations"):
            self.structure()

    def test_root_own_law_needs_implementation(self):
        self.append("LAWS.bend", "law root_law:\n  {True{} == True{} : Bool}\n")
        with self.rejected("missing law implementations.*root_law"):
            self.structure()
        self.append("PROOF.bend", "def Laws.root_law():\n  {==}\n")
        self.structure()

    def test_imported_helpers_valid(self):
        self.write("docs/formal/example/bend/helper.bend", "import Base\ndef helper() -> {True{} == True{} : Bool}:\n  {==}\n")
        self.write("docs/formal/example/bend/PROOF.bend", "import ./LAWS.bend as Claims\nimport ./helper.bend as H\ndef Claims.reflexive():\n  H.helper()\n")
        self.structure()

    def test_unsafe_foreign_and_legacy_markers(self):
        base = (self.component / "PROOF.bend").read_text()
        for marker in ["@unsafe def bad() -> Unit:\n  Unit{}", "@ unsafe\ndef bad() -> Unit:\n  Unit{}", "def bad?() -> Unit:\n  Unit{}", "def bad ? () -> Unit:\n  Unit{}", "foreign", "@foreign", "extern"]:
            with self.subTest(marker=marker):
                self.write("docs/formal/example/bend/PROOF.bend", base + marker + "\n")
                with self.rejected("unsafe/foreign"):
                    self.structure()

    def test_comment_and_string_markers_are_not_directives(self):
        self.append("docs/formal/example/bend/PROOF.bend", '# @unsafe foreign def hidden?() import hub@1/x.bend\ndef text() -> String:\n  "# @unsafe foreign import fake.bend"\n')
        self.structure()

    def test_unsafe_unimported_source_still_rejected(self):
        self.write("docs/formal/unimported.bend", "@unsafe def evil() -> Unit:\n  evil()\n")
        with self.rejected("unsafe/foreign"):
            self.structure()

    def test_no_remote_absolute_quoted_or_foreign_imports(self):
        base = (self.component / "PROOF.bend").read_text()
        for line in ["import example@1.0/main.bend as M", "import 0x123/main.bend as M", "import https://example/x.bend as M", "import /tmp/x.bend as M", 'import "./x.bend" as M', '  import "./foreign.c"', '  import "./foreign.js"', 'import ./x.js as M', 'import Base trailing', '  import Base']:
            with self.subTest(line=line):
                self.write("docs/formal/example/bend/PROOF.bend", base + line + "\n")
                with self.rejected("imports allowed|unsafe/foreign"):
                    self.structure()

    def test_parent_relative_import_inside_root_valid(self):
        self.write("docs/formal/example/helper.bend", "import Base\n")
        self.append("docs/formal/example/bend/PROOF.bend", "import ../helper.bend as H\n")
        self.structure()

    def test_import_escape_rejected(self):
        self.append("PROOF.bend", "import ../outside.bend as Escape\n")
        with self.rejected("escapes project"):
            self.structure()

    def test_symlink_source_rejected(self):
        original = self.component / "LAWS.bend"
        destination = self.component / "real.bend"
        original.rename(destination)
        original.symlink_to(destination.name)
        with self.rejected("symlink"):
            self.structure()

    def test_symlink_directory_rejected(self):
        (self.root / "docs/formal/link").symlink_to(self.component, target_is_directory=True)
        with self.rejected("symlink"):
            self.structure()

    def test_import_symlink_before_dotdot_rejected(self):
        (self.root / "link").symlink_to(self.component, target_is_directory=True)
        self.append("PROOF.bend", "import ./link/../LAWS.bend as Escape\n")
        with self.rejected("symlink"):
            self.structure()

    def test_missing_import_rejected(self):
        self.append("PROOF.bend", "import ./missing.bend as M\n")
        with self.rejected("missing/non-regular"):
            self.structure()

    def test_import_outside_seed_scanned(self):
        self.write("local/helper.bend", "@unsafe def bad() -> Unit:\n  bad()\n")
        self.append("PROOF.bend", "import ./local/helper.bend as H\n")
        with self.rejected("unsafe/foreign"):
            self.structure()

    def test_external_cache_worktree_is_not_seeded(self):
        self.write("vendor/cache/not-our-proof.bend", "@unsafe def bad() -> Unit:\n  bad()\n")
        self.structure()

    def test_duplicate_law_rejected(self):
        self.append("docs/formal/example/bend/LAWS.bend", "law reflexive:\n  Unit\n")
        with self.rejected("duplicate law"):
            self.structure()

    def test_duplicate_import_alias_rejected(self):
        self.write("docs/formal/example/bend/helper.bend", "import Base\n")
        self.append("docs/formal/example/bend/PROOF.bend", "import ./helper.bend as Claims\n")
        with self.rejected("duplicate import alias"):
            self.structure()

    def test_duplicate_import_path_rejected(self):
        self.append("docs/formal/example/bend/PROOF.bend", "import ./LAWS.bend as Again\n")
        with self.rejected("duplicate import"):
            self.structure()

    def test_unterminated_literal_rejected(self):
        self.append("docs/formal/example/bend/PROOF.bend", 'def bad() -> String:\n  "unterminated\n')
        with self.rejected("unterminated literal"):
            self.structure()


class RunnerTests(Fixture):
    def call(self, *, stdout="All terms check.\n", stderr="", returncode=0, expected="All terms check."):
        result = subprocess.CompletedProcess([], returncode, stdout, stderr)
        with patch.object(prove.subprocess, "run", return_value=result) as run:
            prove.invoke("/mock/bend", ["PROOF.bend", "--check-only"], self.root, 5, expected)
        return run

    def test_exact_clean_success_and_no_telemetry(self):
        run = self.call()
        args, kwargs = run.call_args
        self.assertEqual(args[0], ["/mock/bend", "PROOF.bend", "--check-only"])
        self.assertEqual(kwargs["env"]["BEND_NO_TELEMETRY"], "1")
        self.assertEqual(kwargs["stdin"], subprocess.DEVNULL)
        self.assertEqual(kwargs["timeout"], 5)

    def test_known_update_notice_accepted(self):
        self.call(stderr="bend 2.0.39 is available: run bend update\n")

    def test_unsafe_foreign_taint_even_zero_exit(self):
        for text in ["All terms check, but 2 defs rely on unsafe or foreign code:\n- LAWS.bad\n", "All terms check.\nAll terms check, but 1 defs rely on unsafe or foreign code:", "SOME PROOFS FAIL", "All terms check.\nunsafe"]:
            with self.subTest(text=text), self.rejected("exact required clean result"):
                self.call(stdout=text)

    def test_malformed_output_rejected(self):
        for text in ["", "All terms check", "All terms check!", "prefix All terms check.", "All terms check.\nError: invalid proof", "All terms check.\nAll terms check.", "\x1b[32mAll terms check.\x1b[0m"]:
            with self.subTest(text=text), self.rejected():
                self.call(stdout=text)

    def test_error_or_unsafe_stderr_rejected_even_clean_stdout(self):
        for text in ["Error: bad", "unsafe", "foreign", "All terms check.", "bend 2.0.39 is available: run bend update\nmalicious notice"]:
            with self.subTest(text=text), self.rejected():
                self.call(stderr=text)

    def test_nonzero_exit_rejected_even_clean_output(self):
        with self.rejected("Bend failed"):
            self.call(returncode=1)

    def test_exact_version_required(self):
        self.call(stdout="bend 2.0.27\n", expected="bend 2.0.27")
        for version in ["bend 2.0.26", "bend 2.0.270", "Bend 2.0.27: help", "bend 2.0.27\nError", "bend 2.0.39"]:
            with self.subTest(version=version), self.rejected():
                self.call(stdout=version, expected="bend 2.0.27")

    def test_all_components_and_root_checked_separately(self):
        project = self.structure()
        calls = []
        def fake(bend, args, root, timeout, expected):
            calls.append((args, expected))
        with patch.object(prove, "invoke", side_effect=fake), redirect_stdout(io.StringIO()):
            prove.prove(project, "/mock/bend", 5)
        self.assertEqual(calls, [(["version"], "bend 2.0.27"), (["docs/formal/example/bend/PROOF.bend", "--check-only"], "All terms check."), (["PROOF.bend", "--check-only"], "All terms check.")])

    def test_explicit_missing_binary_never_falls_back(self):
        with self.rejected("not found"):
            prove.find_bend("/definitely/not/a/bend")
        with patch.dict(os.environ, {"BEND": "/definitely/not/a/bend"}), self.rejected("not found"):
            prove.find_bend(None)

    def test_missing_default_binary(self):
        with patch.dict(os.environ, {}, clear=True), patch.object(prove.shutil, "which", return_value=None), patch.object(prove.Path, "home", return_value=self.root), self.rejected("nothing was fetched"):
            prove.find_bend(None)

    def test_empty_explicit_binary_rejected(self):
        with self.rejected("empty"):
            prove.find_bend("")

    def test_missing_bend_cli_fails_closed(self):
        error = io.StringIO()
        with redirect_stderr(error):
            status = prove.main(["--root", str(self.root), "--bend", "/definitely/not/a/bend"])
        self.assertEqual(status, 1)
        self.assertIn("FAIL", error.getvalue())

    def test_timeout_cli_fails_closed(self):
        with patch.object(prove, "find_bend", return_value="/mock/bend"), patch.object(prove.subprocess, "run", side_effect=subprocess.TimeoutExpired("bend", 1)), redirect_stderr(io.StringIO()):
            self.assertEqual(prove.main(["--root", str(self.root)]), 1)

    def test_source_edits_during_check_fail(self):
        project = self.structure()
        def change(*args):
            self.append("docs/formal/example/bend/PROOF.bend", "# changed\n")
        with patch.object(prove, "invoke", side_effect=change), redirect_stdout(io.StringIO()), self.rejected("changed during checking"):
            prove.prove(project, "/mock/bend", 5)

    def test_invalid_timeout_values(self):
        for value in ["0", "-1", "nan", "inf"]:
            with self.subTest(value=value), self.assertRaises(SystemExit), redirect_stderr(io.StringIO()):
                prove.main(["--timeout", value])

    def test_mock_executable_full_cli_wrong_version_and_taint(self):
        binary = self.root / "mock bend"
        for version, output, expected in [("bend 2.0.27", "All terms check.", 0), ("bend 2.0.28", "All terms check.", 1), ("bend 2.0.27", "All terms check, but 1 defs rely on unsafe or foreign code:", 1), ("bend 2.0.27", "", 1)]:
            with self.subTest(version=version, output=output):
                binary.write_text(f"#!{sys.executable}\nimport os,sys\nassert os.environ['BEND_NO_TELEMETRY'] == '1'\nassert sys.argv[1:] == ['version'] or sys.argv[-1] == '--check-only'\nprint({version!r} if sys.argv[1:] == ['version'] else {output!r})\n")
                binary.chmod(0o755)
                result = subprocess.run([sys.executable, str(Path(prove.__file__)), "--root", str(self.root), "--bend", str(binary)], capture_output=True, text=True, timeout=15)
                self.assertEqual(result.returncode, expected, result.stdout + result.stderr)


class RealCheckerTests(Fixture):
    def setUp(self):
        super().setUp()
        try:
            self.bend = prove.find_bend(None)
            prove.invoke(self.bend, ["version"], self.root, 15, "bend 2.0.27")
        except (prove.ProofError, OSError, subprocess.TimeoutExpired):
            self.skipTest("pinned Bend 2.0.27 not installed; mock/offline tests remain active")

    def test_missing_law_implementation_is_an_actual_checker_error(self):
        path = self.component / "PROOF.bend"
        path.write_text("import ./LAWS.bend as Claims\n")
        env = dict(os.environ, BEND_NO_TELEMETRY="1")
        result = subprocess.run([self.bend, str(path), "--check-only"], capture_output=True, text=True, timeout=15, env=env)
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("TODO", result.stdout + result.stderr)

    def test_real_clean_fixture_passes(self):
        with redirect_stdout(io.StringIO()):
            prove.prove(self.structure(), self.bend, 15)

    def test_real_unsafe_zero_exit_output_rejected(self):
        path = self.write("unsafe-fixture.bend", "import Base\n@unsafe def loop() -> {True{} == False{} : Bool}:\n  loop()\n")
        env = dict(os.environ, BEND_NO_TELEMETRY="1")
        result = subprocess.run([self.bend, str(path), "--check-only"], capture_output=True, text=True, timeout=15, env=env)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("unsafe or foreign code", result.stdout + result.stderr)
        with self.rejected("exact required clean result"):
            prove.invoke(self.bend, [str(path), "--check-only"], self.root, 15, "All terms check.")


if __name__ == "__main__":
    unittest.main()
