#!/bin/bash
set -euo pipefail

debug=$(jq -r .debug < $CONFIG_JSON)
source $DIR/env.sh

for f in $DIR/pre-templates/*; do
  # Skip if no matching files are found under pre-templates/
  [ -f "$f" ] || continue

  envsubst -no-unset < $f > "$DIR/pre-manifests/$(basename $f)"
  if [ "$debug" = "true" ]; then
    echo "Interpolating $DIR/pre-manifests/$(basename $f)"
    cat "$DIR/pre-manifests/$(basename $f)"
  fi
done

for f in $DIR/post-templates/*; do
  # Skip if no matching files are found under post-templates/
  [ -f "$f" ] || continue

  envsubst -no-unset < $f > "$DIR/post-manifests/$(basename $f)"
  if [ "$debug" = "true" ]; then
    echo "Interpolating $DIR/post-manifests/$(basename $f)"
    cat "$DIR/post-manifests/$(basename $f)"
  fi
done

shopt -s nullglob
for f in $DIR/templates/*; do
  # Skip if no matching files are found under templates/
  [ -f "$f" ] || continue

  envsubst -no-unset < $f > "$DIR/manifests/$(basename $f)"
  if [ "$debug" = "true" ]; then
    echo "Interpolating $DIR/manifests/$(basename $f)"
    cat "$DIR/manifests/$(basename $f)"
  fi
done
