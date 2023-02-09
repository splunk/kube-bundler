#!/bin/bash
set -euo pipefail

qbec_env=$(jq -r .env < $CONFIG_JSON)

# Generate the kubeconfig
$DIR/generate_kubeconfig.sh
export KUBECONFIG=/kubeconfig

#Delete the service
qbec delete $qbec_env --root $DIR --yes