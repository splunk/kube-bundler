FROM alpine:3.15
RUN mkdir -p /deploy /config
WORKDIR /deploy

RUN apk add --update --latest --no-cache \
  curl bash jq

# Setup kubectl (can be up to one minor version ahead of K0s version)
# See skew policy: https://kubernetes.io/releases/version-skew-policy/#kubectl
ENV KUBECTL_VERSION v1.25.4
RUN wget https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    mv kubectl /usr/local/bin && \
    chmod +x /usr/local/bin/kubectl

# see https://github.com/splunk/qbec/releases
ENV QBEC_VERSION 0.15.2
ENV QBEC_SHA ac19255fbeabc1d864def149896928638323866333c350a951bbf64e8af345a3
RUN curl -L -O https://github.com/splunk/qbec/releases/download/v${QBEC_VERSION}/qbec-linux-amd64.tar.gz && \
    echo "${QBEC_SHA}  qbec-linux-amd64.tar.gz" | sha256sum -c && \
    tar -xzf qbec-linux-amd64.tar.gz -C /usr/local/bin qbec && rm -rf qbec-linux-amd64.tar.gz

# TODO: setup waitfor
#COPY waitfor /usr/local/bin
#RUN chmod +x /usr/local/bin/waitfor

COPY entrypoint.sh apply.sh diff.sh delete.sh wait.sh smoketest.sh generate_kubeconfig.sh outputs.sh /deploy/
COPY components /deploy/components/

ENTRYPOINT ["/deploy/entrypoint.sh"]
