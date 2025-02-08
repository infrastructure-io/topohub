#!/bin/bash

set -x
set -o errexit
set -o pipefail
set -o nounset

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)
PROJECT_ROOT_PATH=$(cd ${CURRENT_DIR_PATH}/../..; pwd)

IMAGE_NAME=${IMAGE_NAME:-"ghcr.io/infrastructure-io/topohub-tools:latest"}
IMAGE_VERSION=${IMAGE_VERSION:-"latest"}
CLUSTER_NAME=${CLUSTER_NAME:-"topohub"}

#====================================

echo "Deploying application using Helm chart..."

helm uninstall topohub -n topohub --wait &>/dev/null || true

echo "run topo on worker nodes"
kubectl label node ${CLUSTER_NAME}-worker topohub=true
kubectl label node ${CLUSTER_NAME}-worker2 topohub=true

cat <<EOF >/tmp/topo.yaml
replicaCount: 1
logLevel: "debug"
image:
  tag: "${IMAGE_VERSION}"

defaultConfig:
  redfish:
    https: false
    port: 8000
    username: ""
    password: ""
  dhcpServer:
    interface: "eth0"

storage:
    type: "hostPath"

nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: topohub
        operator: In
        values:
        - "true"
EOF

IMAGE_LIST=$( helm template topohub ${PROJECT_ROOT_PATH}/chart -f /tmp/topo.yaml  | grep "image:" | awk '{print $2}' | tr -d '"' )
for IMAGE in $IMAGE_LIST; do
    echo "loading $IMAGE"
    docker inspect $IMAGE &>/dev/null || docker pull $IMAGE  
    kind load docker-image $IMAGE --name ${CLUSTER_NAME}
done

helm install topohub ${PROJECT_ROOT_PATH}/chart \
    --namespace topohub \
    --create-namespace \
    --debug \
    --wait -f /tmp/topo.yaml
