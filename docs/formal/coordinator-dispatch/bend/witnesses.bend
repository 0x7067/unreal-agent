# Generated finite execution witnesses; all computations rechecked in Bend.
import Base
import ./product.bend as P

def skipCommitConfig() -> P.Config: P.Config{True{}, False{}, False{}}
def skipCommitEvents() -> List<&2, P.Boundary>:
  [P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}]

def skipTerminalConfig() -> P.Config: P.Config{False{}, True{}, False{}}
def skipTerminalEvents() -> List<&2, P.Boundary>:
  [P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.UpdateOneTerminal{}, P.Tick{}, P.Crash{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}]

def skipOverlayConfig() -> P.Config: P.Config{False{}, False{}, True{}}
def skipOverlayEvents() -> List<&2, P.Boundary>:
  [P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.UpdateOneAwaiting{}, P.Tick{}, P.Crash{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}]

def twoOperationRecoveryEvents() -> List<&2, P.Boundary>:
  [P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.UpdateOneAwaiting{}, P.Tick{}, P.Tick{}, P.UpdateTwoTerminal{}, P.Tick{}, P.Tick{}, P.Crash{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}]
def twoOperationRecoveryExpected() -> P.Core:
  P.Core{P.Idle{}, P.End{}, True{}, P.Snapshots{P.Awaiting{}, P.Terminal{}}, P.Snapshots{P.Awaiting{}, P.Terminal{}}, True{}, P.Flags{True{}, False{}}, False{}, False{}}

def visibleStatusRecoveryEvents() -> List<&2, P.Boundary>:
  [P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.StatusVisible{}, P.Crash{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}]
def visibleStatusRecoveryExpected() -> P.Core:
  P.Core{P.Guard{}, P.One{}, True{}, P.Snapshots{P.Ready{}, P.Ready{}}, P.Snapshots{P.Ready{}, P.Ready{}}, True{}, P.Flags{False{}, False{}}, False{}, False{}}

def invisibleStatusRecoveryEvents() -> List<&2, P.Boundary>:
  [P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.StatusInvisible{}, P.Crash{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}]
def invisibleStatusRecoveryExpected() -> P.Core:
  P.Core{P.Translate{}, P.One{}, False{}, P.Snapshots{P.Absent{}, P.Absent{}}, P.Snapshots{P.Absent{}, P.Absent{}}, False{}, P.Flags{False{}, False{}}, False{}, False{}}

def visibleUpdateRecoveryEvents() -> List<&2, P.Boundary>:
  [P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.UpdateTwoAwaiting{}, P.UpdateVisible{}, P.Crash{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}]
def visibleUpdateRecoveryExpected() -> P.Core:
  P.Core{P.Guard{}, P.One{}, True{}, P.Snapshots{P.Ready{}, P.Awaiting{}}, P.Snapshots{P.Ready{}, P.Awaiting{}}, True{}, P.Flags{False{}, False{}}, False{}, False{}}

def invisibleUpdateRecoveryEvents() -> List<&2, P.Boundary>:
  [P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.UpdateTwoAwaiting{}, P.UpdateInvisible{}, P.Crash{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}]
def invisibleUpdateRecoveryExpected() -> P.Core:
  P.Core{P.Guard{}, P.One{}, True{}, P.Snapshots{P.Ready{}, P.Ready{}}, P.Snapshots{P.Ready{}, P.Ready{}}, True{}, P.Flags{False{}, False{}}, False{}, False{}}

def partialOverlayCrashEvents() -> List<&2, P.Boundary>:
  [P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.UpdateTwoTerminal{}, P.Tick{}, P.Tick{}, P.Crash{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Crash{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}, P.Tick{}]
def partialOverlayCrashExpected() -> P.Core:
  P.Core{P.Guard{}, P.One{}, True{}, P.Snapshots{P.Ready{}, P.Terminal{}}, P.Snapshots{P.Ready{}, P.Terminal{}}, True{}, P.Flags{False{}, False{}}, False{}, False{}}
