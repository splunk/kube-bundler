#!/bin/bash
set -euo pipefail

k8s_namespace=$(jq -r .namespace < $CONFIG_JSON)

# apply.sh already waits before marking success
# optionally add a 'kubectl status' command here if needed
echo "Waiting already done during apply-time, skipping"
