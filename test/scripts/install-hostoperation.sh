#!/bin/bash

hostStatusName=$1
action=$2

echo "hostStatusName: $hostStatusName"
echo "action: $action"

[ -n "${hostStatusName}" ] || {
    echo "kubectl get hoststatus"
    kubectl get hoststatus
    echo "error: HostStatusName is required"
    exit 1
}

[ -n "${action}" ] || {
    echo "error: Action is required"
    echo "Valid actions: On, ForceOn, ForceOff, GracefulShutdown, ForceRestart, GracefulRestart, PxeReboot"
    exit 1
}

case "${action}" in
    "On"|"ForceOn"|"ForceOff"|"GracefulShutdown"|"ForceRestart"|"GracefulRestart"|"PxeReboot")
        ;;
    *)
        echo "error: Invalid action ${action}"
        echo "Valid actions: On, ForceOn, ForceOff, GracefulShutdown, ForceRestart, GracefulRestart, PxeReboot"
        exit 1
        ;;
esac

kubectl get hoststatus ${hostStatusName} &>/dev/null || {
    echo "kubectl get hoststatus"
    kubectl get hoststatus
    echo "error: HostEndpoint ${hostStatusName} not found"
    exit 1
}

name=${hostStatusName}-${action}

# 创建测试用的 HostOperation 实例
cat <<EOF | kubectl apply -f -
apiVersion: topohub.infrastructure.io/v1beta1
kind: HostOperation
metadata:
  name: $( echo "${name}" | tr '[:upper:]' '[:lower:]')
spec:
  action: "${action}"
  hostStatusName: ${hostStatusName}
EOF

echo "HostOperation for ${hostStatusName} created with action ${action}"

