#!/usr/bin/env bash

FUNCTION="$1"

set -euo pipefail

FULL_PATH_TO_SCRIPT="$(realpath "$0")"
SCRIPT_DIRECTORY="$(dirname "$FULL_PATH_TO_SCRIPT")"
ROOT_DIRECTORY="$(realpath "${SCRIPT_DIRECTORY}/..")"
FUNCTIONS_DIRECTORY="$(realpath "${SCRIPT_DIRECTORY}/../cmd/functions")"

pushd "${FUNCTIONS_DIRECTORY}" > /dev/null || exit 1
trap "popd  > /dev/null" EXIT

COMMIT_SHA="$(git rev-parse HEAD)"

if [[ -z "${FUNCTION}" ]]; then
  for d in "${FUNCTIONS_DIRECTORY}"/*/; do
      if [ -d "${d}" ]; then
        FUNCTION="$(basename "${d}")"
        docker build -f "build/docker/${FUNCTION}-function.dockerfile" \
                     -t "ghcr.io/arikkfir/kude/functions/${FUNCTION}:${COMMIT_SHA}" \
                     --build-arg "function=${FUNCTION}" \
                     "${ROOT_DIRECTORY}"
        docker tag "ghcr.io/arikkfir/kude/functions/${FUNCTION}:${COMMIT_SHA}" "ghcr.io/arikkfir/kude/functions/${FUNCTION}:unknown"
      fi
  done
else
  d="${FUNCTIONS_DIRECTORY}/${FUNCTION}/"
  docker build -f "build/docker/${FUNCTION}-function.dockerfile" \
               -t "ghcr.io/arikkfir/kude/functions/${FUNCTION}:${COMMIT_SHA}" \
               --build-arg "function=${FUNCTION}" \
               "${ROOT_DIRECTORY}"
  docker tag "ghcr.io/arikkfir/kude/functions/${FUNCTION}:${COMMIT_SHA}" "ghcr.io/arikkfir/kude/functions/${FUNCTION}:unknown"
fi
