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

echo "loading $IMAGE_NAME"
docker inspect $IMAGE_NAME &>/dev/null || { echo "error, failed to find image $IMAGE_NAME"; exit 1; }
kind load docker-image $IMAGE_NAME --name ${CLUSTER_NAME}
echo "Images loaded successfully"

echo "Deploying application using Helm chart..."

helm uninstall topohub -n topohub --wait &>/dev/null || true

echo "run topo on worker nodes"
kubectl label node ${CLUSTER_NAME}-worker topohub=true
kubectl label node ${CLUSTER_NAME}-worker2 topohub=true

cat <<EOF >/tmp/topo.yaml
replicaCount: 2
logLevel: "debug"
image:
  tag: ${IMAGE_VERSION}

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
        - true
EOF

		# --set clusterAgent.feature.dhcpServerConfig.subnet="192.168.0.0/24" \
		# --set clusterAgent.feature.dhcpServerConfig.ipRange="192.168.0.100-192.168.0.200" \
		# --set clusterAgent.feature.dhcpServerConfig.gateway="192.168.0.1" \
		# --set clusterAgent.feature.dhcpServerConfig.selfIp="192.168.0.2/24" \

helm install topohub ${PROJECT_ROOT_PATH}/chart \
    --namespace topohub \
    --create-namespace \
    --debug \
    --wait -f /tmp/topo.yaml

