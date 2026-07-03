#!/bin/bash
# run-integration-test.sh — On-commit smoke validation of the ratelimit service
#
# Flow:
#   Phase 0 — Deploy test pods (curl + k6) into the cluster
#   Phase 1 — Set up test backend (httpbin + HTTPRoute + DestinationRule +
#             gateway-pod-label EnvoyFilter). No rate limiting involved here.
#   Phase 2 — Baseline k6 load test (50 req/s × 30s, NO ratelimit active)
#   Phase 3 — Install ratelimit service (Redis if absent + Helm + its EnvoyFilter
#             + gateway restart) — this is what actually activates enforcement
#   Phase 4 — Run all test scenarios (priorities / accuracy / algo-compare / k6 tests)
#   Phase 5 — Final k6 load test (same 50 req/s × 30s, ratelimit active)
#   Phase 6 — Generate markdown comparison report + write to $GITHUB_STEP_SUMMARY
#
# Required env vars (all have defaults):
#   NAMESPACE         — k8s namespace            (default: core-1-core)
#   HELM_RELEASE      — Helm release name         (default: ratelimit)
#   HELM_CHART        — path to helm chart dir    (default: auto-detected from script location)
#   HELM_IMAGE_REPO   — ratelimit image repo      (default: ghcr.io/netcracker/ratelimit)
#   HELM_IMAGE_TAG    — ratelimit image tag        (default: feat-ratelimit-snapshot)
#   REDIS_ADDR        — Redis address             (default: redis.<ns>.svc.cluster.local:6379)
#   RESULTS_DIR       — where to store JSON+report (default: /tmp/ratelimit-smoke-<timestamp>)
#   SKIP_INSTALL      — set to "true" to skip Phase 2 (ratelimit already installed)
#   GATEWAY_URL       — in-cluster gateway URL    (default: http://public-gateway-istio.<ns>.svc.cluster.local:8080)

set -euo pipefail

# ── config ────────────────────────────────────────────────────────────────────

NAMESPACE="${NAMESPACE:-core-1-core}"
CURL_POD="curl-test-runner"
K6_POD="k6-test-runner"
SKIP_INSTALL="${SKIP_INSTALL:-false}"

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURES_DIR="$TEST_DIR/../fixtures"
HELM_CHART="${HELM_CHART:-$TEST_DIR/../../helm-charts}"

HELM_RELEASE="${HELM_RELEASE:-ratelimit}"
HELM_IMAGE_REPO="${HELM_IMAGE_REPO:-ghcr.io/netcracker/ratelimit}"
HELM_IMAGE_TAG="${HELM_IMAGE_TAG:-feat-ratelimit-snapshot}"
REDIS_ADDR="${REDIS_ADDR:-redis.${NAMESPACE}.svc.cluster.local:6379}"
GATEWAY_URL="${GATEWAY_URL:-http://public-gateway-istio.${NAMESPACE}.svc.cluster.local:8080}"

RESULTS_DIR="${RESULTS_DIR:-/tmp/ratelimit-smoke-$(date +%Y%m%d-%H%M%S)}"
mkdir -p "$RESULTS_DIR"

BASELINE_SUMMARY="$RESULTS_DIR/baseline-summary.json"
FINAL_SUMMARY="$RESULTS_DIR/final-summary.json"
REPORT_FILE="$RESULTS_DIR/comparison-report.md"

# ── colors (disabled in CI / non-tty) ────────────────────────────────────────

if [ -t 1 ] && [ "${CI:-false}" != "true" ]; then
  GREEN='\033[0;32m'; RED='\033[0;31m'; BLUE='\033[0;34m'; YELLOW='\033[1;33m'; NC='\033[0m'
else
  GREEN=''; RED=''; BLUE=''; YELLOW=''; NC=''
fi

# ── helpers ───────────────────────────────────────────────────────────────────

log_phase() {
  echo ""
  echo -e "${BLUE}════════════════════════════════════════════════${NC}"
  echo -e "${GREEN}  $1${NC}"
  echo -e "${BLUE}════════════════════════════════════════════════${NC}"
  echo ""
}

log_ok()   { echo -e "${GREEN}✅  $1${NC}"; }
log_err()  { echo -e "${RED}❌  $1${NC}"; }
log_info() { echo -e "${YELLOW}ℹ   $1${NC}"; }

# Compute expected rejection rate (%) from ratelimit config params.
# Usage: calc_expected_rejection LIMIT_PER_SEC TOTAL_REQUESTS DURATION_SEC
# Example: calc_expected_rejection 2 10 1  =>  "80.0%"
calc_expected_rejection() {
  local limit_per_sec=$1 total_req=$2 duration_sec=$3
  awk -v l="$limit_per_sec" -v t="$total_req" -v d="$duration_sec" 'BEGIN {
    max_allowed = l * d
    if (t > max_allowed) {
      printf "%.1f%%\n", (t - max_allowed) / t * 100
    } else {
      print "0.0%"
    }
  }'
}

apply_config() {
  local yaml_file=$1
  log_info "Applying $(basename "$yaml_file")"
  kubectl apply -f "$yaml_file" -n "$NAMESPACE"
  kubectl exec -n "$NAMESPACE" deployment/ratelimit -- \
    curl -s -X POST http://localhost:8082/api/v1/config/reload > /dev/null
  sleep 2
}

delete_config() {
  local cm_name=$1
  kubectl delete configmap "$cm_name" -n "$NAMESPACE" --ignore-not-found > /dev/null
  kubectl exec -n "$NAMESPACE" deployment/ratelimit -- \
    curl -s -X POST http://localhost:8082/api/v1/config/reload > /dev/null
  sleep 2
}

run_test() {
  local name=$1 script=$2 args=${3:-}
  echo ""
  echo -e "${BLUE}──────────────────────────────────────────────${NC}"
  echo -e "${GREEN}▶  $name${NC}"
  kubectl exec -n "$NAMESPACE" "$CURL_POD" -- sh -c "sh /scripts/$script $args"
  local rc=$?
  if [ $rc -eq 0 ]; then log_ok "$name"; else log_err "$name (exit $rc)"; fi
}

# Run a k6 test inside the k6 pod.
# If summary_out is non-empty, --summary-export is used and the result is
# fetched back to the host so the comparison report can read it.
run_k6_test() {
  local name=$1 script=$2 summary_out=${3:-} run_label=${4:-no ratelimit}
  echo ""
  echo -e "${BLUE}──────────────────────────────────────────────${NC}"
  echo -e "${GREEN}▶  K6: $name${NC}"

  local export_flag=""
  [ -n "$summary_out" ] && export_flag="--summary-export /tmp/k6-run-summary.json"

  kubectl exec -n "$NAMESPACE" "$K6_POD" -- sh -c "
    export K6_QUIET=1
    export RUN_LABEL='${run_label}'
    k6 run $export_flag /scripts/$script 2>&1 | tail -60
  "
  local rc=$?

  if [ -n "$summary_out" ]; then
    kubectl exec -n "$NAMESPACE" "$K6_POD" -- \
      cat /tmp/k6-run-summary.json > "$summary_out" 2>/dev/null || true
  fi

  if [ $rc -eq 0 ]; then log_ok "K6 $name"; else log_err "K6 $name (exit $rc)"; fi
}

# ── Phase 0: deploy test pods ─────────────────────────────────────────────────

phase_setup_pods() {
  log_phase "PHASE 0 — Setting up test pods"

  kubectl delete pod "$CURL_POD" -n "$NAMESPACE" --ignore-not-found
  kubectl delete pod "$K6_POD"   -n "$NAMESPACE" --ignore-not-found

  kubectl apply -f "$TEST_DIR/test-scripts.yaml" -n "$NAMESPACE"
  kubectl apply -f "$TEST_DIR/curl-test-runner.yaml"  -n "$NAMESPACE"
  kubectl apply -f "$TEST_DIR/k6-test-runner.yaml"    -n "$NAMESPACE"

  kubectl wait --for=condition=ready pod/"$K6_POD"   -n "$NAMESPACE" --timeout=120s
  kubectl wait --for=condition=ready pod/"$CURL_POD" -n "$NAMESPACE" --timeout=60s

  log_ok "Test pods ready"
}


phase_setup_backend() {
  log_phase "PHASE 1 — Setting up test backend (httpbin + routes)"

  # Deploy httpbin (test backend) if not present
  if ! kubectl get deployment httpbin -n "$NAMESPACE" &>/dev/null; then
    log_info "httpbin not found — deploying..."
    kubectl apply -f "$FIXTURES_DIR/httpbin.yaml" -n "$NAMESPACE"
    kubectl rollout status deployment/httpbin -n "$NAMESPACE" --timeout=120s
    log_ok "httpbin deployed"
  else
    log_info "httpbin already present — skipping"
  fi

  # Apply HTTPRoute: /test /burst /spike /fixed /sliding → httpbin
  log_info "Applying HTTPRoute (routes-loadtest.yaml)..."
  kubectl apply -f "$FIXTURES_DIR/routes-loadtest.yaml" -n "$NAMESPACE"
  log_ok "HTTPRoute applied"

  # Apply EnvoyFilter that adds x-gateway-id response header (needed for distribution
  # test). Response-header-only Lua filter — no rate limiting involved.
  log_info "Applying gateway-pod-label EnvoyFilter (response header only, no rate limiting)..."
  kubectl apply -f "$FIXTURES_DIR/gateway-pod-label.yaml" -n "$NAMESPACE"
  log_ok "gateway-pod-label EnvoyFilter applied"

  # Apply DestinationRule (ROUND_ROBIN LB for gateway).
  # The gateway host FQDN embeds the namespace, so render it from $NAMESPACE.
  log_info "Applying DestinationRule..."
  sed "s|__NAMESPACE__|${NAMESPACE}|g" "$FIXTURES_DIR/dest-rule.yaml" \
    | kubectl apply -n "$NAMESPACE" -f -
  log_ok "DestinationRule applied"
}

# ── Phase 2: baseline load test (no ratelimit) ─────────────────────────────────

phase_baseline() {
  log_phase "PHASE 2 — Baseline load test (WITHOUT ratelimit)"
  log_info "50 req/s × 30s — pure Istio gateway, no rate-limit enforcement"

  run_k6_test "Baseline" "k6-baseline-test.js" "$BASELINE_SUMMARY"

  if [ -s "$BASELINE_SUMMARY" ]; then
    log_ok "Baseline summary saved → $BASELINE_SUMMARY"
  else
    log_err "Baseline summary not captured — comparison will show partial data"
  fi
}

# ── Phase 3: install ratelimit ────────────────────────────────────────────────

phase_install() {
  if [ "$SKIP_INSTALL" = "true" ]; then
    log_phase "PHASE 3 — Skipped (SKIP_INSTALL=true)"
    return
  fi

  log_phase "PHASE 3 — Installing ratelimit service"

  # Deploy Redis if not already present
  if ! kubectl get deployment redis -n "$NAMESPACE" &>/dev/null; then
    log_info "Redis not found — deploying..."
    kubectl apply -f "$FIXTURES_DIR/redis.yaml" -n "$NAMESPACE"
    kubectl rollout status deployment/redis -n "$NAMESPACE" --timeout=120s
    log_ok "Redis deployed"
  else
    log_info "Redis already present — skipping"
  fi

  # Install / upgrade ratelimit via Helm
  # EnvoyFilter for ratelimit is included in the chart (envoyFilter.enabled=true).
  log_info "Deploying ratelimit via Helm (release: $HELM_RELEASE)..."
  helm upgrade --install "$HELM_RELEASE" "$HELM_CHART" \
    --namespace "$NAMESPACE" \
    --set namespace="$NAMESPACE" \
    --set image.repository="$HELM_IMAGE_REPO" \
    --set image.tag="$HELM_IMAGE_TAG" \
    --set config.redis.addr="$REDIS_ADDR" \
    --set monitoring.enabled=false \
    --set monitoring.grafanaDashboard.enabled=false \
    --wait --timeout=120s
  log_ok "Ratelimit deployed"

  # Restart gateway so all EnvoyFilters take effect
  log_info "Restarting Istio gateway to activate EnvoyFilters..."
  kubectl rollout restart deployment/public-gateway-istio -n "$NAMESPACE"
  kubectl rollout status  deployment/public-gateway-istio -n "$NAMESPACE" --timeout=120s
  log_ok "Gateway restarted — ratelimit is now active"
}

# ── Phase 4: all test scenarios ───────────────────────────────────────────────

phase_scenarios() {
  log_phase "PHASE 4 — Running all test scenarios"

  run_test "1. Show Current Rules" "get-rules.sh" ""

  apply_config "$FIXTURES_DIR/ratelimit-config-priority.yaml"
  run_test "2. Add Rules with Priorities"  "add-rules-with-priority.sh" ""
  run_test "3. Priority Test (Admin/VIP/Normal)" "priority-test.sh" ""
  run_test "4. Gateway Distribution (200 req)"   "gateway-distribution.sh" "200"
  delete_config "k6-priority-rules"

  apply_config "$FIXTURES_DIR/ratelimit-config-accuracy.yaml"
  # Config: 2 req/s limit, 10 requests sent → expected rejection rate:
  log_info "Accuracy test — expected rejection: $(calc_expected_rejection 2 10 1) (2 req/s, 10 req, 1s window)"
  run_test "5. Rate Limit Accuracy Test" "accuracy-test.sh" ""
  delete_config "k6-accuracy-test"

  apply_config "$FIXTURES_DIR/ratelimit-config-algo-compare.yaml"
  run_test "6. Algorithm Comparison (200 req)" "algorithm-compare.sh" ""
  delete_config "k6-fixed-test"
  delete_config "k6-sliding-test"

  apply_config "$FIXTURES_DIR/ratelimit-config-loadtest.yaml"
  run_k6_test "7. K6 Load Test (constant 50 req/s + spike)" "k6-load-test.js" ""
  delete_config "ratelimit-config-loadtest"

  run_k6_test "8. K6 Burst Test (spike to 500 req/s)" "k6-burst-test.js" ""

  log_ok "All test scenarios complete"
}

# ── Phase 5: final load test (with ratelimit) ─────────────────────────────────

phase_final_load() {
  log_phase "PHASE 5 — Final load test (WITH ratelimit)"
  log_info "50 req/s × 30s — same parameters as baseline, ratelimit now enforcing"

  apply_config "$FIXTURES_DIR/ratelimit-config-loadtest.yaml"

  # Use a standalone run that only captures the constant_load phase metrics
  # by running k6-baseline-test.js (same scenario) but now against the
  # rate-limited gateway. This gives an apples-to-apples latency comparison.
  run_k6_test "Final constant load (50 req/s × 30s)" "k6-baseline-test.js" "$FINAL_SUMMARY" "ratelimit active"

  delete_config "ratelimit-config-loadtest"

  if [ -s "$FINAL_SUMMARY" ]; then
    log_ok "Final summary saved → $FINAL_SUMMARY"
  fi
}

# ── Phase 6: comparison report ────────────────────────────────────────────────

phase_report() {
  log_phase "PHASE 6 — Generating comparison report"

  python3 - "$BASELINE_SUMMARY" "$FINAL_SUMMARY" "$REPORT_FILE" <<'PYEOF'
import json, sys, os
from datetime import datetime

baseline_file, final_file, report_file = sys.argv[1], sys.argv[2], sys.argv[3]

def load_metrics(path):
    if not os.path.exists(path) or os.path.getsize(path) == 0:
        return None
    try:
        with open(path) as f:
            return json.load(f)
    except Exception:
        return None

def dig(d, *keys, default=None):
    """Safe nested dict getter."""
    for k in keys:
        if not isinstance(d, dict):
            return default
        d = d.get(k)
        if d is None:
            return default
    return d if d is not None else default

def extract(data):
    if data is None:
        return {}
    m = data.get('metrics', {})
    # NOTE: k6's --summary-export JSON puts stats directly under the metric
    # name (e.g. metrics.http_reqs.count) — there is no extra "values" level.
    return {
        'total':     dig(m, 'http_reqs',        'count',  default=None),
        'rate':      dig(m, 'http_reqs',        'rate',   default=None),
        'avg_ms':    dig(m, 'http_req_duration','avg',    default=None),
        'p95_ms':    dig(m, 'http_req_duration','p(95)',  default=None),
        'med_ms':    dig(m, 'http_req_duration','med',    default=None),
        'min_ms':    dig(m, 'http_req_duration','min',    default=None),
        'max_ms':    dig(m, 'http_req_duration','max',    default=None),
        'fail_rate': dig(m, 'http_req_failed',  'value',  default=None),
    }

b = extract(load_metrics(baseline_file))
f = extract(load_metrics(final_file))

def fmt_ms(v):     return f"{v:.2f} ms"     if v is not None else "—"
def fmt_pct(v):    return f"{v*100:.2f}%"   if v is not None else "—"
def fmt_int(v):    return f"{int(v):,}"     if v is not None else "—"
def fmt_rps(v):    return f"{v:.1f} req/s"  if v is not None else "—"

def delta(bv, fv, lower_is_better=True):
    if bv is None or fv is None or bv == 0:
        return ""
    change = (fv - bv) / bv * 100
    sign  = "+" if change >= 0 else ""
    arrow = "▲" if change > 0 else "▼"
    worse = (change > 0) == lower_is_better
    badge = "🔴" if (worse and abs(change) > 10) else ("🟡" if (worse and abs(change) > 2) else "🟢")
    return f"{badge} {arrow}{sign}{change:.1f}%"

lines = [
    "## Rate Limit Smoke Test — Baseline vs. Final Comparison",
    "",
    f"> **Generated:** {datetime.utcnow().strftime('%Y-%m-%d %H:%M UTC')}",
    "",
    "### Test configuration",
    "",
    "Both load tests used identical parameters: **50 req/s constant arrival rate, 30 s, 20 rotating VUs**.",
    "",
    "| | Baseline | Final |",
    "|---|---|---|",
    "| Rate limiting | ❌ None | ✅ Active (50 req/s path, 10 req/s per user) |",
    "| Gateway | Istio only | Istio + EnvoyFilter + ratelimit|",
    "",
    "### Results",
    "",
    "| Metric | Baseline | Final | Δ |",
    "|--------|----------|-------|---|",
    f"| Total requests  | {fmt_int(b.get('total'))}  | {fmt_int(f.get('total'))}  | {delta(b.get('total'), f.get('total'), lower_is_better=False)} |",
    f"| Throughput      | {fmt_rps(b.get('rate'))}   | {fmt_rps(f.get('rate'))}   | {delta(b.get('rate'),  f.get('rate'),  lower_is_better=False)} |",
    f"| Avg latency     | {fmt_ms(b.get('avg_ms'))}  | {fmt_ms(f.get('avg_ms'))}  | {delta(b.get('avg_ms'), f.get('avg_ms'))} |",
    f"| Median latency  | {fmt_ms(b.get('med_ms'))}  | {fmt_ms(f.get('med_ms'))}  | {delta(b.get('med_ms'), f.get('med_ms'))} |",
    f"| p95 latency     | {fmt_ms(b.get('p95_ms'))}  | {fmt_ms(f.get('p95_ms'))}  | {delta(b.get('p95_ms'), f.get('p95_ms'))} |",
    f"| Min latency     | {fmt_ms(b.get('min_ms'))}  | {fmt_ms(f.get('min_ms'))}  | {delta(b.get('min_ms'), f.get('min_ms'))} |",
    f"| Max latency     | {fmt_ms(b.get('max_ms'))}  | {fmt_ms(f.get('max_ms'))}  | {delta(b.get('max_ms'), f.get('max_ms'))} |",
    f"| Failure rate    | {fmt_pct(b.get('fail_rate'))} | {fmt_pct(f.get('fail_rate'))} | {delta(b.get('fail_rate'), f.get('fail_rate'))} |",
    "",
    "> 🟢 improvement or negligible change &nbsp;&nbsp;🟡 minor regression (2–10%) &nbsp;&nbsp;🔴 notable regression (>10%)",
    "",
    "### Interpretation",
    "",
]

# Auto-generated narrative
if b and f:
    avg_b = b.get('avg_ms') or 0
    avg_f = f.get('avg_ms') or 0
    overhead = avg_f - avg_b

    if overhead < 1:
        lines.append(f"- **Latency overhead of ratelimit**: +{overhead:.2f} ms avg — negligible (Redis RTT within noise)")
    elif overhead < 5:
        lines.append(f"- **Latency overhead of ratelimit**: +{overhead:.2f} ms avg — low, acceptable for production")
    else:
        lines.append(f"- **Latency overhead of ratelimit**: +{overhead:.2f} ms avg — investigate Redis RTT or gRPC filter latency")

    fail_f = (f.get('fail_rate') or 0) * 100
    fail_b = (b.get('fail_rate') or 0) * 100
    if fail_f < 2:
        lines.append(f"- **Rate enforcement (constant load)**: ✅ {fail_f:.2f}% rejections — config correctly permits 50 req/s")
    else:
        lines.append(f"- **Rate enforcement (constant load)**: ⚠️ {fail_f:.2f}% rejections — "
                     "check whether loadtest config was applied or limit is set below 50 req/s")

    if fail_b > 2:
        lines.append(f"- **Baseline failure rate {fail_b:.2f}%**: unexpected — gateway may have returned errors without ratelimit")
    else:
        lines.append(f"- **Baseline failure rate**: {fail_b:.2f}% — as expected (no rejections without ratelimit)")

elif not b:
    lines.append("- ⚠️ Baseline metrics not available — run the script again with ratelimit NOT installed for Phase 1.")
elif not f:
    lines.append("- ⚠️ Final metrics not available — Phase 4 may have failed.")

lines += [
    "",
    "---",
    "_Report generated by `tests/smoke/run-integration-test.sh`_",
]

report = "\n".join(lines)
with open(report_file, "w") as fh:
    fh.write(report)

print(report)
PYEOF

  echo ""
  log_ok "Report written → $REPORT_FILE"

}

# ── cleanup ───────────────────────────────────────────────────────────────────

cleanup() {
  log_info "Removing test pods..."
  kubectl delete pod "$K6_POD"   -n "$NAMESPACE" --ignore-not-found || true
  kubectl delete pod "$CURL_POD" -n "$NAMESPACE" --ignore-not-found || true
}

trap cleanup EXIT

# ── main ──────────────────────────────────────────────────────────────────────

echo -e "${BLUE}"
echo "╔══════════════════════════════════════════════════════╗"
echo "║   RATELIMIT SMOKE TEST WITH BASELINE COMPARISON     ║"
echo "╚══════════════════════════════════════════════════════╝"
echo -e "${NC}"
echo "Namespace    : $NAMESPACE"
echo "Helm release : $HELM_RELEASE"
echo "Helm chart   : $HELM_CHART"
echo "Skip install : $SKIP_INSTALL"
echo "Results dir  : $RESULTS_DIR"
echo ""

phase_setup_pods
phase_setup_backend
phase_baseline
phase_install
phase_scenarios
phase_final_load
phase_report

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║       ALL PHASES COMPLETED SUCCESSFULLY              ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Results : $RESULTS_DIR"
echo "Report  : $REPORT_FILE"
