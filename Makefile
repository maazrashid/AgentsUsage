GO ?= go
GOFLAGS ?= -trimpath -buildvcs=false

ifeq ($(OS),Windows_NT)
BINARY ?= AgentsUsage.exe
LDFLAGS ?= -s -w -H=windowsgui
else
BINARY ?= AgentsUsage
LDFLAGS ?= -s -w
endif

.PHONY: build test vet check dist clean

build:
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/agentsusage

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

check: test vet

dist:
	pwsh -NoProfile -File scripts/build-release.ps1 -Target all

clean:
	$(GO) clean
