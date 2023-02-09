# static-deploy
Simple base deploy container implementation that deploys static yaml. Basic parameterization of the yaml is support with `envsubst`.

This base is not useful by itself, but needs yaml templates added to the `templates/` directory.

## Building
docker build . -t local/static-deploy:latest
