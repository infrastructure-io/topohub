#!/bin/bash

set -x
set -o errexit
set -o pipefail
set -o nounset

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)
PROJECT_ROOT_PATH=$(cd ${CURRENT_DIR_PATH}/../..; pwd)

TOOLS_IMAGE_REF=ghcr.io/infrastructure-io/topohub-tools:latest

# try to load tools image
docker inspect ${TOOLS_IMAGE_REF} &>/dev/null || \
    docker pull ${TOOLS_IMAGE_REF} || \
    ( cd ${PROJECT_ROOT_PATH} && make build-tools-image )

IMAGES=$( helm template redfish ${CURRENT_DIR_PATH}/../redfishchart | grep "image:"  | awk '{print $2}' | sort | tr -d '"' | uniq )
echo "IMAGES"
echo "${IMAGES}"
for IMAGE in $IMAGES; do
    echo "loading $IMAGE"
    docker inspect $IMAGE &>/dev/null || docker pull $IMAGE 
    kind load docker-image $IMAGE --name ${E2E_CLUSTER_NAME}
done

DISABLE_REDFISH_MOCKUP=${DISABLE_REDFISH_MOCKUP:-"false"}

echo "install redfish"
helm uninstall ${HELM_NAME} -n  redfish || true 
helm install ${HELM_NAME} ${CURRENT_DIR_PATH}/../redfishchart \
  --wait \
  --debug \
  --namespace redfish \
  --create-namespace \
  --set replicaCount=2  \
  --set networkInterface=net1  \
  --set underlayMultusCNI="${UNDERLAY_CNI}" \
  --set nodeName="${NODE_NAME}" \
  --set disableRedfishMockup=${DISABLE_REDFISH_MOCKUP}
