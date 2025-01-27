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


echo "get the eth0 subnet of node"
NODE_ID=`docker ps | grep ${E2E_CLUSTER_NAME}-control-plane  | awk '{print $1}' `
INTERFACE_MASK=` docker exec ${NODE_ID} ip a s eth0  | grep -oP '(?<=inet\s)[0-9]+(\.[0-9]+){3}/[0-9]+' | awk -F'/' '{print $2}' `
# INTERFACE_IP="172.18.0.3"
NODE_INTERFACE_IP=` docker exec ${NODE_ID} ip a s eth0  |  grep -oP '(?<=inet\s)[0-9]+(\.[0-9]+){3}' `
# INTERFACE_IP="172.18.0.13"
NEW_INTERFACE_IP=$(echo ${NODE_INTERFACE_IP} | awk -F. '{print $1"."$2"."$3"."$4+10}')


echo "install redfish"
helm uninstall static-redfish -n  redfish || true 
helm install static-redfish ${CURRENT_DIR_PATH}/../redfishchart \
  --wait \
  --debug \
  --namespace redfish \
  --create-namespace \
  --set replicaCount=1  \
  --set networkInterface=net1  \
  --set underlayMultusCNI="${UNDERLAY_CNI}" \
  --set staticIp="${NEW_INTERFACE_IP}" \
  --set staticMask="${INTERFACE_MASK}" \
  --set nodeName="${NODE_NAME}"
