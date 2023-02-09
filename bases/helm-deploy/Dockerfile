# Helm 3.10 supports K8s 1.22.x - 1.25.x
# See versioning: https://helm.sh/docs/topics/version_skew/#supported-version-skew
# Helm release notes: https://github.com/helm/helm/releases
FROM alpine/helm:3.10.2
RUN mkdir -p /deploy /config
WORKDIR /deploy

# https://github.com/databus23/helm-diff#install
RUN helm plugin install https://github.com/databus23/helm-diff --version v3.5.0 > /dev/null

RUN apk add --update --latest --no-cache \
  curl bash jq

RUN curl -L https://github.com/a8m/envsubst/releases/download/v1.2.0/envsubst-$(uname -s)-$(uname -m) -o envsubst && chmod +x envsubst && mv envsubst /usr/local/bin

# Setup kubectl (can be up to one minor version ahead of K0s version)
# See skew policy: https://kubernetes.io/releases/version-skew-policy/#kubectl
ENV KUBECTL_VERSION v1.25.4
RUN wget https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    mv kubectl /usr/local/bin && \
    chmod +x /usr/local/bin/kubectl

# TODO: setup waitfor
#COPY waitfor /usr/local/bin
#RUN chmod +x /usr/local/bin/waitfor

COPY apply.sh delete.sh diff.sh entrypoint.sh env.sh generate_kubeconfig.sh interpolate.sh smoketest.sh wait.sh outputs.sh /deploy/
COPY helm-charts /deploy/helm-charts/
COPY dry-run-output /deploy/dry-run-output/
COPY pre-templates /deploy/pre-templates/
COPY pre-manifests /deploy/pre-manifests/
COPY post-templates /deploy/post-templates/
COPY post-manifests /deploy/post-manifests/

# Fixing security issues DSPOP-6679
# hadolint ignore=DL3018
RUN apk add --upgrade git

ENTRYPOINT ["/deploy/entrypoint.sh"]
