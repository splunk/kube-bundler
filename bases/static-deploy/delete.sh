#!/bin/bash
set -euo pipefail

# Materialize the manifests
$DIR/interpolate.sh

k8s_namespace=$(jq -r .namespace < $CONFIG_JSON)
kubectl delete -f $DIR/manifests --ignore-not-found -n $k8s_namespace

# delete the post-Helm manifests after the Helm delete
if [ -n "$(ls $DIR/post-manifests)" ]; then
  echo "Post-apply static manifests found:"
  ls "$DIR/post-manifests"
  kubectl delete -f "$DIR/post-manifests/" --ignore-not-found -n "$k8s_namespace"
fi