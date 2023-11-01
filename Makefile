# Workaround when docker run with -u.
ifeq ($(HOME),/)
	export GOCACHE = /tmp/.cache
	export HOME =
endif

ROOTDIR					?= $(CURDIR)
SHELL					:= /bin/bash
GO						:= $(shell command -v go 2> /dev/null)
GOFUMPT					:= $(shell command -v gofumpt 2> /dev/null)
REVIVE					:= $(shell command -v revive 2> /dev/null)
GINKGO					:= $(shell command -v ginkgo 2> /dev/null)
GOCOV					:= $(shell command -v gocov 2> /dev/null)
GOCOVXML				:= $(shell command -v gocov-xml 2> /dev/null)
COVERAGE_DIR			:= $(CURDIR)/coverage
ZENKIT_BUILD_VERSION	:= 1.17.0
BUILD_IMG				:= zenoss/zenkit-build:$(ZENKIT_BUILD_VERSION)
DOCKER_PARAMS			:=	--rm \
							--volume $(ROOTDIR):/workspace/:rw \
							--env CGO_ENABLED=1 \
							--workdir /workspace/
DOCKER_CMD				:= docker run -t $(DOCKER_PARAMS) $(BUILD_IMG)

M = $(shell printf "\033[34;1m▶\033[0m")
RED = $(shell printf "\033[31;1m▶\033[0m")

# Static Checks

.PHONY: fmt
fmt:
	@echo "$(M) gofumpt: running"
	@if [[ "$(shell $(GOFUMPT) -l -w .)" ]]; then \
		echo "$(RED) gofumpt: Please commit formatted files"; \
		exit 1; \
	else \
		echo "$(M) gofumpt: files look good"; \
	fi

.PHONY: lint
lint: 
	@echo "$(M) running revive linter…"
	$(REVIVE) \
		-config $(ROOTDIR)/.revive.toml \
		-formatter=friendly \
		-exclude $(ROOTDIR)/vendor/... \
		$(ROOTDIR)/...
	@echo "$(M) linted with revive linter"

# Test

.PHONY: test
test: COVERAGE_PROFILE := coverprofile.out
test: COVERAGE_HTML    := $(COVERAGE_DIR)/index.html
test: COVERAGE_XML     := $(COVERAGE_DIR)/coverage.xml
test: fmt lint
	@echo "Please ENABLE RACE DETECTOR"
	@mkdir -p $(COVERAGE_DIR)
	@chmod -R o+w $(COVERAGE_DIR)
	@$(GINKGO) \
		run \
		-r \
		--tags integration \
		--cover \
		--coverprofile $(COVERAGE_PROFILE) \
		--covermode=count \
		--junit-report=junit.xml
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)
	@echo "Please ENABLE RACE DETECTOR"

.PHONY: test-containerized
test-containerized:
	$(DOCKER_CMD) make test

# Misc

.PHONY: clean
clean: ; $(info $(M) cleaning…)	@ ## Cleanup everything.
	@rm -f junit.xml internal/junit.xml
	@rm -f coverprofile.out internal/coverprofile.out
	@rm -rf $(COVERAGE_DIR)
