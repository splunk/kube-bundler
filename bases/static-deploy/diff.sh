#!/bin/bash
set -euxo pipefail

k8s_namespace=$(jq -r .namespace < $CONFIG_JSON)
$DIR/interpolate.sh

# Print the diff. Ignore any failures since diff returns nonzero when any changes are present.
kubectl diff -f $DIR/manifests/ -n $k8s_namespace || true
