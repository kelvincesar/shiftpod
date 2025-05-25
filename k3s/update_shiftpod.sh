#!/bin/bash
set -e

BINARY=containerd-shim-shiftpod-v2

echo "[*] Updating shim binary..."
sudo mv $BINARY /usr/local/bin/

echo "[*] Deleting test pod"
kubectl delete pod test-pod-shim --ignore-not-found

echo "[*] Restarting k3s"
sudo systemctl restart k3s

echo "[*] Verifying binary installed:"
sha256sum /usr/local/bin/$BINARY

echo "[*] Waiting for cluster to be ready"
sleep 5

echo "[*] Applying test pod"
kubectl apply -f testpod.yaml