#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="shift-cluster"
COUNTER_IMAGE="shiftpod/counter:latest"
MANAGER_IMAGE="shiftpod/manager:latest"
NAMESPACE="default"
CHECKPOINT_DIR="/var/lib/shiftpod/checkpoints"

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

wait_for_pods() {
    local selector=$1
    local timeout=${2:-60}
    local namespace=${3:-default}

    log_info "Waiting for pods with selector '$selector' to be ready..."
    kubectl wait --for=condition=ready pod -l "$selector" -n "$namespace" --timeout="${timeout}s"
}

wait_for_deployment() {
    local deployment=$1
    local namespace=${2:-default}
    local timeout=${3:-60}

    log_info "Waiting for deployment '$deployment' to be ready..."
    kubectl rollout status deployment "$deployment" -n "$namespace" --timeout="${timeout}s"
}

check_manager_health() {
    log_info "Checking manager health..."
    local manager_pods=$(kubectl get pods -n kube-system -l app=shiftpod-manager --no-headers | wc -l)

    if [ "$manager_pods" -eq 0 ]; then
        log_error "No manager pods found"
        return 1
    fi

    log_info "Found $manager_pods manager pod(s)"
    kubectl get pods -n kube-system -l app=shiftpod-manager

    # Check if Unix socket exists
    kubectl exec -n kube-system daemonset/shiftpod-manager -- test -S /var/run/shiftpod/manager.sock
    if [ $? -eq 0 ]; then
        log_info "Manager Unix socket is available"
    else
        log_error "Manager Unix socket not found"
        return 1
    fi
}

test_checkpoint_creation() {
    log_info "=== Testing Checkpoint Creation ==="

    # Deploy counter application
    log_info "Deploying counter application..."
    kubectl apply -f k3s/counter/deployment.yaml
    wait_for_deployment "counter" "$NAMESPACE" 120

    # Get pod name and container ID
    local pod_name=$(kubectl get pods -l app=counter -o jsonpath='{.items[0].metadata.name}')
    log_info "Counter pod: $pod_name"

    # Let it run for a bit to generate some state
    log_info "Waiting for application to generate state..."
    sleep 30

    # Check application logs
    log_info "Counter application logs:"
    kubectl logs deploy/counter --tail=10

    # Restart the pod to trigger checkpoint creation
    log_info "Restarting pod to trigger checkpoint creation..."
    kubectl delete pod "$pod_name"
    wait_for_deployment "counter" "$NAMESPACE" 120

    # Wait for checkpoint creation
    sleep 10

    # Check for ContainerMigration CRDs
    log_info "Checking for ContainerMigration CRDs..."
    local migration_count=$(kubectl get containermigrations -A --no-headers 2>/dev/null | wc -l)

    if [ "$migration_count" -gt 0 ]; then
        log_info "✓ Found $migration_count ContainerMigration(s)"
        kubectl get containermigrations -A
    else
        log_warn "No ContainerMigration CRDs found yet"
    fi

    # Check manager logs for checkpoint notifications
    log_info "Manager logs (checkpoint creation):"
    kubectl logs daemonset/shiftpod-manager -n kube-system --tail=20 | grep -i checkpoint || true

    # Check shim logs for checkpoint activity
    log_info "Shim logs:"
    docker exec k3d-${CLUSTER_NAME}-agent-0 cat /tmp/shiftpod/shim.log | tail -20 | grep -i checkpoint || true
}

test_checkpoint_restoration() {
    log_info "=== Testing Checkpoint Restoration ==="

    # Scale down to 0 to clear running pods
    log_info "Scaling deployment to 0..."
    kubectl scale deployment counter --replicas=0
    kubectl wait --for=delete pod -l app=counter --timeout=60s

    # Wait a moment
    sleep 5

    # Scale back up to trigger restoration
    log_info "Scaling deployment back to 1 (should restore from checkpoint)..."
    kubectl scale deployment counter --replicas=1
    wait_for_deployment "counter" "$NAMESPACE" 120

    # Check if the application restored its state
    log_info "Counter application logs after restoration:"
    kubectl logs deploy/counter --tail=20

    # Check manager logs for restoration activity
    log_info "Manager logs (restoration):"
    kubectl logs daemonset/shiftpod-manager -n kube-system --tail=20 | grep -i restore || true

    # Check shim logs for restoration activity
    log_info "Shim logs (restoration):"
    docker exec k3d-${CLUSTER_NAME}-agent-0 cat /tmp/shiftpod/shim.log | tail -20 | grep -i restore || true

    # Check updated ContainerMigration status
    log_info "ContainerMigration status after restoration:"
    kubectl get containermigrations -A -o wide
}

test_cross_node_migration() {
    log_info "=== Testing Cross-Node Migration ==="

    # Check if we have multiple nodes
    local node_count=$(kubectl get nodes --no-headers | wc -l)

    if [ "$node_count" -lt 2 ]; then
        log_warn "Skipping cross-node migration test (only $node_count node(s) available)"
        return 0
    fi

    log_info "Found $node_count nodes, testing cross-node migration..."

    # Get current pod node
    local current_pod=$(kubectl get pods -l app=counter -o jsonpath='{.items[0].metadata.name}')
    local current_node=$(kubectl get pod "$current_pod" -o jsonpath='{.spec.nodeName}')
    log_info "Current pod '$current_pod' is on node '$current_node'"

    # Get other nodes
    local other_nodes=$(kubectl get nodes --no-headers -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | grep -v "$current_node" | head -1)

    if [ -z "$other_nodes" ]; then
        log_warn "No other nodes available for cross-node migration test"
        return 0
    fi

    local target_node="$other_nodes"
    log_info "Target node for migration: '$target_node'"

    # First create a checkpoint by killing current pod
    log_info "Creating checkpoint by killing current pod..."
    kubectl delete pod "$current_pod"

    # Wait for pod to be deleted and new one created
    sleep 10
    wait_for_deployment "counter" "$NAMESPACE" 60

    # Now force migration by adding node affinity
    log_info "Adding node affinity to force pod migration to '$target_node'..."
    kubectl patch deployment counter --type='merge' -p='
{
  "spec": {
    "template": {
      "spec": {
        "affinity": {
          "nodeAffinity": {
            "requiredDuringSchedulingIgnoredDuringExecution": {
              "nodeSelectorTerms": [{
                "matchExpressions": [{
                  "key": "kubernetes.io/hostname",
                  "operator": "In",
                  "values": ["'$target_node'"]
                }]
              }]
            }
          }
        }
      }
    }
  }
}'

    # Delete the pod again to trigger recreation on target node with migration
    local new_current_pod=$(kubectl get pods -l app=counter -o jsonpath='{.items[0].metadata.name}')
    kubectl delete pod "$new_current_pod"
    wait_for_deployment "counter" "$NAMESPACE" 120

    # Verify pod is now on target node
    local new_pod=$(kubectl get pods -l app=counter -o jsonpath='{.items[0].metadata.name}')
    local new_node=$(kubectl get pod "$new_pod" -o jsonpath='{.spec.nodeName}')

    if [ "$new_node" = "$target_node" ]; then
        log_info "✓ Pod successfully migrated to '$target_node'"
    else
        log_warn "Pod migration may not have worked as expected (pod is on '$new_node', expected '$target_node')"
    fi

    # Check if checkpoint was downloaded and restored
    log_info "Checking cross-node migration logs..."
    kubectl logs deploy/counter --tail=20

    # Check ContainerMigration CRDs for cross-node activity
    log_info "ContainerMigration CRDs after cross-node migration:"
    kubectl get containermigrations -A -o wide

    # Check manager logs on both nodes
    log_info "Manager logs from all nodes:"
    kubectl logs daemonset/shiftpod-manager -n kube-system --all-containers=true --tail=30 | grep -i -E "(download|migrate|pull)" || true
}

cleanup() {
    log_info "=== Cleanup ==="

    # Delete counter deployment
    kubectl delete deployment counter --ignore-not-found=true

    # Delete ContainerMigration CRDs
    kubectl delete containermigrations --all -A --ignore-not-found=true

    # Clean up any remaining pods
    kubectl delete pods -l app=counter --ignore-not-found=true
}

show_summary() {
    log_info "=== Integration Test Summary ==="

    echo "Cluster Status:"
    kubectl get nodes
    echo

    echo "Manager Status:"
    kubectl get pods -n kube-system -l app=shiftpod-manager
    echo

    echo "ContainerMigration CRDs:"
    kubectl get containermigrations -A || echo "No migrations found"
    echo

    echo "Recent Manager Logs:"
    kubectl logs daemonset/shiftpod-manager -n kube-system --tail=10 || echo "No manager logs"
    echo

    echo "Recent Shim Logs:"
    docker exec k3d-${CLUSTER_NAME}-agent-0 cat /tmp/shiftpod/shim.log 2>/dev/null | tail -10 || echo "No shim logs"
}

main() {
    log_info "Starting Shiftpod Integration Tests"
    log_info "Cluster: $CLUSTER_NAME"
    log_info "Counter Image: $COUNTER_IMAGE"
    log_info "Manager Image: $MANAGER_IMAGE"

    # Check prerequisites
    log_info "=== Checking Prerequisites ==="

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found"
        exit 1
    fi

    if ! command -v docker &> /dev/null; then
        log_error "docker not found"
        exit 1
    fi

    # Check if cluster exists
    if ! k3d cluster list | grep -q "$CLUSTER_NAME"; then
        log_error "Cluster '$CLUSTER_NAME' not found. Please create it first with 'task k3d:create'"
        exit 1
    fi

    # Set kubectl context
    kubectl config use-context k3d-$CLUSTER_NAME

    # Check manager health
    if ! check_manager_health; then
        log_error "Manager health check failed"
        exit 1
    fi

    # Clean up any existing state
    cleanup

    # Run tests
    test_checkpoint_creation
    sleep 5
    test_checkpoint_restoration
    sleep 5
    test_cross_node_migration

    # Show summary
    show_summary

    log_info "✓ Integration tests completed"
}

# Handle script termination
trap cleanup EXIT

# Allow running specific test functions
if [ $# -gt 0 ]; then
    case "$1" in
        "checkpoint")
            check_manager_health && test_checkpoint_creation
            ;;
        "restore")
            check_manager_health && test_checkpoint_restoration
            ;;
        "migrate")
            check_manager_health && test_cross_node_migration
            ;;
        "cleanup")
            cleanup
            ;;
        "summary")
            show_summary
            ;;
        "force-migrate")
            log_info "=== Force Cross-Node Migration Test ==="
            check_manager_health
            kubectl apply -f k3s/counter/deployment.yaml
            wait_for_deployment "counter" "$NAMESPACE" 120
            sleep 30  # Let it run to build state
            test_cross_node_migration
            ;;
        *)
            echo "Usage: $0 [checkpoint|restore|migrate|cleanup|summary|force-migrate]"
            echo "  checkpoint     - Test checkpoint creation"
            echo "  restore        - Test checkpoint restoration"
            echo "  migrate        - Test cross-node migration"
            echo "  force-migrate  - Force cross-node migration with counter app"
            echo "  cleanup        - Clean up test resources"
            echo "  summary        - Show test summary"
            echo "  (no args)      - Run all tests"
            exit 1
            ;;
    esac
else
    main
fi
