ARG BASE_DOCKER_IMAGE
FROM ${BASE_DOCKER_IMAGE}

# TODO: use a real base in the FROM statement like docker.io/splunk/kube-bundler/bases/static-deploy:latest

COPY env.sh smoketest.sh outputs.sh /deploy/
COPY templates /deploy/templates/
