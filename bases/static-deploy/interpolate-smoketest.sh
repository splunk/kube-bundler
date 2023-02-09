#!/bin/bash
set -euo pipefail

debug=$(jq -r .debug < $CONFIG_JSON)
source $DIR/env.sh

shopt -s nullglob
for f in $DIR/smoketest-templates/*; do
  # Skip if no matching files are found under templates/
  [ -f "$f" ] || continue

  envsubst -no-unset < $f > "$DIR/smoketest-manifests/$(basename $f)"
  if [ "$debug" = "true" ]; then
    echo "Interpolating $DIR/smoketest-manifests/$(basename $f)"
    cat "$DIR/smoketest-manifests/$(basename $f)"
  fi
done
