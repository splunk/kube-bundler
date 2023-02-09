#!/bin/bash
set -euo pipefail

# See https://kubernetes.io/docs/tasks/run-application/access-api-from-pod/#without-using-a-proxy
APISERVER=https://kubernetes.default.svc
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
NAMESPACE=$(cat ${SERVICEACCOUNT}/namespace)
TOKEN=$(cat ${SERVICEACCOUNT}/token)
CACERT=${SERVICEACCOUNT}/ca.crt

cat <<EOF > kubeconfig
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: "$CACERT"
    server: "$APISERVER"
  name: default
contexts:
- context:
    cluster: default
    namespace: "$NAMESPACE"
    user: default
  name: default
current-context: default
users:
- name: default
  user:
    token: "$TOKEN"
EOF

chmod 600 kubeconfig
export KUBECONFIG=$(pwd)/kubeconfig
