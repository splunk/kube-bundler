# Deploy Image Bases

Deploy images are containers that hold all tools and templates needed to deploy a bundle.

Available bases include:

* *static-deploy*: performs yaml template materialization using `envsubst` and applies the resources against the cluster with `kubectl apply`
* *helm-deploy*: deploys a [helm](https://helm.sh) project leveraging `overrides.yaml` for custom configuration
* *qbec-deploy* deploys a [qbec](https://qbec.io/) project leveraging `params.libsonnet` for custom configuration
