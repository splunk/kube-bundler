#!/bin/bash
set -euo pipefail

PROG=$(basename $0)
export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
export CONFIG_JSON=/config/parameters.json
export INSTALL_JSON=/config/install.json
export REQUIRES_JSON=/config/requires.json
export FLAVOR_JSON=/config/flavor.json
export INPUT_DIR=/config/inputs

# Set env variables, and materialize the dry-run manifests
# Do not run interpolate.sh - this is only for creating overrides.yaml, only needed for apply()
source "$DIR/env.sh"
debug=$(jq -r .debug < $CONFIG_JSON)
DEBUG_FLAG=""
if [ "$debug" = "true" ]; then
  DEBUG_FLAG="--debug"
fi
export DEBUG_FLAG

# Define KUBECONFIG for Helm
source "$DIR/generate_kubeconfig.sh"

sub_help() {
  echo "Usage: $PROG <subcommand> [options]"
  echo "Subcommands:"
  echo "    apply      Install or upgrade the service"
  echo "    diff       Diff the current service deployment with new code or configuration (performs no changes)"
  echo "    outputs    Save the service output variables for dependencies to use (e.g. endpoints, usernames, kubernetes secret names)"
  echo "    smoketest  Smoketest the service"
  echo "    wait       Wait for the service to complete deployment"
  echo "    delete     Delete the service"
  echo ""
}

sub_apply() {
  $DIR/apply.sh
}
  
sub_diff() {
  $DIR/diff.sh
}

sub_outputs() {
  $DIR/outputs.sh
}

sub_smoketest() {
  $DIR/smoketest.sh
}

sub_wait() {
  $DIR/wait.sh
}

sub_delete() {
  $DIR/delete.sh
}

# Call each requested function in order
for action in "$@"; do
  echo "======> Executing action $action"
  "sub_$action"
  echo "======> Completed action $action"
done
