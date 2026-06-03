#!/usr/bin/env bash
# tests/scripts/helm-validate.sh
#
# Validates the helm chart:
#   1. helm lint
#   2. helm template (default values, then a perUser override)
#   3. kubeconform (if available) — strict mode
#   4. YAML syntax sanity (python3 yaml.safe_load_all)
#
# All steps are run; if any fails, exit non-zero with a clear marker.

set -euo pipefail

CHART_DIR="${1:-helm-charts}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "=== 1) helm lint ==="
helm lint "$CHART_DIR"

echo ""
echo "=== 2) helm template (default values) ==="
DEFAULT_RENDER="$TMP_DIR/default.yaml"
helm template "$CHART_DIR" > "$DEFAULT_RENDER"
echo "rendered $(wc -l < "$DEFAULT_RENDER") lines to $DEFAULT_RENDER"

echo ""
echo "=== 3) helm template (with extra perUser rule) ==="
PERUSER_VALUES="$TMP_DIR/peruser-values.yaml"
cat > "$PERUSER_VALUES" <<'EOF'
config:
  rateLimits:
    rules:
      - name: layered_demo
        pattern: "/demo"
        limit: 1000
        window: "minute"
        algorithm: fixed_window
        priority: 50
        perUser:
          limit: 10
          window: "minute"
          algorithm: fixed_window
          priority: 100
EOF
PERUSER_RENDER="$TMP_DIR/peruser.yaml"
helm template "$CHART_DIR" -f "$PERUSER_VALUES" > "$PERUSER_RENDER"
echo "rendered with perUser to $PERUSER_RENDER"

# Sanity check: nested descriptors block must appear when perUser is set.
if ! grep -q 'descriptors:' "$PERUSER_RENDER"; then
    echo "FAIL: perUser override did not produce nested descriptors block"
    exit 1
fi
echo "perUser → nested descriptors confirmed"

echo ""
if command -v kubeconform > /dev/null 2>&1; then
    echo "=== 4) kubeconform (strict) ==="
    kubeconform -strict -ignore-missing-schemas \
        -summary "$DEFAULT_RENDER" "$PERUSER_RENDER"
else
    echo "=== 4) kubeconform not installed — skipping (recommend: brew/apt install kubeconform) ==="
fi

echo ""
echo "=== 5) YAML syntax sanity ==="
if command -v python3 > /dev/null 2>&1; then
    python3 -c "
import sys, yaml
for path in ['$DEFAULT_RENDER', '$PERUSER_RENDER']:
    docs = list(yaml.safe_load_all(open(path)))
    print(f'{path}: {len(docs)} documents parsed OK')
"
else
    echo "python3 not available — skipping yaml syntax sanity"
fi

echo ""
echo "PASS"
