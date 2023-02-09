#!/bin/bash
set -euo pipefail

# to be consistent, delete the pre-Helm manifests first
if [ -n "$(ls $DIR/pre-manifests)" ]; then
  echo "Pre-apply static manifests found:"
  ls "$DIR/pre-manifests"
  kubectl delete -f "$DIR/pre-manifests/" -n "$K8S_NAMESPACE"
fi

# will error if the target release doesn't exist, rather than fail silently
# --timeout default-value is already 5m, but set explicitly
# omit double-quotes around DEBUG_FLAG so that it's ignored when empty
helm uninstall $DEBUG_FLAG \
  -n="${K8S_NAMESPACE}" \
  --timeout=5m \
  --wait \
  "${K8S_RELEASE_NAME}${K8S_RESOURCE_SUFFIX}"

# delete the post-Helm manifests after the Helm delete
if [ -n "$(ls $DIR/post-manifests)" ]; then
  echo "Post-apply static manifests found:"
  ls "$DIR/post-manifests"
  kubectl delete -f "$DIR/post-manifests/" -n "$K8S_NAMESPACE"
fi
