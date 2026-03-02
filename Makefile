.PHONY: cli-build cli-run cli-clean cli-test-loop cli-test-local-openspend cli-test-real-backend

CLI_BIN_DIR := bin
CLI_BIN := $(CLI_BIN_DIR)/openspend
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
OPENSPEND_DEFAULT_BASE_URL ?= https://openspend.ai
LDFLAGS := -X main.version=$(VERSION) -X github.com/promptingcompany/openspend-cli/internal/config.defaultBaseURL=$(OPENSPEND_DEFAULT_BASE_URL)

cli-build:
	mkdir -p $(CLI_BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(CLI_BIN) .

cli-run: cli-build
	./$(CLI_BIN) --help

cli-clean:
	rm -rf $(CLI_BIN_DIR)

cli-test-loop: cli-build
	go test ./...
	./$(CLI_BIN) --version
	./$(CLI_BIN) auth login --help
	./$(CLI_BIN) dashboard --help
	./$(CLI_BIN) dashboard policy init --help
	./$(CLI_BIN) dashboard agent create --help
	./$(CLI_BIN) dashboard agent update --help
	./$(CLI_BIN) search --help
	./$(CLI_BIN) whoami --help

cli-test-local-openspend: cli-build
	OPENSPEND_MARKETPLACE_BASE_URL=$${OPENSPEND_MARKETPLACE_BASE_URL:-http://127.0.0.1:5555} OPENSPEND_ALLOW_SIGNUP=1 OPENSPEND_TEST_EMAIL=$${OPENSPEND_TEST_EMAIL:-admin@example.com} OPENSPEND_TEST_PASSWORD=$${OPENSPEND_TEST_PASSWORD:-changeme123} ./scripts/test-real-backend.sh

cli-test-real-backend: cli-build
	./scripts/test-real-backend.sh
