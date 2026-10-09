.PHONY: build test check install-local

# Local tap for install-local; the cask mirrors unreallabsai/tap/unreal-agent.
LOCAL_TAP ?= 0x7067/local

build:
	go build -trimpath -o bin/unreal-agent-runner ./cmd/unreal-agent-runner
	go -C cmd/unreal-agent-tui build -trimpath -o ../../bin/unreal-agent-tui .

test:
	go test -race ./...
	go -C cmd/unreal-agent-tui test -race ./...

check:
	go vet ./...
	go -C cmd/unreal-agent-tui vet ./...
	@test -z "$$(gofmt -l cmd harness internal)" || { gofmt -l cmd harness internal; exit 1; }

# Build a snapshot release and install it as the unreal-agent Homebrew cask,
# with the cask's download URLs pointed at the local archives.
install-local:
	goreleaser release --snapshot --clean --skip=docker
	test -d "$$(brew --repository $(LOCAL_TAP))" || brew tap-new --no-git $(LOCAL_TAP)
	tap="$$(brew --repository $(LOCAL_TAP))" && mkdir -p "$$tap/Casks" && \
		sed -E 's|https://github.com/unreallabsai/unreal-agent/releases/download/v[^/]+/|file://$(CURDIR)/dist/|' \
			dist/homebrew/Casks/unreal-agent.rb > "$$tap/Casks/unreal-agent.rb"
	if brew list --cask $(LOCAL_TAP)/unreal-agent >/dev/null 2>&1; then \
		brew reinstall --cask $(LOCAL_TAP)/unreal-agent; \
	else \
		brew install --cask $(LOCAL_TAP)/unreal-agent; \
	fi
