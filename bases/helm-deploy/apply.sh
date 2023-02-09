#!/bin/bash
set -euo pipefail

# Create namespace if it doesn't exist. Adds a label for network policy support
ns=$(cat <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: "$K8S_NAMESPACE"
  labels:
    name: "$K8S_NAMESPACE"
EOF
)

# Create the namespace. Note: $ns must be quoted to preserve newlines from the heredoc above
echo "$ns" | kubectl apply -f -

# fill in overrides.yaml.tmpl to create overrides.yaml, and materialize any static templates
"$DIR/interpolate.sh"

echo "Deleting secrets for ${K8S_RELEASE_NAME} if created previously"
kubectl -n "${K8S_NAMESPACE}" delete secrets -l name="${K8S_RELEASE_NAME}",owner=helm --ignore-not-found

# apply any static manifests (need to explicitly skip if there are no <.json, .yaml, .yml> files)
# first do the pre-manifests, meant to be applied before the Helm chart (e.g. secrets to be mounted, etc.)
# note that .keep is hidden and will not count against the emptiness check
if [ -n "$(ls $DIR/pre-manifests)" ]; then
  echo "Pre-apply static manifests found:"
  ls "$DIR/pre-manifests"
  kubectl apply -f "$DIR/pre-manifests/" -n "$K8S_NAMESPACE"
fi

# Docs: https://helm.sh/docs/helm/helm_upgrade/
# idempotent - install if not present, otherwise upgrade
# --wait until all Pods, PVCs, Services, and min number of Pods of a Deployment/StatefulSet are Ready
# note that overrides.yaml could be empty (will not error, as long as it exists)
# omit double-quotes around DEBUG_FLAG so that it's ignored when empty
# otherwise, 'helm upgrade' will complain that it needs two arguments
helm upgrade --install $DEBUG_FLAG \
  -n="${K8S_NAMESPACE}" \
  --timeout=5m \
  --wait \
  --wait-for-jobs \
  --values "$DIR/overrides.yaml" \
  "${K8S_RELEASE_NAME}${K8S_RESOURCE_SUFFIX}" \
  "$DIR/helm-charts/$K8S_RELEASE_NAME"

# after the Helm apply, do the post-manifests (e.g. jobs to be run after the main service is up and running)
if [ -n "$(ls $DIR/post-manifests)" ]; then
  echo "Post-apply static manifests found:"
  ls "$DIR/post-manifests"
  kubectl apply -f "$DIR/post-manifests/" -n "$K8S_NAMESPACE"
fi
