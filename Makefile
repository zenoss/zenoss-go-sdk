# Workaround when docker run with -u.
ifeq ($(HOME),/)
	export GOCACHE = /tmp/.cache
	export HOME =
endif

GO				:= go
GOBIN			:= $(GOPATH)/bin
BASE            := $(GOPATH)/src/$(PACKAGE)
COVERAGE_DIR	:= $(CURDIR)/coverage

V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

.PHONY: default
default: tools dependencies check test

# Tools

$(GOBIN):
	@mkdir -p $@
$(GOBIN)/%: $(GOBIN) | $(BASE)
	@[ -x $@ ] || echo "$(M)" Installing $* && go get $(PACKAGE)

GOLINT = $(GOBIN)/golint
$(GOBIN)/golint: PACKAGE=golang.org/x/lint/golint

GINKGO = $(GOBIN)/ginkgo
$(GOBIN)/ginkgo: PACKAGE=github.com/onsi/ginkgo/ginkgo

GOCOVMERGE = $(GOBIN)/gocovmerge
$(GOBIN)/gocovmerge: PACKAGE=github.com/wadey/gocovmerge

GOCOV = $(GOBIN)/gocov
$(GOBIN)/gocov: PACKAGE=github.com/axw/gocov/...

GOCOVXML = $(GOBIN)/gocov-xml
$(GOBIN)/gocov-xml: PACKAGE=github.com/AlekSi/gocov-xml

.PHONY: tools
tools: $(GOLINT) $(GINKGO) $(GOCOVMERGE) $(GOCOV) $(GOCOVXML) ; $(info $(M) installed tools) @ ## Install tools.

# Dependencies

.PHONY: dependencies
dependencies: ; $(info $(M) downloading dependencies…) @ ## Install dependencies with go mod download.
	$Q go mod download

# Static Checks

.PHONY: check
check: fmt-check vet lint ; $(info $(M) static checks complete) @ ## Run all static checks.

.PHONY: fmt-check
fmt-check: ; $(info $(M) running gofmt…) @ ## Check all files with gofmt.
	$Q [ -z "$$(gofmt -l -s .)" ] || (gofmt -d -e -s . ; exit 1)

.PHONY: vet
vet: ; $(info $(M) running go vet…) @ ## Run go vet on all packages.
	$Q $(GO) vet $$($(GO) list ./...)

.PHONY: lint
lint: | $(GOLINT) ; $(info $(M) running golint…) @ ## Check packages with golint.
	$Q $(GOLINT) -set_exit_status $$($(GO) list ./...)

# Test

.PHONY: test
test: COVERAGE_PROFILE := $(COVERAGE_DIR)/profile.out
test: COVERAGE_HTML    := $(COVERAGE_DIR)/index.html
test: COVERAGE_XML     := $(COVERAGE_DIR)/coverage.xml
test: | $(GINKGO) $(GOCOVMERGE) $(GOCOV) $(GOCOVXML) ; $(info $(M) running tests with coverage…) @ ## Execute tests with ginkgo. (including coverage)
	$Q mkdir -p $(COVERAGE_DIR)
	$Q $(GINKGO) -r -cover -covermode=count
	$Q $(GOCOVMERGE) $$(find . -name \*.coverprofile) > $(COVERAGE_PROFILE)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)

.PHONY: test-quick
test-quick: | $(GINKGO) ; $(info $(M) running tests without coverage…) @ ## Execute tests with ginkgo. (no coverage)
	$Q mkdir -p $(COVERAGE_DIR)
	$Q $(GINKGO) -r

.PHONY: test-watch
test-watch: | $(GINKGO) ; $(info $(M) watching for changes with ginkgo…) @ ## Watch for changes and execute tests.
	$Q mkdir -p $(COVERAGE_DIR)
	$Q $(GINKGO) watch -r -cover -covermode=count

# Fixes

.PHONY: fix
fix: mod-tidy fmt

.PHONY: mod-tidy
mod-tidy: ; $(info $(M) running go mod tidy…) @ ## Update dependencies with go mod tidy.
	$Q go mod tidy

.PHONY: fmt
fmt: ; $(info $(M) running gofmt…) @ ## Run gofmt on all files.
	$Q gofmt -l -w .

# Misc

.PHONY: clean
clean: ; $(info $(M) cleaning…)	@ ## Cleanup everything.
	@rm -f **/junit.xml internal/**/junit.xml
	@rm -f **/*.coverprofile internal/**/*.coverprofile
	@rm -rf $(COVERAGE_DIR)

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'
