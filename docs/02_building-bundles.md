# Building Bundles

kube-bundler can package almost any application as a bundle.

A typical kubernetes application will consist of the following:

* yaml, json, helm, or other manifests that describe the application
* docker images
* scripts, makefiles, or perhaps an operator to manage various lifecycle tasks

When an application is deployed to the cluster, the user usually needs to have an understanding of each of these components. For example, they need to install tools necessary apply the resources to the cluster (e.g. helm). They may need to push docker images to their own internal registry (or signup for a dockerhub account to avoid rate limiting). They may need to read the application's documentation to understand the chosen deployment methodology for that project.

Application bundles relieve the user from most of these concerns. Users don't have to directly install any additional tools, they can optionally use kube-bundler's airgap registry support, and they configure the bundle through the standardized kube-bundler interface.

Project owners also gain the ability to be prescriptive in the tooling used for deployments. Tools like helm are packaged inside the bundle's deploy container, so every deployment uses the exact tool version specified by the project owner. Project owners get to choose how maintenance operations are performed such as restaring the service. This leads to a consistent, first-class experience for everyone.

## Creating your first bundle

We will use the nginx bundle as an example.

Application bundles consist of these minimum parts:

* `app.yaml` - a yaml document containing an `Application` custom resource
* `Dockerfile` - a dockerfile for the container that deploys the application
* Deploy container - a container that contains all tools, templates, and scripts necessary to deploy the application

A minimal nginx `app.yaml` looks like this:

```
apiVersion: bundle.splunk.com/v1alpha1
kind: Application
spec:
  name: nginx
  version: v0.0.1
  deployImage: docker.io/myorg/nginx-deploy:latest
  images:
    - image: docker.io/nginxinc/nginx-unprivileged:latest
  parameters:
    - name: namespace
      default: default
      description: Namespace to deploy
    - name: replicas
      default: "2"
      description: Number of service replicas
  resources:
    - name: nginx
      type: deployment
```

This creates an nginx bundle with two parameters - `namespace` and `replicas` - and specifies the `nginx` Deployment will be part of the running application. The application service docker image is specified under the `images` key and will be embedded in the bundle `.kb` file. The `deployImage` refers to the bundle's deploy container, and will be used to apply yaml manifests to the kubernetes cluster.

The above are sufficient to build a very minimal bundle, but most applications will require more. Because application deployments often look similar across projects, bundles usually leverage a `base` deploy container. The base provides repeatable deploy pattern so each project doesn't have to implement a deploy container from scratch.

The nginx bundle uses the base called `static-deploy` and the rest of this document will focus on building bundles using static-deploy.

Static-deploy expects the following:

* Configuration parameters are defined in `app.yaml` (see `namespace` and `replicas` above)
* `env.sh` is used to load parameters into environment variables

Example loading the `replicas` value from `app.yaml`:

```
export K8S_REPLICAS=$(jq -r .replicas < $CONFIG_JSON)
```

* yaml manifests are held under the `templates` directory and accept parameterization of the form `${VAR}`

Example using parameters in a yaml manifest:

```
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  replicas: ${K8S_REPLICAS}
```

The `K8S_REPLICAS` key name must match the `export K8S_REPLICAS=...` line in `env.sh` for the substitution to take place.

* The yaml mainfests can be applied in any order using `kubectl apply`

A full example can be seen at https://github.com/splunk/kube-bundler/examples/bundles/nginx

## Optional features

In addition to the basics, the nginx bundle also leverages these static-deploy features:

* `outputs.sh` is used to provide the nginx endpoint, so that other bundles can discover this nginx service on the cluster
* `smoketest.sh` is used for a simple smoketest that runs an HTTP GET on nginx

## Building the bundle

Once the bundle definition is in place, use `kb build`. For nginx, the current invocation looks like this:

```
cd ~/go/src/github.com/splunk/kube-bundler/examples/bundles/nginx
kb build .
```

This step requires a local docker daemon and will do the following:

* Build the deploy image using `docker build`. In this step, all tools are downloaded and yaml templates are packaged into the deploy container
* Push the deploy image to the remote docker registry specified by the `deployImage` key
* Download the docker images specified in the `images` key (these are the service's docker images used by the Deployments, Statefulsets, etc)
* Bundle all docker images into a `.kb` file

Once this process is complete, the resulting `.kb` file will have the entire application definition, deploy image, and all docker images associated with the application.

The bundle is ready to be installed with `kb install bundle`.

Next up: [Deploying Complex Applications](03_deploying-complex-applications.md)