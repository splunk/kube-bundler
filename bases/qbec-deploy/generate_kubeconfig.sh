#!/bin/bash
set -euo pipefail

server="https://kubernetes.default"
ca="/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
token=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
namespace=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)

cat <<EOF > /kubeconfig
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: ${ca}
    server: ${server}
  name: default
contexts:
- context:
    cluster: default
    namespace: ${namespace}
    user: default
  name: default
current-context: default
users:
- name: default
  user:
    token: ${token}
EOF
