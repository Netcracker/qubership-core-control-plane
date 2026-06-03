# tests/e2e.mk
# Local end-to-end (requires kubectl port-forward to in-cluster Redis).

.PHONY: test-e2e test-integration-e2e

test-e2e:
	@echo "$(BLUE)Running local e2e tests...$(NC)"
	@$(PF_SCRIPT) --profile=local --start
	@go clean -testcache
	@TEST_REDIS_ADDR=localhost:6379 TEST_RATELIMIT_ADDR=localhost:8081 \
		go test -v -tags=e2e -timeout $(TEST_TIMEOUT) ./tests/e2e/... ; \
		EXIT=$$? ; \
		$(PF_SCRIPT) --profile=local --stop ; \
		exit $$EXIT
	@echo "$(GREEN)✓ E2E tests passed!$(NC)"

# Backward-compat alias (CI references this target name).
test-integration-e2e: test-e2e
