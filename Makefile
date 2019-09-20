PROJECT_NAME := "ddns-server"
PKG := "gitlab.com/lsoftop/$(PROJECT_NAME)"
PKG_LIST := $(shell go list ./... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/ | grep -v _test.go)

.PHONY: all dep build clean test coverage coverhtml lint

all: build

lint: dep ## Lint the files
	`go list -f {{.Target}} golang.org/x/lint/golint` -set_exit_status

test: ## Run unittests
	@go test -short 

race: dep ## Run data race detector
	@go test -race -short

coverage: ## Generate global code coverage report
	./tools/coverage.sh;

coverhtml: ## Generate global code coverage report in HTML
	./tools/coverage.sh html;

dep: ## Get the dependencies
	@go get -v -d ./...
	@go get -u golang.org/x/lint/golint

build: dep ## Build the binary file
	@go build -i -v $(PKG)

clean: ## Remove previous build
	@rm -f $(PROJECT_NAME)

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
