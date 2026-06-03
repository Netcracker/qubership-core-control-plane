# tests/suite.mk
# Shared variables and helpers for all test targets.

# Environment parameters (can be overridden from CI: NAMESPACE=... make ...).
NAMESPACE      ?= core-1-core
TEST_TIMEOUT   ?= 10m
PF_SCRIPT      := bash tests/scripts/port-forward.sh

# ANSI colours for output.
GREEN  := \033[0;32m
RED    := \033[0;31m
YELLOW := \033[1;33m
BLUE   := \033[0;34m
NC     := \033[0m

# Re-export for sub-scripts.
export NAMESPACE

# Helper: run go test without integration build tags.
define go_test_unit
	@go clean -testcache
	@go test -v -race -short -timeout $(TEST_TIMEOUT) $(1)
endef

# Helper: run go test with the integration tag (no port-forward — uses miniredis).
define go_test_integration
	@go clean -testcache
	@go test -v -tags=integration -timeout $(TEST_TIMEOUT) -run "TestIntegration" $(1)
endef
