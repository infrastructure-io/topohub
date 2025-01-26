cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: 4u-secret
  namespace: bmc
type: Opaque
data:
  username: $(echo -n "ADMIN" | base64)
  password: $(echo -n "NSHETVTLJA" | base64)
---
apiVersion: topohub.infrastructure.io/v1beta1
kind: HostEndpoint
metadata:
  name: 4u-host
spec:
  # The IP address of the host endpoint (required)
  ipAddr: "10.64.64.94"
  # The cluster agent this host endpoint belongs to (optional)
  #clusterAgent: "bmc-clusteragent"
  # Credentials for accessing the host endpoint (optional)
  secretName: "4u-secret"
  secretNamespace: "bmc"
  # Communication settings (optional)
  https: true  # Defaults to true if not specified
  port: 443     
EOF
