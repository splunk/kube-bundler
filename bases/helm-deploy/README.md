# helm-deploy
Simple base deploy container implementation for deploying Helm charts. This base is not useful by itself, but needs Helm charts added to the `helm-charts/` directory. Use the [standard directory structure](https://helm.sh/docs/topics/charts/#the-chart-file-structure) for Helm charts.

There should only be one directory (the named Helm chart, with nested subcharts) under `helm-charts/`; this will be the target of `helm upgrade --install`, and should ideally match the `name` in `app.yaml.tmpl` (see `K8S_RELEASE_NAME` below).

A few variables are expected to always be present in `env.sh`. These are:
* `K8S_RELEASE_NAME` (hard-coded name of the Helm chart; ideally matches the app bundle name to avoid confusion, since the `kb` pod would be named differently from the deployed objects)
* `K8S_NAMESPACE` (the primary namespace; other namespaces can be named something separate, but will not be labeled)
* `K8S_RESOURCE_SUFFIX` (isolation-suffix)

## Overrides
`overrides.yaml.tmpl` will be converted into `overrides.yaml` and be provided with `--values` at install-time, whose values will take precedence over any provided `values.yaml` ([precedence list](https://helm.sh/docs/chart_template_guide/values_files/)). This will be the only file provided; make adjustments for subcharts here as well. Use this file so that it's easier to tell at a glance what variables we're overriding. The overrides in `overrides.yaml` will not have context but the key structure + variable names together should be reasonably self-explanatory (combined with the descriptions in `app.yaml.tmpl`).

Optionally, you can also create a copy of `values.yaml` called `values-modified.yaml`. This is only for reference, and will not be interpolated; this just makes it easier to see the comment-documentation on what a particular override is for (since the `overrides.yaml` doesn't show the K8s context). This will also make it easier to track and diff changes if we decide to update the Helm chart to a newer one.

## Dependencies
Note that it _is_ required to update any dependencies that dynamically pull from a remote registry, to instead manually download the chart and add it under a new directory `charts/`, then update accordingly. E.g. from `nginx-helm-example`:
```
dependencies:
  - name: common
    # repository: https://charts.bitnami.com/bitnami
    # tags:
    #   - bitnami-common
    version: 1.x.x
    
    # download the 'common' chart and move it here to a subdirectory under 'charts/' called 'common'
    # MUST match the name; otherwise Helm will error and only specify that Chart.yaml is missing "in nginx"
    repository: "file://charts/common"

```
Given a Helm chart, you can download all the dependencies locally (see [docs](https://helm.sh/docs/topics/charts/#managing-dependencies-with-the-dependencies-field)):
```
helm repo add <...>
helm dep update
```

Otherwise to pull an individual Helm chart locally, use [`helm pull`](https://helm.sh/docs/helm/helm_pull/), like:
```
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# note that nested subdirectories will be created, and the extrated directory will be called the chart name
# e.g. no need to specify <...>/charts/nginx, just <...>/charts
helm pull bitnami/nginx --version 11.0.2 --untar --untardir path/to/target
```
Helm can use zipped tarballs, but we should unzip the tarballs to verify that they don't have nested dependencies that require Internet access to pull from.


## Building
docker build . -t local/helm-deploy:latest
