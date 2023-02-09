#!/bin/bash
set -euo pipefail

k8s_namespace=$(jq -r .namespace < $CONFIG_JSON)

# TODO: Implement functionality to wait for the service to deploy
