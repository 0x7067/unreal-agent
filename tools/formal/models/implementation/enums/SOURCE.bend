# Generated from actual restricted Go AST. DO NOT EDIT.
# Explicit refresh changes SOURCE ONLY, never the independent policy.
import Base
import ./semantics.bend as M

# harness/operation/local_manager.go: localOperationFinished; inputs ['status']
def local_finished_ast() -> M.Expr:
  M.Branch{M.Either{M.Same{M.First{}, [99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]}, M.Either{M.Same{M.First{}, [102n, 97n, 105n, 108n, 101n, 100n]}, M.Same{M.First{}, [99n, 97n, 110n, 99n, 101n, 108n, 101n, 100n]}}}, M.Truth{True{}}, M.Truth{False{}}}

def local_finished(+x: List<&2, Nat>) -> Bool:
  M.eval(local_finished_ast(), x, [])

# harness/operation/local_manager.go: localPrimitiveCompleted; inputs ['eventType']
def primitive_completed_ast() -> M.Expr:
  M.Branch{M.Either{M.Same{M.First{}, [105n, 111n, 46n, 99n, 114n, 101n, 97n, 116n, 101n, 95n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]}, M.Either{M.Same{M.First{}, [105n, 111n, 46n, 114n, 101n, 97n, 100n, 95n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]}, M.Either{M.Same{M.First{}, [112n, 114n, 111n, 99n, 101n, 115n, 115n, 46n, 101n, 120n, 105n, 116n, 101n, 100n]}, M.Either{M.Same{M.First{}, [112n, 114n, 105n, 109n, 105n, 116n, 105n, 118n, 101n, 46n, 102n, 97n, 105n, 108n, 101n, 100n]}, M.Either{M.Same{M.First{}, [112n, 114n, 105n, 109n, 105n, 116n, 105n, 118n, 101n, 46n, 99n, 97n, 110n, 99n, 101n, 108n, 101n, 100n]}, M.Same{M.First{}, [99n, 111n, 109n, 112n, 117n, 116n, 101n, 46n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]}}}}}}, M.Truth{True{}}, M.Truth{False{}}}

def primitive_completed(+x: List<&2, Nat>) -> Bool:
  M.eval(primitive_completed_ast(), x, [])

# harness/operation/remote_job_handler.go: validRemoteJobStatusTransition; inputs ['current', 'updated']
def remote_transition_ast() -> M.Expr:
  M.Branch{M.Same{M.First{}, [114n, 101n, 97n, 100n, 121n]}, M.Either{M.Either{M.Either{M.Either{M.Same{M.Second{}, [97n, 119n, 97n, 105n, 116n, 105n, 110n, 103n]}, M.Same{M.Second{}, [99n, 97n, 110n, 99n, 101n, 108n, 105n, 110n, 103n]}}, M.Same{M.Second{}, [99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]}}, M.Same{M.Second{}, [102n, 97n, 105n, 108n, 101n, 100n]}}, M.Same{M.Second{}, [99n, 97n, 110n, 99n, 101n, 108n, 101n, 100n]}}, M.Branch{M.Same{M.First{}, [97n, 119n, 97n, 105n, 116n, 105n, 110n, 103n]}, M.Either{M.Either{M.Either{M.Either{M.Same{M.Second{}, [97n, 119n, 97n, 105n, 116n, 105n, 110n, 103n]}, M.Same{M.Second{}, [99n, 97n, 110n, 99n, 101n, 108n, 105n, 110n, 103n]}}, M.Same{M.Second{}, [99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]}}, M.Same{M.Second{}, [102n, 97n, 105n, 108n, 101n, 100n]}}, M.Same{M.Second{}, [99n, 97n, 110n, 99n, 101n, 108n, 101n, 100n]}}, M.Branch{M.Same{M.First{}, [99n, 97n, 110n, 99n, 101n, 108n, 105n, 110n, 103n]}, M.Either{M.Either{M.Same{M.Second{}, [99n, 97n, 110n, 99n, 101n, 108n, 105n, 110n, 103n]}, M.Same{M.Second{}, [102n, 97n, 105n, 108n, 101n, 100n]}}, M.Same{M.Second{}, [99n, 97n, 110n, 99n, 101n, 108n, 101n, 100n]}}, M.Truth{False{}}}}}

def remote_transition(+x: List<&2, Nat>, +y: List<&2, Nat>) -> Bool:
  M.eval(remote_transition_ast(), x, y)

# harness/coordinator/loop.go: operationIsTerminal; inputs ['status']
def coordinator_terminal_ast() -> M.Expr:
  M.Branch{M.Either{M.Same{M.First{}, [99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]}, M.Either{M.Same{M.First{}, [102n, 97n, 105n, 108n, 101n, 100n]}, M.Same{M.First{}, [99n, 97n, 110n, 99n, 101n, 108n, 101n, 100n]}}}, M.Truth{True{}}, M.Truth{False{}}}

def coordinator_terminal(+x: List<&2, Nat>) -> Bool:
  M.eval(coordinator_terminal_ast(), x, [])

# harness/sessionstore/localfile/state.go: terminalOperationStatus; inputs ['status']
def stored_terminal_ast() -> M.Expr:
  M.Branch{M.Either{M.Same{M.First{}, [99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]}, M.Either{M.Same{M.First{}, [102n, 97n, 105n, 108n, 101n, 100n]}, M.Same{M.First{}, [99n, 97n, 110n, 99n, 101n, 108n, 101n, 100n]}}}, M.Truth{True{}}, M.Truth{False{}}}

def stored_terminal(+x: List<&2, Nat>) -> Bool:
  M.eval(stored_terminal_ast(), x, [])

# harness/llm/model.go: Valid; inputs ['effort']
def reasoning_valid_ast() -> M.Expr:
  M.Branch{M.Either{M.Same{M.First{}, [108n, 111n, 119n]}, M.Either{M.Same{M.First{}, [109n, 101n, 100n, 105n, 117n, 109n]}, M.Either{M.Same{M.First{}, [104n, 105n, 103n, 104n]}, M.Either{M.Same{M.First{}, [120n, 104n, 105n, 103n, 104n]}, M.Same{M.First{}, [109n, 97n, 120n]}}}}}, M.Truth{True{}}, M.Truth{False{}}}

def reasoning_valid(+x: List<&2, Nat>) -> Bool:
  M.eval(reasoning_valid_ast(), x, [])

