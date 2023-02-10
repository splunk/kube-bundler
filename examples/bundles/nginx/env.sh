#!/bin/bash
set -euo pipefail

export K8S_REPLICAS=$(jq -r .replicas < $CONFIG_JSON)
if [ -z "$K8S_REPLICAS" ]; then
  export K8S_REPLICAS=$(jq -r .statelessReplicas < $FLAVOR_JSON)
fi
export K8S_PORT=$(jq -r .port < $CONFIG_JSON)
export K8S_DOCKER_REGISTRY=$(jq -r .dockerRegistry < $INSTALL_JSON)
export K8S_DOCKER_TAG=$(jq -r .docker_tag < $CONFIG_JSON)
export K8S_RESOURCE_SUFFIX=$(jq -r 'select(.suffix != null and .suffix != "") | "-" + .suffix' < $CONFIG_JSON)

