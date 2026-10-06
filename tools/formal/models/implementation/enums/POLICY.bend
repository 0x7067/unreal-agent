# Independent reviewed policy. NOT read or rewritten by the extractor/generator.
# Constants are specified literal byte lists, not imported source definitions.
import Base
import ./semantics.bend as M

def completed() -> List<&2, Nat>:
  [99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]

def failed() -> List<&2, Nat>:
  [102n, 97n, 105n, 108n, 101n, 100n]

def canceled() -> List<&2, Nat>:
  [99n, 97n, 110n, 99n, 101n, 108n, 101n, 100n]

def ready() -> List<&2, Nat>:
  [114n, 101n, 97n, 100n, 121n]

def awaiting() -> List<&2, Nat>:
  [97n, 119n, 97n, 105n, 116n, 105n, 110n, 103n]

def canceling() -> List<&2, Nat>:
  [99n, 97n, 110n, 99n, 101n, 108n, 105n, 110n, 103n]

def unsupported() -> List<&2, Nat>:
  []

def io_create() -> List<&2, Nat>:
  [105n, 111n, 46n, 99n, 114n, 101n, 97n, 116n, 101n, 95n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]

def io_read() -> List<&2, Nat>:
  [105n, 111n, 46n, 114n, 101n, 97n, 100n, 95n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]

def process_exit() -> List<&2, Nat>:
  [112n, 114n, 111n, 99n, 101n, 115n, 115n, 46n, 101n, 120n, 105n, 116n, 101n, 100n]

def primitive_failed() -> List<&2, Nat>:
  [112n, 114n, 105n, 109n, 105n, 116n, 105n, 118n, 101n, 46n, 102n, 97n, 105n, 108n, 101n, 100n]

def primitive_canceled() -> List<&2, Nat>:
  [112n, 114n, 105n, 109n, 105n, 116n, 105n, 118n, 101n, 46n, 99n, 97n, 110n, 99n, 101n, 108n, 101n, 100n]

def compute() -> List<&2, Nat>:
  [99n, 111n, 109n, 112n, 117n, 116n, 101n, 46n, 99n, 111n, 109n, 112n, 108n, 101n, 116n, 101n, 100n]

def read_out() -> List<&2, Nat>:
  [114n, 101n, 97n, 100n, 95n, 111n, 117n, 116n]

def read_out_tail() -> List<&2, Nat>:
  [114n, 101n, 97n, 100n, 95n, 111n, 117n, 116n, 95n, 116n, 97n, 105n, 108n]

def read_err() -> List<&2, Nat>:
  [114n, 101n, 97n, 100n, 95n, 101n, 114n, 114n]

def read_err_tail() -> List<&2, Nat>:
  [114n, 101n, 97n, 100n, 95n, 101n, 114n, 114n, 95n, 116n, 97n, 105n, 108n]

def low() -> List<&2, Nat>:
  [108n, 111n, 119n]

def medium() -> List<&2, Nat>:
  [109n, 101n, 100n, 105n, 117n, 109n]

def high() -> List<&2, Nat>:
  [104n, 105n, 103n, 104n]

def xhigh() -> List<&2, Nat>:
  [120n, 104n, 105n, 103n, 104n]

def max() -> List<&2, Nat>:
  [109n, 97n, 120n]

# The differing empty/unsupported treatment is intentional, not unified away.
def terminal3(+x: List<&2, Nat>) -> Bool:
  M.byteEq(x, completed()) || (M.byteEq(x, failed()) || M.byteEq(x, canceled()))

def terminal4(+x: List<&2, Nat>) -> Bool:
  ((M.byteEq(x, completed()) || M.byteEq(x, failed())) || M.byteEq(x, canceled())) || M.byteEq(x, unsupported())

def primitive_allowed(+x: List<&2, Nat>) -> Bool:
  M.byteEq(x, io_create()) || (M.byteEq(x, io_read()) || (M.byteEq(x, process_exit()) || (M.byteEq(x, primitive_failed()) || (M.byteEq(x, primitive_canceled()) || M.byteEq(x, compute())))))

def shell_allowed(+x: List<&2, Nat>) -> Bool:
  M.byteEq(x, read_out()) || (M.byteEq(x, read_out_tail()) || (M.byteEq(x, read_err()) || M.byteEq(x, read_err_tail())))

def reasoning_allowed(+x: List<&2, Nat>) -> Bool:
  M.byteEq(x, low()) || (M.byteEq(x, medium()) || (M.byteEq(x, high()) || (M.byteEq(x, xhigh()) || M.byteEq(x, max()))))

# Ready/awaiting may progress to any live-or-terminal update; canceling may
# only stay canceling or fail/cancel. Other current states reject all updates.
# This is a declared priority-guard expression, independent of source AST.
def live_update(+y: List<&2, Nat>) -> Bool:
  (((M.byteEq(y, awaiting()) || M.byteEq(y, canceling())) || M.byteEq(y, completed())) || M.byteEq(y, failed())) || M.byteEq(y, canceled())

def cancel_update(+y: List<&2, Nat>) -> Bool:
  (M.byteEq(y, canceling()) || M.byteEq(y, failed())) || M.byteEq(y, canceled())

def remote_allowed(+x: List<&2, Nat>, +y: List<&2, Nat>) -> Bool:
  M.choose(M.byteEq(x, ready()) || M.byteEq(x, awaiting()), live_update(y), M.choose(M.byteEq(x, canceling()), cancel_update(y), False{}))
