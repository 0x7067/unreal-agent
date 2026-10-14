#!/usr/bin/env bash
# Drive bin/unreal-agent-tui in an isolated tmux session against the fake model.
# Usage: tui.sh start RUN_DIR [TOOL_CALLS] [MESSAGES] [COLS] [ROWS]
#        tui.sh doctor RUN_DIR
#        tui.sh keys RUN_DIR KEY...          (tmux send-keys syntax)
#        tui.sh click RUN_DIR COL ROW        (1-based, left button, SGR mouse)
#        tui.sh wheel RUN_DIR COL ROW up|down
#        tui.sh snap RUN_DIR NAME            (writes RUN_DIR/evidence/NAME.txt)
#        tui.sh stop RUN_DIR
set -euo pipefail
cmd=$1 run=$2
shift 2
session="tui-$(basename "$run")"
repo=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)

case $cmd in
start)
  calls=${1:-120} messages=${2:-3} cols=${3:-160} rows=${4:-50}
  mkdir -p "$run"/{workspace,state,config,evidence}
  port=$(python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])')
  python3 -I "$repo/.claude/skills/verify-unreal-agent-tui/scripts/fake_model.py" "$port" "$calls" "$messages" &
  echo $! >"$run/fake.pid"
  echo "$port" >"$run/port"
  tmux new-session -d -s "$session" -x "$cols" -y "$rows" -c "$run/workspace" \
    "env XDG_STATE_HOME=$run/state XDG_CONFIG_HOME=$run/config OPENAI_API_KEY=fake \
     $repo/bin/unreal-agent-tui -provider openai -model fake -base-url http://127.0.0.1:$port 2>$run/stderr.txt"
  for _ in $(seq 50); do
    tmux capture-pane -p -t "$session" | grep -q "Send a message" && exit 0
    sleep 0.2
  done
  echo "TUI not ready; see $run/stderr.txt" >&2
  exit 1
  ;;
doctor)
  tmux has-session -t "$session" && echo "tmux session $session: up"
  kill -0 "$(cat "$run/fake.pid")" && echo "fake model pid $(cat "$run/fake.pid") on port $(cat "$run/port"): up"
  echo "binary: $(git -C "$repo" rev-parse --short HEAD) $(stat -f %Sm "$repo/bin/unreal-agent-tui")"
  ;;
keys) tmux send-keys -t "$session" "$@" ;;
click)
  tmux send-keys -t "$session" -l $'\e[<0;'"$1;$2"'M'
  tmux send-keys -t "$session" -l $'\e[<0;'"$1;$2"'m'
  ;;
wheel)
  button=64
  [ "$3" = down ] && button=65
  tmux send-keys -t "$session" -l $'\e[<'"$button;$1;$2"'M'
  ;;
snap) tmux capture-pane -p -t "$session" >"$run/evidence/$1.txt" && cat "$run/evidence/$1.txt" ;;
stop)
  tmux kill-session -t "$session" 2>/dev/null || true
  kill "$(cat "$run/fake.pid")" 2>/dev/null || true
  rm -rf "$run"/{workspace,state,config}
  ;;
*) echo "unknown command $cmd" >&2; exit 2 ;;
esac
