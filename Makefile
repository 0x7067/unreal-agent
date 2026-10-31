.PHONY: build test check install-local review-canary review-canary-live review-prove review-lint install-review-lint

GOLANGCI_LINT ?= golangci-lint
GOLANGCI_LINT_VERSION ?= v2.13.2
BEND ?= bend

# Local tap for install-local. The generated cask's download URLs point at
# this fork's GitHub releases; install-local rewrites them to local archives.
LOCAL_TAP ?= 0x7067/local

build:
	go build -trimpath -o bin/unreal-agent-runner ./cmd/unreal-agent-runner
	go -C cmd/unreal-agent-tui build -trimpath -o ../../bin/unreal-agent-tui .
	go build -trimpath -o bin/unreal-review ./cmd/unreal-review

test:
	go test -race ./...
	go -C cmd/unreal-agent-tui test -race ./...

check:
	go vet ./...
	go -C cmd/unreal-agent-tui vet ./...
	@test -z "$$(gofmt -l cmd harness internal review)" || { gofmt -l cmd harness internal review; exit 1; }

# Offline review CLI regression. The stub answers 401 on loopback; this target
# does not call openrouter.ai. review-canary-live does, and is not a CI gate.
review-canary: build
	bash tools/review-canary/canary.sh
	UNREAL_REVIEW_BIN=$(CURDIR)/bin/unreal-review go test -tags canary -count=1 -timeout 180s ./cmd/unreal-review -run '^TestCanary'

review-canary-live: build
	bash tools/review-canary/canary-live.sh

review-prove:
	BEND=$(BEND) sh docs/formal/review/prove.sh

review-lint:
	$(GOLANGCI_LINT) run ./cmd/internal/review/... ./cmd/unreal-review/... ./review/...

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
