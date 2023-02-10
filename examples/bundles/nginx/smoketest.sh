#!/bin/bash
set -euxo pipefail

source env.sh

export k8s_namespace=$(jq -r .namespace < $CONFIG_JSON)
K8S_PORT=$(jq -r .port < $CONFIG_JSON)
K8S_RESOURCE_SUFFIX=$(jq -r 'select(.suffix != null and .suffix != "") | "-" + .suffix' < $CONFIG_JSON)
TARGETS=http://nginx-deployment${K8S_RESOURCE_SUFFIX}.${k8s_namespace}:${K8S_PORT} waitfor
