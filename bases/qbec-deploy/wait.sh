#!/bin/bash
set -euo pipefail

k8s_namespace=$(jq -r .namespace < $CONFIG_JSON)

# Qbec already waits for objects to be ready upon apply.
# Add any additional functionality to wait for the service to deploy here
