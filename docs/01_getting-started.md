# Getting Started with Kube Bundler

Assuming a Go toolchain is installed and the source has been checked out to `~/go/src/github.com/splunk/kube-bundler`, building kb is straightforward:

```
cd ~/go/github.com/splunk/kube-bundler/cmd/kb
go install
```

The resulting binary will be called `kb` and copied to `~/go/bin`. To make it convenient to use `kb`, include `~/go/bin` in your PATH:

```
export PATH=$PATH:$HOME/go/bin
```

## Setup a cluster

By default, `kb` will search for the kubernetes config from the `KUBECONFIG` environment variable, or `~/.kube/config`.

Before using `kb` to install or manage bundles, the cluster must first be bootstrapped:

```
kb bootstrap
```

This command installs the necessary CRDs for kube-bundler to function. At this time, `kb` does not require a controller or any resources to be running on the cluster. All `kb` operations are initiated by CLI commands and not through the use of a running reconcile loop (this may change in the future).

## Installing your first bundle

Prebuilt bundles are easy to install. On your kubernetes cluster, install the nginx bundle:

```
kb install bundle ~/go/src/github.com/splunk/kube-bundler/examples/bundles/nginx-v0.0.1.kb
```

Note: ability to pull from dockerhub is required for this exercise

Check for the following output:

```
Waiting 1m30s for action 'apply outputs' on nginx...
Waiting for deployment "nginx" rollout to finish: 0 of 2 updated replicas are available...
Waiting for deployment "nginx" rollout to finish: 0 of 2 updated replicas are available...
deployment "nginx" successfully rolled out
Waiting 1m30s for action 'smoketest' on nginx...
deployment "nginx" successfully rolled out
```

After a few minutes, nginx will be installed. The following happened:

1. The nginx bundle was registered with the cluster
2. An instance of the nginx bundle was installed to the `default` namespace
3. The kubernetes resources for nginx were applied to the cluster
4. Kubernetes launched the nginx deployment
5. The nginx service was smoketested and determined healthy

## Configuring nginx

kube-bundler provides a standardized mechanism for application bundle authors to provide configuration parameters, default values, and descriptions of each parameter.

To see the nginx bundle's configuration:

```
kb config list nginx
```

which should produce output similar to

```
debug=false
docker_tag=latest
namespace=default
port=8080
replicas=2
suffix=
```

To change configuration options, such as the number of replicas, try this:

```
kb config set nginx replicas=3
```

This will change the bundle configuration, but not apply any changes to the running resources.

If desired, preview the changes before they're applied using `kb diff`

```
kb diff bundle nginx
```

which shows output similar to the following:

```
--- /tmp/LIVE-2930081926/apps.v1.Deployment.default.nginx
+++ /tmp/MERGED-118012016/apps.v1.Deployment.default.nginx
@@ -6,14 +6,14 @@
     kubectl.kubernetes.io/last-applied-configuration: |
       {"apiVersion":"apps/v1","kind":"Deployment","metadata":{"annotations":{},"name":"nginx","namespace":"default"},"spec":{"replicas":2,"selector":{"matchLabels":{"app":"nginx"}},"template":{"metadata":{"labels":{"app":"nginx"}},"spec":{"containers":[{"env":null,"image":"docker.io/nginxinc/nginx-unprivileged:latest","name":"nginx","ports":[{"containerPort":8080}]}],"securityContext":{"fsGroup":100,"runAsNonRoot":true,"runAsUser":100}}}}}
   creationTimestamp: "2023-02-01T21:32:35Z"
-  generation: 1
+  generation: 2
   name: nginx
   namespace: default
   resourceVersion: "44713601"
   uid: 2ea3cece-9fc0-4444-bbb7-642ac3f401b4
 spec:
   progressDeadlineSeconds: 600
-  replicas: 2
+  replicas: 3
   revisionHistoryLimit: 10
   selector:
     matchLabels:
```

And apply the changes with a deploy:

```
kb deploy bundle nginx
```

Sample output output:

```
Waiting 1m30s for action 'apply outputs' on nginx...
Waiting for deployment "nginx" rollout to finish: 2 of 3 updated replicas are available...
deployment "nginx" successfully rolled out
Waiting 1m30s for action 'smoketest' on nginx...
deployment "nginx" successfully rolled out
```
What just happened:

1. The `replicas` configuration parameter was changed from `2` to `3`
2. By running the `kb diff` command, we saw the changes that would be applied to the cluster as a result of the new configuration
3. On deploy, the running resources were updated with the newly specified replica count

## Manually Running a Smoketest

Smoketests are quick and simple tests that ensure the deployment is working correctly. They are run automatically after installation and deployment. The exact actions taken by the smoketest are determined by the bundle author.

However, if you want extra verification, perhaps during some kubernetes cluster instability, you can run smoketests manually. 

```
kb smoketest bundle nginx
```

which produces the following output:

```
Waiting 1m30s for action 'smoketest' on nginx...
deployment "nginx" successfully rolled out
Smoketests complete
```

## Uninstall

Finally, to remove a bundle and all of its running resources, do the following:

```
kb uninstall nginx
```

which shows the following output:

```
Waiting 1m30s for action 'delete' on nginx...
```

At the completetion of this command, all nginx resources have been removed.

Next up, learn how to [build your own bundles](02_building-bundles.md).
