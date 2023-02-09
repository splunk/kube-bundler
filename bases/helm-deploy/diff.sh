#!/bin/bash
set -euxo pipefail

# fill in overrides.yaml.tmpl to create overrides.yaml
"$DIR/interpolate.sh"

helm diff upgrade \
  -n="${K8S_NAMESPACE}" \
  --values "$DIR/overrides.yaml" \
  "${K8S_RELEASE_NAME}${K8S_RESOURCE_SUFFIX}" \
  "./helm-charts/$K8S_RELEASE_NAME"
