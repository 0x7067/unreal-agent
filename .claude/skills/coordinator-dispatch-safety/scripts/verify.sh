#!/usr/bin/env bash
# Run from any directory. --self-test also compiles temporary broken real code.
set -euo pipefail
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
root=$(cd -- "$here/../../../.." && pwd)
cd "$root"
case "${1:-}" in
  '') ;;
  --self-test) ;;
  *) echo "usage: $0 [--self-test]" >&2; exit 2 ;;
esac
if (( $# > 1 )); then echo "usage: $0 [--self-test]" >&2; exit 2; fi
"$root/docs/formal/coordinator-dispatch/check.sh"
checks='TestDispatchExhaustiveRecovery|TestDispatchCommitBeforeAdd|TestCoordinatorRetriesInitialToolStatusPersistenceFailure|TestCoordinatorResumesPartiallyCompletedToolCall'
go test ./harness/coordinator -run "^($checks)$" -count=1
if [[ "${1:-}" == --self-test ]]; then
  python3 "$here/mutations.py"
  # Re-run the unchanged baseline after all scratch changes are discarded.
  go test ./harness/coordinator -run "^($checks)$" -count=1
fi
