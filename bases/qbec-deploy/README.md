# qbec-deploy
Qbec deploy container implementation that deploys yaml and jsonnet using qbec. 

This base is not useful by itself, but needs components to be added to the `/components` directory.
Components are comprised of jsonnet files containing definitions for Kubernetes objects as well as a `params.libsonnet`
file that contains environment variable values. 

Any application bundle that is created with this base will require the `environments.server` value 
to be set to `"https://kubernetes.default"` in `qbec.yaml`.

## Building
docker build . -t local/qbec-deploy:latest
