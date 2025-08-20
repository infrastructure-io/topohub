#!/usr/bin/env bash

set -euo pipefail
# set -x

HARBOR_HOST=${1}
USERNAME=${2}
PASSWORD=${3}
HARBOR_PROJECT=${4:-"infrastructure-io"}

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
GIT_COMMIT_TIME=$(git show -s --format='format:%aI')
GIT_COMMIT_VERSION=$(git show -s --format='format:%H')
VERSION=$(cat ${ROOT}/VERSION | tr -d ' ' | tr -d '\n')-dev-$(git rev-parse --short=8 HEAD)
CHART_VERSION=$(echo "${VERSION}" | sed  's/^v//g' )
IMAGE_NAME=${HARBOR_HOST}/${HARBOR_PROJECT}/topohub

build_images() {
    docker login ${HARBOR_HOST} -u ${USERNAME} -p ${PASSWORD}

    docker build \
      --platform linux/amd64 \
      --build-arg GIT_COMMIT_VERSION=${GIT_COMMIT_VERSION} \
      --build-arg GIT_COMMIT_TIME=${GIT_COMMIT_TIME} \
      --build-arg PROJECT_VERSION=${VERSION} \
      -t ${IMAGE_NAME}:${VERSION} -f ${ROOT}/image/topohub/Dockerfile .
    
    docker push ${IMAGE_NAME}:${VERSION}
}

build_charts() {
    local output_dir="${ROOT}/output"
    trap "rm -rf ${ROOT}/output" EXIT SIGINT
    mkdir -p ${output_dir}
    cp -r ${ROOT}/chart/ ${output_dir}/

    # Set values.yaml
    REGISTRY=${HARBOR_HOST} REPOSITORY=${HARBOR_PROJECT}/topohub yq -i '
    .image.registry = env(REGISTRY) |
    .image.repository = env(REPOSITORY)
    ' ${output_dir}/chart/values.yaml

    cat ${output_dir}/chart/values.yaml

    helm package ${output_dir}/chart -d ${ROOT}/output/ --version ${CHART_VERSION}
    helm repo add topohub-release https://${HARBOR_HOST}/chartrepo/topohub
    helm repo update topohub-release
    helm cm-push ${ROOT}/output/topohub-${CHART_VERSION}.tgz topohub-release \
      -a ${CHART_VERSION} -v ${CHART_VERSION} -u ${USERNAME} -p ${PASSWORD}
}

build_images
build_charts
