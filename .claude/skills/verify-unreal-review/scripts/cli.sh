#!/bin/bash
set -u

. "$(dirname "$0")/lib.sh"
require_launch

name=""
no_github=0
stdin_file=""
while [ $# -gt 0 ]; do
	case "$1" in
	--name)
		name=$2
		shift 2
		;;
	--no-github-auth)
		no_github=1
		shift
		;;
	--stdin)
		stdin_file=$2
		shift 2
		;;
	--)
		shift
		break
		;;
	-*)
		echo "usage: cli.sh --name <step> [--no-github-auth] [--stdin file] -- <unreal-review args>" >&2
		exit 2
		;;
	*)
		break
		;;
	esac
done

if [ -z "$name" ]; then
	echo "cli.sh: --name is required" >&2
	exit 2
fi
case "$name" in
*[!A-Za-z0-9._-]*)
	echo "cli.sh: --name must be a single path segment of letters, digits, dot, underscore, or hyphen" >&2
	exit 2
	;;
esac

step="$VERIFY_EVIDENCE/$name"
mkdir -p "$step"

bin=$VERIFY_BIN

{
	echo "bin=$bin"
	echo "cwd=$(pwd)"
	echo "no_github_auth=$no_github"
	if [ -n "$stdin_file" ]; then
		echo "stdin=$stdin_file"
	fi
	printf 'args='
	printf '%q ' "$bin" "$@"
	echo
} >"$step/cmd.txt"

set +e
# Pin OpenRouter and drop the generic key and base URL. Recipes still supply
# OPENROUTER_API_KEY. The loopback stub applies only under canary.sh or when
# UNREAL_REVIEW_OPENROUTER_API is exported.
# Every -u precedes the assignment: env treats a later -u as the command.
review_env=(env)
review_env+=(-u UNREAL_HARNESS_LLM_PROVIDER)
review_env+=(-u UNREAL_HARNESS_LLM_API_KEY)
review_env+=(-u UNREAL_HARNESS_LLM_BASE_URL)
if [ "$no_github" -eq 1 ]; then
	review_env+=(-u GH_TOKEN)
	review_env+=(-u GITHUB_TOKEN)
fi
review_env+=(UNREAL_HARNESS_LLM_PROVIDER=openrouter)
if [ -n "$stdin_file" ]; then
	"${review_env[@]}" "$bin" "$@" <"$stdin_file" >"$step/stdout.txt" 2>"$step/stderr.txt"
else
	"${review_env[@]}" "$bin" "$@" >"$step/stdout.txt" 2>"$step/stderr.txt"
fi
status=$?
set -e
echo "$status" >"$step/exit.txt"
echo "$name exit=$status evidence=$step"
exit 0
