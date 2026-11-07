.PHONY: build test check install-local review-canary review-canary-live review-prove review-lint install-review-lint prove proof-check proof-test proof-coverage prove-complete install-bend proof-source proof-report proof-regressions

GOLANGCI_LINT ?= golangci-lint
GOLANGCI_LINT_VERSION ?= v2.13.2
# Prefer PATH; installer keeps the pinned compiler under ~/.bend by default.
BEND ?= $(shell command -v bend 2>/dev/null || printf '%s' '$(HOME)/.bend/bin/bend')

# Local tap for install-local. The generated cask's download URLs point at
# this fork's GitHub releases; install-local rewrites them to local archives.
LOCAL_TAP ?= 0x7067/local

build:
	go build -trimpath -o bin/unreal-agent-runner ./cmd/unreal-agent-runner
	go -C cmd/unreal-agent-tui build -trimpath -o ../../bin/unreal-agent-tui .

test:
	go test -race ./...
	go -C cmd/unreal-agent-tui test -race ./...

check: proof-check
	go vet ./...
	go -C cmd/unreal-agent-tui vet ./...
	@test -z "$$(gofmt -l cmd harness internal review tools)" || { gofmt -l cmd harness internal review tools; exit 1; }

# Offline review command regression. The stub answers 401 on loopback; this target
# does not call openrouter.ai. review-canary-live does, and is not a CI gate.
review-canary: build
	bash tools/review-canary/canary.sh
	UNREAL_REVIEW_BIN=$(CURDIR)/bin/unreal-agent-runner go test -tags canary -count=1 -timeout 180s ./cmd/internal/reviewcli -run '^TestCanary'

review-canary-live: build
	bash tools/review-canary/canary-live.sh

# Bend 2 is authoritative; models/restricted source contracts are not whole-Go refinement.
install-bend:
	sh docs/formal/review/install-bend.sh

proof-coverage:
	python3 docs/formal/check_coverage.py

# Source freshness uses the installed local Go parser; no downloading/toolchain upgrade.
proof-source:
	python3 tools/formal/gosource/check.py
	python3 tools/formal/apienums/check.py
	python3 docs/formal/coordinator-dispatch/bend/generate_certificate.py --check
	python3 docs/formal/operations/bend/generate_certificate.py --check

proof-check: proof-coverage proof-source
	python3 tools/formal/prove.py --check-structure

proof-test:
	python3 -m unittest discover -s tools/formal -p 'test_*.py'
	python3 -m unittest discover -s docs/formal -p 'test_*.py'
	python3 tools/formal/gosource/test.py
	python3 tools/formal/apienums/test.py

proof-regressions:
	BEND="$(BEND)" BEND_NO_TELEMETRY=1 bash docs/formal/coordinator-dispatch/bend/check.sh
	BEND="$(BEND)" BEND_NO_TELEMETRY=1 bash docs/formal/operations/bend/check.sh

proof-report:
	python3 tools/formal/completion.py

prove: proof-check
	BEND="$(BEND)" python3 tools/formal/verify.py

# Completion requires connected implementation proofs, not a successful model gate.
# Report actual remaining named declarations and semantic/environment obligations.
prove-complete: prove
	python3 tools/formal/completion.py --require-complete

review-prove:
	BEND="$(BEND)" sh docs/formal/review/prove.sh

review-lint:
	$(GOLANGCI_LINT) run ./cmd/internal/review/... ./cmd/internal/reviewcli/... ./review/...

install-review-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# Build a snapshot release and install it as the unreal-agent Homebrew cask,
# with the cask's download URLs pointed at the local archives.
install-local:
	goreleaser release --snapshot --clean --skip=docker
	test -d "$$(brew --repository $(LOCAL_TAP))" || brew tap-new --no-git $(LOCAL_TAP)
	tap="$$(brew --repository $(LOCAL_TAP))" && mkdir -p "$$tap/Casks" && \
		sed -E 's|https://github.com/0x7067/unreal-agent/releases/download/[^/]+/|file://$(CURDIR)/dist/|' \
			dist/homebrew/Casks/unreal-agent.rb > "$$tap/Casks/unreal-agent.rb"
	if brew list --cask $(LOCAL_TAP)/unreal-agent >/dev/null 2>&1; then \
		brew reinstall --cask $(LOCAL_TAP)/unreal-agent; \
	else \
		brew install --cask $(LOCAL_TAP)/unreal-agent; \
	fi
