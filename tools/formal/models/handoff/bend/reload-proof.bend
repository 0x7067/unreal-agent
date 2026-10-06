import Base
import ./generation.bend as G
import ./reload.bend as R

# Prove the transition for normalized independent metadata, then transport
# along the inductive equality. This is not runtime invariant checking.

def begin(+host: G.State) -> R.Inv(R.begin(R.Attempt{host, R.expected(R.phase(host)), True{}})):
  match host:
    case G.Host{live, cur, G.Idle{}, retired, False{}, status}: ({==}, {==})
    case G.Host{live, cur, G.Idle{}, retired, True{}, status}: ({==}, {==})
    case G.Host{live, cur, G.Staging{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Staged{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Fenced{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Awaiting{}, retired, failed, status}: ({==}, {==})

def probeTrue(+host: G.State) -> R.Inv(R.probe(R.Attempt{host, R.expected(R.phase(host)), True{}}, True{})):
  match host:
    case G.Host{live, cur, G.Idle{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Staging{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Staged{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Fenced{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Awaiting{}, retired, failed, status}: ({==}, {==})

def probeFalse(+host: G.State) -> R.Inv(R.probe(R.Attempt{host, R.expected(R.phase(host)), True{}}, False{})):
  match host:
    case G.Host{live, cur, G.Idle{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Staging{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Staged{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Fenced{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Awaiting{}, retired, failed, status}: ({==}, {==})

def fence(+host: G.State) -> R.Inv(R.fence(R.Attempt{host, R.expected(R.phase(host)), True{}})):
  match host:
    case G.Host{live, cur, G.Idle{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Staging{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Staged{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Fenced{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, cur, G.Awaiting{}, retired, failed, status}: ({==}, {==})

def quiesce(+host: G.State) -> R.Inv(R.quiesce(R.Attempt{host, R.expected(R.phase(host)), True{}})):
  match host:
    case G.Host{live, None{}, r, retired, failed, status}: ({==}, {==})
    case G.Host{live, Some{pid}, G.Idle{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, Some{pid}, G.Staging{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, Some{pid}, G.Staged{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, Some{pid}, G.Fenced{}, retired, failed, status}: ({==}, {==})
    case G.Host{live, Some{pid}, G.Awaiting{}, retired, failed, status}: ({==}, {==})

def abort(+host: G.State) -> R.Inv(R.abort(R.Attempt{host, R.expected(R.phase(host)), True{}})):
  match host:
    case G.Host{live, cur, r, retired, failed, status}: ({==}, {==})

def exit(+host: G.State) -> R.Inv(R.exit(R.Attempt{host, R.expected(R.phase(host)), True{}})):
  match host:
    case G.Host{live, cur, r, retired, failed, status}: ({==}, {==})

def launch(+host: G.State, +id: Nat) -> R.Inv(R.launch(R.Attempt{host, R.expected(R.phase(host)), True{}}, id)):
  match host:
    case G.Host{Nil{}, None{}, G.Awaiting{}, True{}, False{}, status}: ({==}, {==})
    case G.Host{Nil{}, None{}, G.Fenced{}, True{}, False{}, status}: ({==}, {==})
    case G.Host{x <> live, cur, r, retired, failed, status}: ({==}, {==})
    case G.Host{Nil{}, Some{pid}, r, retired, failed, status}: ({==}, {==})
    case G.Host{Nil{}, None{}, G.Idle{}, retired, failed, status}: ({==}, {==})
    case G.Host{Nil{}, None{}, G.Staging{}, retired, failed, status}: ({==}, {==})
    case G.Host{Nil{}, None{}, G.Staged{}, retired, failed, status}: ({==}, {==})
    case G.Host{Nil{}, None{}, G.Fenced{}, False{}, failed, status}: ({==}, {==})
    case G.Host{Nil{}, None{}, G.Awaiting{}, False{}, failed, status}: ({==}, {==})
    case G.Host{Nil{}, None{}, G.Fenced{}, True{}, True{}, status}: ({==}, {==})
    case G.Host{Nil{}, None{}, G.Awaiting{}, True{}, True{}, status}: ({==}, {==})

def normalized(+host: G.State, e: G.Event) -> R.Inv(R.step(R.Attempt{host, R.expected(R.phase(host)), True{}}, e)):
  match e:
    case G.Begin{}: begin(host)
    case G.ProbeOK{}: probeTrue(host)
    case G.ProbeBad{}: probeFalse(host)
    case G.Fence{}: fence(host)
    case G.Quiesce{}: quiesce(host)
    case G.Abort{}: abort(host)
    case G.Exit{}: exit(host)
    case G.Launch{id}: launch(host, id)

def step(+s: R.State, +e: G.Event, inv: R.Inv(s)) -> R.Inv(R.step(s, e)):
  match s:
    case R.Attempt{host, meta, audit}:
      (eq, hist) = inv
      %Equal.sym(R.Meta, meta, R.expected(R.phase(host)), eq) : R.Inv(R.step(R.Attempt{host, _, audit}, e))
      %Equal.sym(Bool, audit, True{}, hist) : R.Inv(R.step(R.Attempt{host, R.expected(R.phase(host)), _}, e))
      normalized(host, e)

def run(+es: List<&2, G.Event>, +s: R.State, inv: R.Inv(s)) -> R.Inv(R.run(es, s)):
  match es:
    case Nil{}: inv
    case e <> rest: run(rest, R.step(s, e), step(s, e, inv))

def launchHistory(+s: R.State, inv: R.Inv(s)) -> {R.launchSafe(s) == True{} : Bool}:
  match s:
    case R.Attempt{host, meta, audit}:
      (eq, hist) = inv
      hist

def singleFlightExpected(+host: G.State) -> {R.singleFlight(R.Attempt{host, R.expected(R.phase(host)), True{}}) == True{} : Bool}:
  match host:
    case G.Host{live, cur, G.Idle{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Staging{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Staged{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Fenced{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Awaiting{}, retired, failed, status}: {==}

def singleFlight(+s: R.State, inv: R.Inv(s)) -> {R.singleFlight(s) == True{} : Bool}:
  match s:
    case R.Attempt{host, meta, audit}:
      (eq, hist) = inv
      %Equal.sym(R.Meta, meta, R.expected(R.phase(host)), eq) : {R.singleFlight(R.Attempt{host, _, audit}) == True{} : Bool}
      %Equal.sym(Bool, audit, True{}, hist) : {R.singleFlight(R.Attempt{host, R.expected(R.phase(host)), _}) == True{} : Bool}
      singleFlightExpected(host)

def artifactSafeExpected(+host: G.State) -> {R.artifactSafe(R.Attempt{host, R.expected(R.phase(host)), True{}}) == True{} : Bool}:
  match host:
    case G.Host{live, cur, G.Idle{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Staging{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Staged{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Fenced{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Awaiting{}, retired, failed, status}: {==}

def artifactSafe(+s: R.State, inv: R.Inv(s)) -> {R.artifactSafe(s) == True{} : Bool}:
  match s:
    case R.Attempt{host, meta, audit}:
      (eq, hist) = inv
      %Equal.sym(R.Meta, meta, R.expected(R.phase(host)), eq) : {R.artifactSafe(R.Attempt{host, _, audit}) == True{} : Bool}
      %Equal.sym(Bool, audit, True{}, hist) : {R.artifactSafe(R.Attempt{host, R.expected(R.phase(host)), _}) == True{} : Bool}
      artifactSafeExpected(host)

def probeSafeExpected(+host: G.State) -> {R.probeSafe(R.Attempt{host, R.expected(R.phase(host)), True{}}) == True{} : Bool}:
  match host:
    case G.Host{live, cur, G.Idle{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Staging{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Staged{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Fenced{}, retired, failed, status}: {==}
    case G.Host{live, cur, G.Awaiting{}, retired, failed, status}: {==}

def probeSafe(+s: R.State, inv: R.Inv(s)) -> {R.probeSafe(s) == True{} : Bool}:
  match s:
    case R.Attempt{host, meta, audit}:
      (eq, hist) = inv
      %Equal.sym(R.Meta, meta, R.expected(R.phase(host)), eq) : {R.probeSafe(R.Attempt{host, _, audit}) == True{} : Bool}
      %Equal.sym(Bool, audit, True{}, hist) : {R.probeSafe(R.Attempt{host, R.expected(R.phase(host)), _}) == True{} : Bool}
      probeSafeExpected(host)
