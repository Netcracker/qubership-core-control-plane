# tests/helm.mk
# Helm chart validation targets.

.PHONY: test-helm

test-helm:
	@echo "$(BLUE)Validating Helm chart...$(NC)"
	@bash tests/scripts/helm-validate.sh helm-charts
	@echo "$(GREEN)✓ Helm validation passed!$(NC)"
