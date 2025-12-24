PROGRAM_NAME = notification-relay-linux-amd64

COMMIT=$(shell git rev-parse --short HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
TAG=$(shell git describe --tags |cut -d- -f1)

LDFLAGS = -X main.gitTag=${TAG} -X main.gitCommit=${COMMIT} -X main.gitBranch=${BRANCH}

# Test configuration
TEST_CONFIG_DIR := $(shell pwd)/testdata
TEST_CONFIG_FILE := $(TEST_CONFIG_DIR)/config.json

# Docker configuration
DOCKER_IMAGE := metalmon/notification-relay
DOCKER_TAG ?= $(shell git describe --tags --always)

.PHONY: help clean dep build install uninstall lint lint-deps test test-race test-setup docker-build docker-push release

.DEFAULT_GOAL := help

help: ## Display this help screen.
	@echo "Makefile available targets:"
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  * \033[36m%-15s\033[0m %s\n", $$1, $$2}'

dep: ## Download the dependencies.
	go mod download

build: dep ## Build notification-relay executable.
	mkdir -p ./bin
	CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} go build -trimpath -ldflags "-s -w ${LDFLAGS}" -o bin/${PROGRAM_NAME}

release: build ## Create release archive
	cd bin && tar czf ${PROGRAM_NAME}.tar.gz ${PROGRAM_NAME}
	@echo "Release archive created: bin/${PROGRAM_NAME}.tar.gz"

clean: ## Clean build directory.
	rm -f ./bin/${PROGRAM_NAME}
	rm -f ./bin/${PROGRAM_NAME}.tar.gz
	rmdir ./bin
	rm -rf $(TEST_CONFIG_DIR)

lint-deps: ## Install linting dependencies
	@echo "Installing linting tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2
	@go install github.com/securego/gosec/v2/cmd/gosec@v2.18.2

lint: dep lint-deps ## Lint the source files
	golangci-lint run --timeout 5m
	gosec -quiet ./...

check: lint test ## Run all checks
	go vet ./...

test-setup: ## Setup test environment
	@mkdir -p $(TEST_CONFIG_DIR)
	@mkdir -p $(TEST_CONFIG_DIR)/etc/notification-relay
	@echo '{"vapid_public_key": "test", "firebase_config": {}}' > $(TEST_CONFIG_DIR)/etc/notification-relay/config.json
	@echo '{}' > $(TEST_CONFIG_DIR)/etc/notification-relay/credentials.json
	@echo '{}' > $(TEST_CONFIG_DIR)/etc/notification-relay/user-device-map.json
	@echo '{}' > $(TEST_CONFIG_DIR)/etc/notification-relay/decoration.json
	@echo '{}' > $(TEST_CONFIG_DIR)/etc/notification-relay/icons.json
	@echo '{"type":"service_account","project_id":"test"}' > $(TEST_CONFIG_DIR)/etc/notification-relay/service-account.json
	@chmod -R 600 $(TEST_CONFIG_DIR)/etc/notification-relay/*.json
	@chmod -R 700 $(TEST_CONFIG_DIR)/etc/notification-relay

test: dep test-setup ## Run tests without race detector
	NOTIFICATION_RELAY_CONFIG="$(shell pwd)/testdata/etc/notification-relay/config.json" \
	GOOGLE_APPLICATION_CREDENTIALS="$(shell pwd)/testdata/etc/notification-relay/service-account.json" \
	go test -p 1 -timeout 300s -coverprofile=.test_coverage.txt ./... && \
		go tool cover -func=.test_coverage.txt | tail -n1 | awk '{print "Total test coverage: " $$3}'
	@rm -f .test_coverage.txt

test-race: dep test-setup ## Run tests with race detector
	NOTIFICATION_RELAY_CONFIG="$(shell pwd)/testdata/etc/notification-relay/config.json" \
	GOOGLE_APPLICATION_CREDENTIALS="$(shell pwd)/testdata/etc/notification-relay/service-account.json" \
	CGO_ENABLED=1 go test -race -p 1 -timeout 300s ./...

docker-build: ## Build docker image
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	docker image prune --force --filter label=stage=intermediate

docker-push: ## Push docker image to registry
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest