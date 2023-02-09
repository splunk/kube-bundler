#!/bin/bash
set -euo pipefail

debug=$(jq -r .debug < $CONFIG_JSON)

# only strictly need to interpolate for overrides.yaml; Helm will take care of the rest
envsubst -no-unset < "$DIR/overrides.yaml.tmpl" > "$DIR/overrides.yaml"

if [ "$debug" = "true" ]; then
  echo "Showing interpolated overrides.yaml:"
  cat "$DIR/overrides.yaml"
fi

# also interpolate any static YAML (no-op if empty)
# do both the pre-Helm-apply templates, and the post-Helm-apply templates
shopt -s nullglob
for f in $DIR/pre-templates/*; do
  # Skip if no matching files are found under pre-templates/
  [ -f "$f" ] || continue

  envsubst -no-unset < $f > "$DIR/pre-manifests/$(basename $f)"
  if [ "$debug" = "true" ]; then
    echo "Interpolating $DIR/pre-manifests/$(basename $f)"
    cat "$DIR/pre-manifests/$(basename $f)"
  fi
done

for f in $DIR/post-templates/*; do
  # Skip if no matching files are found under post-templates/
  [ -f "$f" ] || continue

  envsubst -no-unset < $f > "$DIR/post-manifests/$(basename $f)"
  if [ "$debug" = "true" ]; then
    echo "Interpolating $DIR/post-manifests/$(basename $f)"
    cat "$DIR/post-manifests/$(basename $f)"
  fi
done

# unused by anything, but can be used for debugging
# use 'helm template' instead of 'helm upgrade --install --dry-run' to avoid requiring a K8s server
# in particular, gets around the issue with CRDs not existing yet for initial installs
# see: https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations
helm template $DEBUG_FLAG \
  -n="${K8S_NAMESPACE}" \
  --include-crds \
  --values "$DIR/overrides.yaml" \
  "${K8S_RELEASE_NAME}${K8S_RESOURCE_SUFFIX}" \
  "$DIR/helm-charts/$K8S_RELEASE_NAME" > "$DIR/dry-run-output/${K8S_RELEASE_NAME}${K8S_RESOURCE_SUFFIX}.txt"
