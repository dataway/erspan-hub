# ──────────────────────────────────────────────────────────────────────────────
# Project settings
# ──────────────────────────────────────────────────────────────────────────────
MODULE      := erspan-hub
CMD_DIR     := cmd/erspan-hub
BINARY      := erspan-hub
CLIENT_DIR  := cmd/hubcap
CLIENT_BIN  := hubcap
BIN_DIR     := bin
GOFILES     := $(shell find . -type f -name '*.go' -not -path "./$(BIN_DIR)/*")
PKGS        := $(shell go list ./...)
GOLANGCI_LINT?= golangci-lint               # adjust if installed under a different name
# ──────────────────────────────────────────────────────────────────────────────
# protobuf settings
PROTO_SRC_DIR := proto
PROTO_GEN_DIR := generated
PROTO_FILES   := $(shell find $(PROTO_SRC_DIR) -name '*.proto')
# ──────────────────────────────────────────────────────────────────────────────
VERSION	    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "no_git")
RPM_VERSION := $(subst -,_,$(patsubst v%,%,$(VERSION)))
COMMIT    	:= $(shell git rev-parse --short HEAD 2>/dev/null || echo "no_git")
DATE	    := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS	    := -ldflags "-s -w -X 'main.Version=$(VERSION)' -X 'main.Commit=$(COMMIT)' -X 'main.Date=$(DATE)'"
GOOS ?= linux
GOARCH ?= amd64

export PATH := $(shell go env GOPATH)/bin:$(PATH)

.PHONY: all fmt vet lint test proto tidy build buildclient buildwinclient clean

#all: fmt vet lint test build
all: proto fmt vet build buildclient buildwinclient
action_build: proto vet test build buildclient buildwinclient

# format all Go code
fmt:
	@echo "→ go fmt"
	@go fmt ./...

# vet catches suspicious constructs
vet:
	@echo "→ go vet"
	@go vet ./...

# static analysis & style enforcement
lint:
	@echo "→ golangci-lint run"
	@$(GOLANGCI_LINT) run ./...

# code generation from protobuf definitions
proto:
	@echo "→ protoc"
	@protoc \
		-I $(PROTO_SRC_DIR) \
		--go_out=$(PROTO_GEN_DIR) \
		--go-grpc_out=$(PROTO_GEN_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)

# run unit tests with race detector
test:
	@echo "→ go test (race)"
	@go test -race -v ./...

# prune and verify module dependencies
tidy:
	@echo "→ go mod tidy"
	@go mod tidy

# build the production binary
# TODO: add CGO_ENABLED=0 for static binary
build: tidy proto
	@echo "→ go build → $(BIN_DIR)/$(BINARY)"
	@mkdir -p $(BIN_DIR)

	#CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) ./$(CMD_DIR)

buildclient: tidy proto
	@echo "→ go build client → $(BIN_DIR)/$(CLIENT_BIN)"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build $(LDFLAGS) -o $(BIN_DIR)/$(CLIENT_BIN) ./$(CLIENT_DIR)

buildwinclient: tidy proto
	@echo "→ go build windows client → $(BIN_DIR)/$(CLIENT_BIN).exe"
	@mkdir -p $(BIN_DIR)
	go-winres make \
		--in $(CLIENT_DIR)/winres.json \
		--product-version "$(VERSION)" \
		--file-version "$(VERSION)" \
		--out $(CLIENT_DIR)/winres
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
	go build $(LDFLAGS) -o $(BIN_DIR)/$(CLIENT_BIN).exe ./$(CLIENT_DIR)
	rm $(CLIENT_DIR)/winres_windows_*.syso

rpm: build
	@echo "→ create RPM package"
	mkdir -p rpmbuild/BUILD rpmbuild/RPMS rpmbuild/SOURCES rpmbuild/SPECS rpmbuild/SRPMS
	cp $(BIN_DIR)/$(BINARY) erspan-hub.service config-example.yaml vpn.html rpmbuild/SOURCES
	cp erspan-hub.spec rpmbuild/SPECS
	rpmbuild --define "_topdir $(shell pwd)/rpmbuild" \
		--define "version $(RPM_VERSION)" \
		-bb rpmbuild/SPECS/erspan-hub.spec
	cp rpmbuild/RPMS/*/$(BINARY)-$(RPM_VERSION)-1.*.rpm $(BIN_DIR)

# remove built artifacts
clean:
	@echo "→ clean"
	rm -rf $(BIN_DIR) rpmbuild
