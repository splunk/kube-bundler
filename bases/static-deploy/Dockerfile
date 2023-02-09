FROM alpine:3.15
RUN mkdir -p /deploy /config
WORKDIR /deploy

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

COPY entrypoint.sh apply.sh diff.sh env.sh interpolate.sh interpolate-smoketest.sh delete.sh wait.sh smoketest.sh outputs.sh /deploy/
COPY templates /deploy/templates/
COPY manifests /deploy/manifests/
COPY smoketest-templates /deploy/smoketest-templates/
COPY smoketest-manifests /deploy/smoketest-manifests/
COPY pre-templates /deploy/pre-templates/
COPY pre-manifests /deploy/pre-manifests/
COPY post-templates /deploy/post-templates/
COPY post-manifests /deploy/post-manifests/

ENTRYPOINT ["/deploy/entrypoint.sh"]
