GO_BIN ?= $(shell go env GOPATH)/bin
PROTOC_GEN_GO_GRPC_VERSION ?= v1.6.2

.PHONY: compile install-protoc-gen-go-grpc test

compile: install-protoc-gen-go-grpc
	PATH="$(GO_BIN):$$PATH" protoc api/v1/*.proto \
	--go_out=. \
	--go-grpc_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_opt=paths=source_relative \
	--proto_path=.

install-protoc-gen-go-grpc:
	GOBIN="$(GO_BIN)" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

# Verbose: log each test/subtest as it runs. Override flags, e.g.:
#   make test TEST_FLAGS='-race -v -count=1'
#   make test TEST_FLAGS='-race -v -json'   # machine-readable stream
TEST_FLAGS ?= -race -v

test:
	go test $(TEST_FLAGS) ./...