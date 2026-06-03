# tests/unit.mk
# Unit targets (no external dependencies).

.PHONY: test-unit test-ratelimit test-controller test-api test-metrics test-ratelimit-bench test-coverage

test-unit:
	@echo "$(BLUE)Running all unit tests...$(NC)"
	$(call go_test_unit, ./pkg/...)
	@echo "$(GREEN)✓ Unit tests passed!$(NC)"

test-ratelimit:
	@echo "$(BLUE)Running ratelimit unit tests...$(NC)"
	$(call go_test_unit, ./pkg/ratelimit/...)
	@echo "$(GREEN)✓ Ratelimit unit tests passed!$(NC)"

test-controller:
	@echo "$(BLUE)Running controller unit tests...$(NC)"
	$(call go_test_unit, ./pkg/controller/...)
	@echo "$(GREEN)✓ Controller unit tests passed!$(NC)"

test-api:
	@echo "$(BLUE)Running API unit tests...$(NC)"
	$(call go_test_unit, ./pkg/api/...)
	@echo "$(GREEN)✓ API unit tests passed!$(NC)"

test-metrics:
	@echo "$(BLUE)Running metrics unit tests...$(NC)"
	$(call go_test_unit, ./pkg/metrics/...)
	@echo "$(GREEN)✓ Metrics unit tests passed!$(NC)"

test-ratelimit-bench:
	@echo "$(BLUE)Running ratelimit benchmarks...$(NC)"
	@go test -bench=. -benchmem ./pkg/ratelimit/...
	@echo "$(GREEN)✓ Ratelimit benchmarks passed!$(NC)"

test-coverage:
	@echo "$(BLUE)Running coverage with threshold=$${COVERAGE_THRESHOLD:-60}%...$(NC)"
	@bash tests/scripts/coverage.sh
	@echo "$(GREEN)✓ Coverage gate passed!$(NC)"
