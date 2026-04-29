.PHONY: all lint lintmax test build gosec govulncheck goreleaser install docker-lint golangci-lint-install tag-major tag-minor tag-patch release bump-deps

all: build

VERSION ?= v0.1.0
GORELEASER_ARGS ?= --skip=sign --snapshot --clean
GORELEASER_TARGET ?= --single-target
GOLANGCI_LINT_VERSION ?= $(shell cat .golangci-lint-version)
GOLANGCI_LINT_BIN ?= $(CURDIR)/.bin/golangci-lint
GOLANGCI_LINT_ARGS ?= --timeout=5m ./cmd/... ./internal/...
LINT_DIRS := $(shell git ls-files '*.go' | grep -vE '(^|/)ttmp/|(^|/)testdata/' | xargs -r -n1 dirname | sed 's#^#./#' | sort -u)
GOSEC_EXCLUDE_DIRS := -exclude-dir=.history -exclude-dir=testdata -exclude-dir=ttmp
BINARY ?= chat

$(GOLANGCI_LINT_BIN): .golangci-lint-version
	mkdir -p $(dir $(GOLANGCI_LINT_BIN))
	GOBIN=$(dir $(GOLANGCI_LINT_BIN)) GOWORK=off go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

golangci-lint-install: $(GOLANGCI_LINT_BIN)

docker-lint:
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) sh -c "golangci-lint config verify && golangci-lint run -v $(GOLANGCI_LINT_ARGS)"

lint: golangci-lint-install
	GOWORK=off $(GOLANGCI_LINT_BIN) config verify
	GOWORK=off $(GOLANGCI_LINT_BIN) run -v $(GOLANGCI_LINT_ARGS)

lintmax: golangci-lint-install
	GOWORK=off $(GOLANGCI_LINT_BIN) config verify
	GOWORK=off $(GOLANGCI_LINT_BIN) run -v --max-same-issues=100 $(GOLANGCI_LINT_ARGS)

gosec:
	GOWORK=off go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -exclude-generated -exclude=G101,G304,G301,G306,G204 $(GOSEC_EXCLUDE_DIRS) $(LINT_DIRS)

govulncheck:
	GOWORK=off go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

test:
	GOWORK=off go test ./...

build:
	GOWORK=off go generate ./...
	GOWORK=off go build ./...

goreleaser:
	GOWORK=off goreleaser release $(GORELEASER_ARGS) $(GORELEASER_TARGET)

tag-major:
	git tag $(shell svu major)

tag-minor:
	git tag $(shell svu minor)

tag-patch:
	git tag $(shell svu patch)

release:
	git push origin --tags
	GOWORK=off GOPROXY=proxy.golang.org go list -m github.com/go-go-golems/go-go-agent@$(shell svu current)

bump-deps:
	GOWORK=off go get github.com/go-go-golems/geppetto@latest
	GOWORK=off go get github.com/go-go-golems/glazed@latest
	GOWORK=off go get github.com/go-go-golems/go-go-goja@latest
	GOWORK=off go get github.com/go-go-golems/pinocchio@latest
	GOWORK=off go mod tidy

install:
	GOWORK=off go build -o ./dist/$(BINARY) ./cmd/chat
	@if command -v $(BINARY) >/dev/null 2>&1; then \
		cp ./dist/$(BINARY) $$(which $(BINARY)); \
	else \
		echo "$(BINARY) is not on PATH; built ./dist/$(BINARY)"; \
	fi
