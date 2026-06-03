# tests/integration.mk
# Integration targets (miniredis, no kubectl).
.PHONY: test-integration test-integration-all \
        test-integration-ratelimit test-integration-controller \
        test-integration-api test-integration-metrics

test-integration-ratelimit:
	@echo "$(BLUE)Running ratelimit integration tests...$(NC)"
	$(call go_test_integration, ./pkg/ratelimit/...)
	@echo "$(GREEN)✓ Ratelimit integration tests passed!$(NC)"

test-integration-controller:
	@echo "$(BLUE)Running controller integration tests...$(NC)"
	$(call go_test_integration, ./pkg/controller/...)
	@echo "$(GREEN)✓ Controller integration tests passed!$(NC)"

test-integration-api:
	@echo "$(BLUE)Running API integration tests...$(NC)"
	$(call go_test_integration, ./pkg/api/...)
	@echo "$(GREEN)✓ API integration tests passed!$(NC)"

test-integration-metrics:
	@echo "$(BLUE)Running metrics integration tests...$(NC)"
	$(call go_test_integration, ./pkg/metrics/...)
	@echo "$(GREEN)✓ Metrics integration tests passed!$(NC)"

test-integration:
	@echo "$(BLUE)Running all integration tests...$(NC)"
	$(call go_test_integration, ./...)
	@echo "$(GREEN)✓ Integration tests passed!$(NC)"

# Backward-compat alias.
test-integration-all: test-integration
