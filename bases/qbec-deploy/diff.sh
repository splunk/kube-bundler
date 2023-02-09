#!/bin/bash
set -euxo pipefail

qbec_env=$(jq -r .env < $CONFIG_JSON)

# Generate the kubeconfig
$DIR/generate_kubeconfig.sh
export KUBECONFIG=/kubeconfig

# Print the diff
qbec diff $qbec_env --root $DIR
