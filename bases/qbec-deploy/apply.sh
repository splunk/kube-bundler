#!/bin/bash
set -euo pipefail

qbec_env=$(jq -r .env < $CONFIG_JSON)
k8s_namespace=$(jq -r .namespace < $CONFIG_JSON)

# Create namespace if it doesn't exist. Adds a label for network policy support
ns=$(cat <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $k8s_namespace
  labels:
    name: $k8s_namespace
EOF
)

# Create the namespace. Note: $ns must be quoted to preserve newlines from the heredoc above
echo "$ns" | kubectl apply -f -

# Generate the kubeconfig
$DIR/generate_kubeconfig.sh
export KUBECONFIG=/kubeconfig

# Create the service
qbec apply $qbec_env --root $DIR --yes --wait
