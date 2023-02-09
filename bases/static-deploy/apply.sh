#!/bin/bash
set -euo pipefail

debug=$(jq -r .debug < $CONFIG_JSON)

if [[ "$debug" = "true" ]]; then
  set -x
fi

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

# Materialize the manifests
$DIR/interpolate.sh

# Create the service
kubectl apply -f $DIR/manifests/ -n $k8s_namespace
