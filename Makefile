default: ci

ci: lint test fmt-check imports-check integration

# Tooling versions
GOLANGCILINTVERSION?=1.23.8
GOIMPORTSVERSION?=v0.1.2
GOXVERSION?=v1.0.1
GOTESTSUMVERSION?=v1.6.4

CIARTIFACTS?=ci-artifacts
COVERAGEOUT?=coverage.out
COVERAGEHTML?=coverage.html
PACKAGENAME?=lacework-cli
CLINAME?=lacework
# Honeycomb variables
HONEYDATASET?=lacework-cli-dev
# => HONEYAPIKEY should be exported on every developers workstation or else events
#                won't be recorded in Honeycomb. Inside our CI/CD pipeline this
#                secret is set as well as a different dataset for production
GO_LDFLAGS="-X github.com/lacework/go-sdk/cli/cmd.Version=$(shell cat VERSION) \
            -X github.com/lacework/go-sdk/cli/cmd.GitSHA=$(shell git rev-parse HEAD) \
            -X github.com/lacework/go-sdk/cli/cmd.HoneyApiKey=$(HONEYAPIKEY) \
            -X github.com/lacework/go-sdk/cli/cmd.HoneyDataset=$(HONEYDATASET) \
            -X github.com/lacework/go-sdk/cli/cmd.BuildTime=$(shell date +%Y%m%d%H%M%S)"
GOFLAGS=-mod=vendor
CGO_ENABLED?=0
export GOFLAGS GO_LDFLAGS CGO_ENABLED

# CI variables
CI_V2_ACCOUNT?=customerdemo
export CI_V2_ACCOUNT

.PHONY: help
help:
	@echo "-------------------------------------------------------------------"
	@echo "Lacework go-sdk Makefile helper:"
	@echo ""
	@grep -Fh "##" $(MAKEFILE_LIST) | grep -v grep | sed -e 's/\\$$//' | sed -E 's/^([^:]*):.*##(.*)/ \1 -\2/'
	@echo "-------------------------------------------------------------------"

.PHONY: prepare
prepare: install-tools go-vendor ## Initialize the go environment

.PHONY: test
test: prepare ## Run all go-sdk tests
	gotestsum -f testname -- -v -cover -coverprofile=$(COVERAGEOUT) $(shell go list ./... | grep -v integration)

.PHONY: integration
integration: build-cli-cross-platform integration-only ## Build and run integration tests

.PHONY: integration-only
integration-only: ## Run integration tests
	PATH=$(PWD)/bin:${PATH} go test -v github.com/lacework/go-sdk/integration -timeout 30m -tags="\
		account \
		agent_token \
		compliance \
		configure \
		event \
		help \
		integration \
		migration \
		policy \
		query \
		version \
		vulnerability"

.PHONY: integration-lql
integration-lql: build-cli-cross-platform integration-lql-only ## Build and run lql integration tests

.PHONY: integration-lql-only
integration-lql-only: ## Run lql integration tests
	PATH=$(PWD)/bin:${PATH} go test -v github.com/lacework/go-sdk/integration -timeout 30m -tags="query"

.PHONY: integration-policy
integration-policy: build-cli-cross-platform integration-policy-only ## Build and run lql policy tests

.PHONY: integration-policy-only
integration-policy-only: ## Run lql policy tests
	PATH=$(PWD)/bin:${PATH} go test -v github.com/lacework/go-sdk/integration -timeout 30m -tags="policy"

.PHONY: coverage
coverage: test ## Output coverage profile information for each function
	go tool cover -func=$(COVERAGEOUT)

.PHONY: coverage-html
coverage-html: test ## Generate HTML representation of coverage profile
	go tool cover -html=$(COVERAGEOUT)

.PHONY: coverage-ci
coverage-ci: test ## Generate HTML coverage output for ci pipeline.
	mkdir -p $(CIARTIFACTS)
	go tool cover -html=$(COVERAGEOUT) -o "$(CIARTIFACTS)/$(COVERAGEHTML)"

.PHONY: go-vendor
go-vendor: ## Runs go mod tidy, vendor and verify to cleanup, copy and verify dependencies
	go mod tidy
	go mod vendor
	go mod verify

.PHONY: lint
lint: ## Runs go linter
	golangci-lint run

.PHONY: fmt
fmt: ## Runs and applies go formatting changes
	@gofmt -w -l ./
	@goimports -w -l ./

.PHONY: fmt-check
fmt-check: ## Lists formatting issues
	@test -z $(shell gofmt -l $(shell go list -f {{.Dir}} ./...))

.PHONY: imports-check
imports-check: ## Lists imports issues
	@test -z $(shell goimports -l $(shell go list -f {{.Dir}} ./...))

.PHONY: build-cli-cross-platform
build-cli-cross-platform: ## Compiles the Lacework CLI for all supported platforms
	gox -output="bin/$(PACKAGENAME)-{{.OS}}-{{.Arch}}" \
            -os="linux windows" \
            -arch="amd64 386" \
            -osarch="darwin/amd64 darwin/arm64 linux/arm linux/arm64" \
            -ldflags=$(GO_LDFLAGS) \
            github.com/lacework/go-sdk/cli

.PHONY: generate-databox
generate-databox: ## *CI ONLY* Generates in memory representation of template files
	go generate internal/databox/box.go

.PHONY: generate-docs
generate-docs: ## *CI ONLY* Generates documentation
	go generate cli/cmd/docs.go

.PHONY: test-resources
test-resources: ## *CI ONLY* Prepares CI test containers
	scripts/prepare_test_resources.sh all

.PHONY: install-cli
install-cli: build-cli-cross-platform ## Build and install the Lacework CLI binary at /usr/local/bin/lacework
ifeq (x86_64, $(shell uname -m))
	mv bin/$(PACKAGENAME)-$(shell uname -s | tr '[:upper:]' '[:lower:]')-amd64 /usr/local/bin/$(CLINAME)
else
	mv bin/$(PACKAGENAME)-$(shell uname -s | tr '[:upper:]' '[:lower:]')-386 /usr/local/bin/$(CLINAME)
endif
	@echo "\nThe lacework cli has been installed at /usr/local/bin"

.PHONY: release
release: lint test fmt-check imports-check build-cli-cross-platform ## *CI ONLY* Prepares a new release of the go-sdk
	scripts/release.sh prepare

.PHONY: install-tools
install-tools: ## Install go indirect dependencies
ifeq (, $(shell which golangci-lint))
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v$(GOLANGCILINTVERSION)
endif
ifeq (, $(shell which goimports))
	go get golang.org/x/tools/cmd/goimports@$(GOIMPORTSVERSION)
endif
ifeq (, $(shell which gox))
	go get github.com/mitchellh/gox@$(GOXVERSION)
endif
ifeq (, $(shell which gotestsum))
	go get gotest.tools/gotestsum@$(GOTESTSUMVERSION)
endif

.PHONY: git-env
git-env: ## Configure git commit message style enforcement by applying git_env.sh
	scripts/git_env.sh

.PHONY: vagrant-macos-up
vagrant-macos-up: build-cli-cross-platform ## Start and provision the vagrant environment: MacOs Sierra
	$(call run_vagrant,macos-sierra,up)
.PHONY: vagrant-macos-login
vagrant-macos-login: build-cli-cross-platform ## Connect to vagrant environment: MacOs Sierra
	$(call run_vagrant,macos-sierra,ssh)
.PHONY: vagrant-macos-destroy
vagrant-macos-destroy: ## Stop and delete vagrant environment: MacOs Sierra
	$(call run_vagrant,macos-sierra,destroy -f)

.PHONY: vagrant-linux-up
vagrant-linux-up: build-cli-cross-platform ## Start and provision the vagrant environment: Ubuntu 1804
	$(call run_vagrant,ubuntu-1804,up)
.PHONY: vagrant-linux-login
vagrant-linux-login: build-cli-cross-platform ## Connect to vagrant environment: Ubuntu 1804
	$(call run_vagrant,ubuntu-1804,ssh)
.PHONY: vagrant-linux-destroy
vagrant-linux-destroy: ## Stop and delete vagrant environment: Ubuntu 1804
	$(call run_vagrant,ubuntu-1804,destroy -f)

.PHONY: vagrant-windows-up
vagrant-windows-up: build-cli-cross-platform ## Start and provision the vagrant environment: Windows 10
	$(call run_vagrant,windows-10,up)
.PHONY: vagrant-windows-login
vagrant-windows-login: build-cli-cross-platform ## Connect to vagrant environment: Windows 10
	$(call run_vagrant,windows-10,powershell)
.PHONY: vagrant-windows-destroy
vagrant-windows-destroy: ## Stop and delete vagrant environment: Windows 10
	$(call run_vagrant,windows-10,destroy -f)

define run_vagrant
	cd cli/vagrant/${1}; vagrant ${2}
endef
