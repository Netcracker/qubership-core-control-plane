#!/usr/bin/env bash
# tests/scripts/port-forward.sh
#
# Manages kubectl port-forwards for tests.
#
# Usage:
#   port-forward.sh --profile=<local|cloud> --start
#   port-forward.sh --profile=<local|cloud> --stop
#
# Profiles:
#   local  — Redis on 6379 (for tests/e2e/*).
#   cloud  — Gateway 8080, Operator 8082, Redis 6379, Metrics 9090
#            (for tests/cloud-e2e/*).
#
# ENV:
#   NAMESPACE  — k8s namespace. Default: core-1-core.
#   PF_PID_DIR — where to store PID files. Default: /tmp.
#   PF_WAIT    — seconds to wait for port-forward readiness. Default: 5.

set -euo pipefail

NAMESPACE="${NAMESPACE:-core-1-core}"
PF_PID_DIR="${PF_PID_DIR:-/tmp}"
PF_WAIT="${PF_WAIT:-5}"

PROFILE=""
ACTION=""

for arg in "$@"; do
    case "$arg" in
        --profile=*) PROFILE="${arg#*=}" ;;
        --start)     ACTION="start" ;;
        --stop)      ACTION="stop" ;;
        *)
            echo "Unknown argument: $arg" >&2
            exit 2
            ;;
    esac
done

if [[ -z "$PROFILE" || -z "$ACTION" ]]; then
    echo "Usage: $0 --profile=<local|cloud> --start|--stop" >&2
    exit 2
fi

# List of forwards for each profile.
# Format: "<name>:<resource>:<port>"
case "$PROFILE" in
    local)
        FORWARDS=(
            "redis:service/redis:6379"
        )
        ;;
    cloud)
        FORWARDS=(
            "gateway:svc/public-gateway-istio:8080"
            "operator:svc/ratelimit-service:8082"
            "redis:svc/redis:6379"
            "metrics:svc/ratelimit-service:9090"
        )
        ;;
    *)
        echo "Unknown profile: $PROFILE (expected: local|cloud)" >&2
        exit 2
        ;;
esac

start_one() {
    local name="$1" resource="$2" port="$3"
    local pid_file="$PF_PID_DIR/pf_${PROFILE}_${name}.pid"
    local log_file="$PF_PID_DIR/pf_${PROFILE}_${name}.log"

    if [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
        echo "  port-forward $name already running (pid=$(cat "$pid_file"))"
        return 0
    fi

    kubectl port-forward -n "$NAMESPACE" "$resource" "${port}:${port}" \
        > "$log_file" 2>&1 &
    echo $! > "$pid_file"
    echo "  started $name -> ${port}:${port} (pid=$(cat "$pid_file"))"
}

stop_one() {
    local name="$1"
    local pid_file="$PF_PID_DIR/pf_${PROFILE}_${name}.pid"

    if [[ -f "$pid_file" ]]; then
        local pid
        pid="$(cat "$pid_file")"
        if kill "$pid" 2>/dev/null; then
            echo "  stopped $name (pid=$pid)"
        fi
        rm -f "$pid_file"
    fi
}

case "$ACTION" in
    start)
        echo "Starting port-forwards (profile=$PROFILE, namespace=$NAMESPACE)..."
        for fw in "${FORWARDS[@]}"; do
            IFS=':' read -r name resource port <<< "$fw"
            start_one "$name" "$resource" "$port"
        done
        sleep "$PF_WAIT"
        echo "Port-forwards ready."
        ;;
    stop)
        echo "Stopping port-forwards (profile=$PROFILE)..."
        for fw in "${FORWARDS[@]}"; do
            IFS=':' read -r name _ _ <<< "$fw"
            stop_one "$name"
        done
        echo "Port-forwards stopped."
        ;;
esac
