#!/usr/bin/env bash

set -oue pipefail
# set -x

BUILDER_IMAGE=${BUILDER_IMAGE}
MOUNTPATH=${MOUNTPATH:-"/home/topohub"}

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)

echo "Start $1 inside container"
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ${ROOT}:${MOUNTPATH} \
  -w ${MOUNTPATH} \
  ${BUILDER_IMAGE} $@
