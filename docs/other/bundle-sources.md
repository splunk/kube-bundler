# Bundles Sources

Bundle sources are package repositories that host application bundles. Bundles sources are usually used with manifests for ease of installing many bundles.

Manifests can be configured to use a local bundle source like a directory, or pull from a remote bundle source such as S3.

Example bundle source definition:

```
apiVersion: bundle.splunk.com/v1alpha1
kind: Source
metadata:
  name: local
spec:
  type: directory
  path: /opt/myapp/bundles
```

Implementation-wise, a bundle source usually stores the bundles files (.kb) and metadata in the bundle source. A directory listing might look like this:

```
bundles/
    postgres-v1.2.3.kb
    postgres-v1.2.4.kb
    postgres.json
    redis-v2.3.4.kb
    redis-v2.3.5.kb
    redis-v3.1.0.kb
    redis.json
```

The `.json` file is a metadata file containing a pointer to the latest available bundle of that name.
