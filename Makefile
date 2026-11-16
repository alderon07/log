GO_BIN ?= $(shell go env GOPATH)/bin
PROTOC_GEN_GO_GRPC_VERSION ?= v1.6.2

# Directory for generated CA and server certificate files.
CONFIG_PATH ?= $(HOME)/.proglog

.PHONY: init gencert compile install-protoc-gen-go-grpc test

# Ensure the certificate output directory exists.
init:
	mkdir -p $(CONFIG_PATH)

# Generate the CA and server certificates, then move them to CONFIG_PATH.
gencert: init
	cfssl gencert -initca test/ca-csr.json | cfssljson -bare ca
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=test/ca-config.json \
		-profile=server \
		test/server-csr.json | cfssljson -bare server
	mv *.pem *.csr $(CONFIG_PATH)

# Generate Go and gRPC code next to the input proto files.
# source_relative preserves each proto's relative directory; "." is the import root.
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