# Deploying Complex Applications

Installing individual bundles on a cluster can be effective when the number of bundles is small. However, as the number of cluster services grow, it becomes useful to think in terms of a higher level collection of bundles rather than individual bundles.

Bundles usually consist of one or two logically related services. Entire application architectures may be composed of multiple bundles working in tandem. For example, an example complex application may have the following bundles:

* postgres, for an SQL database
* zookeeper, for consensus
* redis, for caching
* web service, for serving HTTP and maintaining business logic

For these complex applications, kube-bundler provides the concept of a `Manifest`. The manifest encapsulates several application deployment concerns into a single declarative resource. Some things that are handled by manifests:

* The list of application bundles to be installed
* The versions of each bundle to be installed
* The ability to install bundles from a [bundle source](other/bundle-sources.md)
* The dependencies of each application bundle
* An initial set of bundle parameters, such as the namespace where the bundle should be installed
* The user's chosen installation [flavor](other/flavors.md)

For this exercise, we will deploy nginx as a manifest.

First, make a directory for bundles and copy the nginx bundle to `/tmp/bundles`:

```
mkdir -p /tmp/bundles
cp ~/go/src/github.com/splunk/kube-bundler/examples/bundles/nginx-v0.0.1.kb /tmp/bundles/
```

Create a local bundle source in the file `source-local.yaml`:

```
---
apiVersion: bundle.splunk.com/v1alpha1
kind: Source
metadata:
  name: local
spec:
  type: directory
  path: /tmp/bundles
```

Apply the source:

```
kubectl apply -f source-local.yaml
```

And then create the manifest in `manifest.yaml`:

```
---
apiVersion: bundle.splunk.com/v1alpha1
kind: Manifest
metadata:
  name: nginx
spec:
  sources:
    - name: local
  bundles:
    - name: nginx
      version: v0.0.1
```

and apply:

```
kubectl apply -f manifest.yaml
```

Now we are ready to install the manifest:

```
kb install manifest nginx
```
