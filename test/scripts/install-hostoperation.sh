#!/bin/bash

redfishStatusName=$1
action=$2

echo "redfishStatusName: $redfishStatusName"
echo "action: $action"

[ -n "${redfishStatusName}" ] || {
    echo "kubectl get redfishstatus"
    kubectl get redfishstatus
    echo "error: redfishStatusName is required"
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

kubectl get redfishstatus ${redfishStatusName} &>/dev/null || {
    echo "kubectl get redfishstatus"
    kubectl get redfishstatus
    echo "error: HostEndpoint ${redfishStatusName} not found"
    exit 1
}

name=${redfishStatusName}-${action}

# 创建测试用的 HostOperation 实例
cat <<EOF | kubectl apply -f -
apiVersion: topohub.infrastructure.io/v1beta1
kind: HostOperation
metadata:
  name: $( echo "${name}" | tr '[:upper:]' '[:lower:]')
spec:
  action: "${action}"
  redfishStatusName: ${redfishStatusName}
EOF

echo "HostOperation for ${redfishStatusName} created with action ${action}"

