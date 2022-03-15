#!/usr/bin/env bash

FULL_PATH_TO_SCRIPT="$(realpath "$0")"
SCRIPT_DIRECTORY="$(dirname "$FULL_PATH_TO_SCRIPT")"
ROOT_DIRECTORY="$(realpath "${SCRIPT_DIRECTORY}/..")"
FUNCTIONS_DIRECTORY="$(realpath "${SCRIPT_DIRECTORY}/../cmd/functions")"

pushd "${FUNCTIONS_DIRECTORY}" > /dev/null || exit 1
trap "popd  > /dev/null" EXIT

FUNCTION="$1"
shift

if [[ -z "${FUNCTION}" ]]; then
  for d in "${FUNCTIONS_DIRECTORY}"/*/; do
      if [ -d "${d}" ]; then
        FUNCTION="$(basename "${d}")"
        docker build -f "${d}Dockerfile" \
                     -t "ghcr.io/arikkfir/kude/functions/${FUNCTION}:test" \
                     --build-arg "function=${FUNCTION}" \
                     "${ROOT_DIRECTORY}"
      fi
  done
else
  d="${FUNCTIONS_DIRECTORY}/${FUNCTION}/"
  docker build -f "${d}Dockerfile" \
               -t "ghcr.io/arikkfir/kude/functions/${FUNCTION}:test" \
               --build-arg "function=${FUNCTION}" \
               "${ROOT_DIRECTORY}"
fi
