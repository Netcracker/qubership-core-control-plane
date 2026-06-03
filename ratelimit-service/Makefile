# ratelimit-service/Makefile
#
# Root Makefile — thin aggregator. Target logic lives in tests/*.mk.

include tests/suite.mk
include tests/unit.mk
include tests/integration.mk
include tests/e2e.mk
include tests/cloud-e2e.mk
include tests/load.mk
include tests/smoke.mk
include tests/helm.mk

.PHONY: help test test-all clean

help:
	@echo "Available targets (grouped):"
	@echo ""
	@echo "Unit tests (fast, no external deps):"
	@echo "  make test-unit                   - Run all unit tests"
	@echo "  make test-ratelimit              - Run ratelimit package unit tests"
	@echo "  make test-controller             - Run controller package unit tests"
	@echo "  make test-api                    - Run API package unit tests"
	@echo "  make test-metrics                - Run metrics package unit tests"
	@echo "  make test-ratelimit-bench        - Run ratelimit benchmarks"
	@echo "  make test-coverage               - Run unit tests with coverage gate (env: COVERAGE_THRESHOLD)"
	@echo ""
	@echo "Integration tests (miniredis, no kubectl required):"
	@echo "  make test-integration            - Run all integration tests"
	@echo "  make test-integration-ratelimit  - Run ratelimit integration tests"
	@echo "  make test-integration-controller - Run controller integration tests"
	@echo "  make test-integration-api        - Run API integration tests"
	@echo "  make test-integration-metrics    - Run metrics integration tests"
	@echo "  make test-integration-all        - Alias for test-integration"
	@echo ""
	@echo "Local end-to-end (port-forward to in-cluster Redis required):"
	@echo "  make test-e2e                    - Run local e2e tests"
	@echo "  make test-integration-e2e        - Alias for test-e2e (kept for CI compat)"
	@echo ""
	@echo "Cloud end-to-end (full stack deployed in cluster):"
	@echo "  make test-cloud-e2e              - Run cloud e2e tests"
	@echo "  make test-cloud-e2e-layered      - Run composite-limits test (long, ~2 min)"
	@echo ""
	@echo "Smoke / on-commit validation (full stack in cluster):"
	@echo "  make test-smoke                  - baseline → install → scenarios → report"
	@echo ""
	@echo "Helm chart:"
	@echo "  make test-helm                   - Validate Helm chart (lint + template + kubeconform)"
	@echo ""
	@echo "Aggregates:"
	@echo "  make test                        - Default for CI: unit + integration + helm"
	@echo "  make test-all                    - Everything (unit + integration + e2e)"
	@echo ""
	@echo "Utilities:"
	@echo "  make clean                       - Clean test cache, stop port-forwards"
	@echo "  make help                        - Show this help"

# Default CI entry point: unit + integration + helm (no kubectl required).
test: test-unit test-integration test-helm

# Everything except cloud e2e (cloud e2e is opt-in due to deployment requirements).
test-all: test-unit test-integration test-e2e
	@echo "$(GREEN)✓ All local tests (unit + integration + e2e) passed!$(NC)"

clean:
	@echo "$(BLUE)Cleaning...$(NC)"
	@go clean -testcache
	@go mod tidy
	@bash tests/scripts/port-forward.sh --profile=local --stop || true
	@bash tests/scripts/port-forward.sh --profile=cloud --stop || true
	@echo "$(GREEN)✓ Clean completed!$(NC)"
