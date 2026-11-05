#!/usr/bin/env python3
"""Compile scratch mutations of real production Go using Go's source overlay.

Never edits production files. A compiler/tool error does NOT count as detecting
an unsafe change. Each mutant must compile and fail its named behavioral check.
"""
import json
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[4]
MUTANTS = [
    (
        "skip-status-commit", "harness/coordinator/loop.go",
        "func (current *coordinator) scheduleToolCall(",
        "current.storeItemInSessionStore(ctx, item)",
        "func(sessionstore.Item) error { return nil }(item)",
        "TestDispatchCommitBeforeAdd", "Add before matching committed initialization",
    ),
    (
        "remove-terminal-guard", "harness/coordinator/loop.go",
        "func (current *coordinator) dispatchOperationToManager(",
        "if operationIsTerminal(value.Status) {",
        "if false && operationIsTerminal(value.Status) {",
        "TestDispatchExhaustiveRecovery", "unsafe Add after crash",
    ),
    (
        "omit-latest-overlay", "harness/coordinator/loop.go",
        "func (current *coordinator) restore(",
        "current.addOperationToLocalState(value)", "_ = value",
        "TestDispatchExhaustiveRecovery", "unsafe Add after crash",
    ),
    (
        "drop-pending-terminal", "harness/sessionstore/localfile/state.go",
        "func (state storedState) resume(",
        "if !terminalOperationStatus(value.Status) || unrecorded {",
        "if !terminalOperationStatus(value.Status) || (unrecorded && false) {",
        "TestDispatchExhaustiveRecovery", "recovery lost or changed checkpoint",
    ),
]


def main():
    with tempfile.TemporaryDirectory(prefix="coordinator-dispatch-mutants-") as directory:
        scratch = Path(directory)
        for name, relative, function, old, new, test, diagnostic in MUTANTS:
            source = ROOT / relative
            text = source.read_text()
            start = text.index(function)
            end = text.find("\nfunc ", start + 1)
            if end == -1:
                end = len(text)
            body = text[start:end]
            if body.count(old) != 1:
                raise SystemExit(f"{name}: source moved; review mutation and code mapping")
            replacement = scratch / f"{name}.go"
            replacement.write_text(text[:start] + body.replace(old, new) + text[end:])
            overlay = scratch / f"{name}.json"
            overlay.write_text(json.dumps({"Replace": {str(source): str(replacement)}}))
            result = subprocess.run(
                ["go", "test", f"-overlay={overlay}", "./harness/coordinator",
                 "-run", f"^{test}$", "-count=1"],
                cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                timeout=120,
            )
            if (result.returncode != 1 or diagnostic not in result.stdout
                    or f"--- FAIL: {test}" not in result.stdout
                    or "[build failed]" in result.stdout):
                print(result.stdout)
                raise SystemExit(f"{name}: expected a compiled behavioral failure, exit={result.returncode}")
            # Include a concrete shortest failing subtest and diagnostic, not
            # thousands of repeated longer traces from the bounded enumeration.
            print(f"{name}: caught scratch production-code mutation")
            shown = 0
            for line in result.stdout.splitlines():
                if "--- FAIL:" in line or diagnostic in line:
                    print(line)
                    shown += 1
                    if shown == 3:
                        break
    print("All four scratch changes caught; production files were never edited.")


if __name__ == "__main__":
    main()
