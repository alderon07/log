compile:
	protoc api/v1/*.proto --go_out=. --go_opt=paths=source_relative --proto_path=.

# Verbose: log each test/subtest as it runs. Override flags, e.g.:
#   make test TEST_FLAGS='-race -v -count=1'
#   make test TEST_FLAGS='-race -v -json'   # machine-readable stream
TEST_FLAGS ?= -race -v

test:
	go test $(TEST_FLAGS) ./...