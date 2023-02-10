#!/bin/bash
set -euo pipefail

k8s_namespace=$(jq -r .namespace < $CONFIG_JSON)
K8S_PORT=$(jq -r .port < $CONFIG_JSON)
K8S_RESOURCE_SUFFIX=$(jq -r 'select(.suffix != null and .suffix != "") | "-" + .suffix' < $CONFIG_JSON)

yaml=$(cat <<EOF
data:
  outputs.json: |-
    {
      "endpoint": "http://nginx-deployment${K8S_RESOURCE_SUFFIX}.${k8s_namespace}:${K8S_PORT}"
    }
EOF
)

echo "$yaml" > patch.yaml

kubectl patch configmap nginx${K8S_RESOURCE_SUFFIX}-config -p "$(cat patch.yaml)"
