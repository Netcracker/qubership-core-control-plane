# tests/smoke.mk
# On-commit smoke validation of the ratelimit service.
# Scripts and k8s manifests live in tests/smoke/.

.PHONY: test-smoke test-smoke-full

# Full pipeline (non-interactive, CI-friendly):
#   baseline load (no ratelimit) → install → all scenarios → final load → comparison report.
# Env vars:
#   NAMESPACE, HELM_RELEASE, HELM_IMAGE_REPO, HELM_IMAGE_TAG, REDIS_ADDR
#   SKIP_INSTALL=true   — skip ratelimit installation (already deployed)
#   RESULTS_DIR         — output dir for JSON summaries and the markdown report
test-smoke:
test-smoke-full: test-smoke
test-smoke:
	@echo "$(BLUE)Running ratelimit smoke validation...$(NC)"
	@bash tests/smoke/run-integration-test.sh
	@echo "$(GREEN)✓ Smoke validation completed.$(NC)"
