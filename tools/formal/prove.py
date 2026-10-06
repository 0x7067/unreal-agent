#!/usr/bin/env python3
"""Fail-closed gate for indexed Bend models and restricted source-linked contracts.

The lexical guards deliberately accept a restricted source/import convention;
they are NOT a Bend parser or a substitute for Bend's type/termination checker.
--check-structure is offline and does not certify any proof. No installer,
package manager, publish operation, or executable model is ever invoked.
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import math
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

PINNED_VERSION = "2.0.27"
NAME = r"[A-Za-z_][A-Za-z0-9_]*"
QUALIFIED = NAME + r"(?:\." + NAME + r")*"
IMPORT = re.compile(r"import (Base|(?:\./|\.\./)[A-Za-z0-9_./-]+\.bend)(?: as (" + NAME + r"))?")
LAW = re.compile(r"law (" + NAME + r")\s*:\s*$")
DEF = re.compile(r"def (" + QUALIFIED + r")\s*\(")
NOTICE = re.compile(r"bend [0-9]+\.[0-9]+\.[0-9]+ is available: run bend update")


class ProofError(Exception):
    """A condition on which the gate must refuse success."""


@dataclass
class Source:
    path: Path
    raw: str
    imports: dict[Path, str | None]
    laws: set[str]
    definitions: set[str]


@dataclass
class Project:
    root: Path
    sources: dict[Path, Source]
    components: list[Path]

    @property
    def targets(self) -> list[Path]:
        return [directory / "PROOF.bend" for directory in self.components] + [self.root / "PROOF.bend"]


def safe_file(root: Path, candidate: Path) -> Path:
    """Check every path segment *before* resolving ..; never follow symlinks."""
    try:
        parts = candidate.relative_to(root).parts
    except ValueError as exc:
        raise ProofError(f"path escapes project: {candidate}") from exc
    current = root
    for part in parts:
        current = current.parent if part == ".." else current / part
        if not current.is_relative_to(root):
            raise ProofError(f"path escapes project: {candidate}")
        if current.is_symlink():
            raise ProofError(f"symlink is not allowed: {current}")
    if not current.is_file():
        raise ProofError(f"missing/non-regular source: {current}")
    return current


def code_only(text: str, path: Path) -> str:
    """Mask # comments and quoted literals, preserving columns/newlines.

    Reject unterminated literals and control characters rather than guessing.
    Masking prevents comments/string contents from impersonating declarations.
    """
    result = list(text)
    quote = None
    escaped = False
    comment = False
    for index, char in enumerate(text):
        if ord(char) < 32 and char not in "\n\r\t":
            raise ProofError(f"control character in {path}")
        if char == "\n":
            comment = False
            if quote is not None:
                raise ProofError(f"multiline/unterminated literal is unsupported: {path}")
            continue
        if comment:
            result[index] = " "
        elif quote is not None:
            result[index] = " "
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == quote:
                quote = None
        elif char == "#":
            comment = True
            result[index] = " "
        elif char in ('"', "'"):
            quote = char
            result[index] = " "
    if quote is not None:
        raise ProofError(f"unterminated literal: {path}")
    return "".join(result)


def read_source(root: Path, path: Path) -> Source:
    path = safe_file(root, path)
    raw = path.read_text(encoding="utf-8")
    code = code_only(raw, path)
    imports: dict[Path, str | None] = {}
    aliases: set[str] = set()
    laws: set[str] = set()
    definitions: set[str] = set()
    for number, line in enumerate(code.splitlines(), 1):
        where = f"{path.relative_to(root)}:{number}"
        # Bend also accepts `def f?` as an unsafe definition (not just @unsafe).
        if "@" in line or re.search(r"\b(?:foreign|extern)\b", line) or re.search(r"\bdef\s+" + QUALIFIED + r"\s*\?", line):
            raise ProofError(f"unsafe/foreign marker or unsupported decorator: {where}")
        if re.search(r"\bimport\b", line):
            match = IMPORT.fullmatch(line.rstrip())
            if match is None:
                raise ProofError(f"only top-level Base/local .bend imports allowed (no foreign/package imports): {where}")
            target, alias = match.groups()
            if target == "Base":
                continue
            imported = safe_file(root, path.parent / target)
            if imported in imports:
                raise ProofError(f"duplicate import: {where}")
            if alias is not None:
                if alias in aliases:
                    raise ProofError(f"duplicate import alias: {where}")
                aliases.add(alias)
            imports[imported] = alias
        if re.search(r"\blaw\b", line):
            match = LAW.fullmatch(line.rstrip())
            if match is None:
                raise ProofError(f"unsupported law declaration: {where}")
            name = match.group(1)
            if name in laws:
                raise ProofError(f"duplicate law {name}: {where}")
            laws.add(name)
        if re.search(r"\bdef\b", line):
            match = DEF.match(line)
            if match is None:
                raise ProofError(f"unsupported definition header: {where}")
            name = match.group(1)
            if name in definitions:
                raise ProofError(f"duplicate definition {name}: {where}")
            definitions.add(name)
    return Source(path, raw, imports, laws, definitions)


def discover_sources(root: Path) -> set[Path]:
    """Root entry files and maintained formal tree, not vendor/cache worktrees.

    Imports outside this seed set are also scanned recursively, within root.
    """
    files = {safe_file(root, path) for path in root.glob("*.bend")}
    formal = root / "tools" / "formal" / "models"
    for directory in (root / "tools", root / "tools" / "formal", formal):
        if directory.is_symlink():
            raise ProofError(f"symlink is not allowed: {directory}")
    if not formal.is_dir():
        raise ProofError("missing tools/formal/models directory")

    def walk_error(error: OSError) -> None:
        raise error

    for directory, subdirs, names in os.walk(formal, followlinks=False, onerror=walk_error):
        for name in subdirs:
            path = Path(directory) / name
            if path.is_symlink():
                raise ProofError(f"symlink directory is not allowed: {path}")
        for name in names:
            if name.endswith(".bend"):
                files.add(safe_file(root, Path(directory) / name))
    return files


def validate_pair(project: Project, directory: Path, *, component: bool) -> None:
    laws_path = directory / "LAWS.bend"
    proof_path = directory / "PROOF.bend"
    laws = project.sources.get(laws_path)
    proof = project.sources.get(proof_path)
    if laws is None or proof is None:
        raise ProofError(f"LAWS.bend requires a sibling PROOF.bend (and vice versa): {directory}")
    if laws_path not in proof.imports:
        raise ProofError(f"{proof_path.relative_to(project.root)} must directly import sibling LAWS.bend")
    if component and not laws.laws:
        raise ProofError(f"component has no law declarations: {laws_path}")
    if laws.laws:
        alias = proof.imports[laws_path]
        if alias is None:
            raise ProofError(f"law import must have an explicit alias: {proof_path}")
        missing = sorted(name for name in laws.laws if f"{alias}.{name}" not in proof.definitions)
        if missing:
            raise ProofError(f"missing law implementations in {proof_path.relative_to(project.root)}: {', '.join(missing)}")


def check_structure(root: Path) -> Project:
    if root.is_symlink():
        raise ProofError(f"project root may not be a symlink: {root}")
    root = root.resolve(strict=True)
    if not root.is_dir():
        raise ProofError(f"not a project directory: {root}")
    pending = discover_sources(root)
    sources: dict[Path, Source] = {}
    while pending:
        path = min(pending)
        pending.remove(path)
        if path in sources:
            continue
        source = read_source(root, path)
        sources[path] = source
        pending.update(set(source.imports) - sources.keys())
    formal = root / "tools" / "formal" / "models"
    components = sorted({path.parent for path in sources if path.name in ("LAWS.bend", "PROOF.bend") and path.is_relative_to(formal)})
    if not components:
        raise ProofError("no formal components discovered")
    project = Project(root, sources, components)
    validate_pair(project, root, component=False)
    for directory in components:
        validate_pair(project, directory, component=True)
        for filename in ("LAWS.bend", "PROOF.bend"):
            if directory / filename not in sources[root / filename].imports:
                raise ProofError(f"root {filename} missing direct import: {(directory / filename).relative_to(root)}")
    reachable: set[Path] = set()
    pending = {root / "PROOF.bend"}
    while pending:
        path = pending.pop()
        if path in reachable:
            continue
        reachable.add(path)
        pending.update(set(sources[path].imports) - reachable)
    orphaned = sorted(set(sources) - reachable)
    if orphaned:
        raise ProofError("sources not reachable from root PROOF.bend: " + ", ".join(str(p.relative_to(root)) for p in orphaned))
    for path, source in sources.items():
        if source.laws and path.name != "LAWS.bend":
            raise ProofError(f"law declarations must be in paired LAWS.bend files: {path}")
        if path.name == "LAWS.bend" and path != root / "LAWS.bend" and path.parent not in components:
            raise ProofError(f"component LAWS.bend must live under tools/formal/models: {path}")
    return project


def find_bend(explicit: str | None) -> str:
    selected = explicit if explicit is not None else os.environ.get("BEND")
    if selected is not None:
        if not selected:
            raise ProofError("BEND/--bend may not be empty")
        found = shutil.which(os.path.expanduser(selected))
        if found is None:
            raise ProofError(f"Bend executable not found: {selected}")
        return str(Path(found).resolve())
    found = shutil.which("bend")
    if found:
        return str(Path(found).resolve())
    fallback = Path.home() / ".bend" / "bin" / "bend"
    if fallback.is_file() and os.access(fallback, os.X_OK):
        return str(fallback)
    raise ProofError(f"Bend {PINNED_VERSION} not found; install it explicitly or use --bend PATH (nothing was fetched)")


def invoke(bend: str, args: list[str], root: Path, timeout: float, expected: str) -> None:
    env = os.environ.copy()
    # Bend otherwise phones home for daily update checks, even for check-only.
    env["BEND_NO_TELEMETRY"] = "1"
    result = subprocess.run([bend, *args], cwd=root, env=env, stdin=subprocess.DEVNULL,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            encoding="utf-8", errors="strict", timeout=timeout, check=False)
    if result.returncode != 0:
        raise ProofError(f"Bend failed ({result.returncode}) for {' '.join(args)}:\n{result.stdout}{result.stderr}")
    stderr = result.stderr.strip()
    if result.stdout.strip() != expected or (stderr and NOTICE.fullmatch(stderr) is None):
        raise ProofError(f"Bend did not report the exact required clean result {expected!r} for {' '.join(args)}:\n{result.stdout}{result.stderr}")


def prove(project: Project, bend: str, timeout: float) -> None:
    invoke(bend, ["version"], project.root, timeout, f"bend {PINNED_VERSION}")
    for target in project.targets:
        relative = str(target.relative_to(project.root))
        invoke(bend, [relative, "--check-only"], project.root, timeout, "All terms check.")
        print(f"prove: checked {relative}")
    # Detect ordinary edits/additions during the run; not an adversarial FS sandbox.
    after = check_structure(project.root)
    if {p: s.raw for p, s in project.sources.items()} != {p: s.raw for p, s in after.sources.items()}:
        raise ProofError("formal sources changed during checking; run again on a stable tree")


def positive_timeout(value: str) -> float:
    number = float(value)
    if not math.isfinite(number) or number <= 0:
        raise argparse.ArgumentTypeError("timeout must be a finite positive number")
    return number


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check-structure", action="store_true", help="offline lexical/import/law coverage checks only; not a proof check")
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2], help="project root (default: repository containing this script)")
    parser.add_argument("--bend", help=f"explicit executable; otherwise BEND, PATH, ~/.bend/bin/bend; exact {PINNED_VERSION} required")
    parser.add_argument("--timeout", type=positive_timeout, default=120.0, help="maximum seconds per Bend invocation (default: 120)")
    args = parser.parse_args(argv)
    try:
        project = check_structure(args.root.absolute())
        if args.check_structure:
            print(f"prove: structure ok ({len(project.components)} components, {len(project.sources)} sources); proofs NOT checked")
        else:
            prove(project, find_bend(args.bend), args.timeout)
            print(f"prove: ok ({len(project.targets)} proof entry points; Bend {PINNED_VERSION}; models and restricted-semantics contracts; not whole-Go verification)")
        return 0
    except (ProofError, OSError, UnicodeError, subprocess.TimeoutExpired) as exc:
        print(f"prove: FAIL: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
