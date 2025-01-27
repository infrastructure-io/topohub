NAME=gpu
USERNAME=daocloud
PASSWORD=DaoCloud..
IP_ADDR=10.64.64.94
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${NAME}-secret
  namespace: topohub
type: Opaque
data:
  username: $(echo -n "${USERNAME}" | base64)
  password: $(echo -n "${PASSWORD}" | base64)
---
apiVersion: topohub.infrastructure.io/v1beta1
kind: HostEndpoint
metadata:
  name: ${NAME}
spec:
  # The IP address of the host endpoint (required)
  ipAddr: "${IP_ADDR}"
  # The cluster agent this host endpoint belongs to (optional)
  clusterName: "cluster-test"
  # Credentials for accessing the host endpoint (optional)
  secretName: "${NAME}-secret"
  secretNamespace: "topohub"
  # Communication settings (optional)
  https: true  # Defaults to true if not specified
  port: 443     
EOF



NAME=device-safe
USERNAME=ADMIN
PASSWORD=DaoCloudPassw0rd
IP_ADDR=10.64.64.42
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${NAME}-secret
  namespace: topohub
type: Opaque
data:
  username: $(echo -n "${USERNAME}" | base64)
  password: $(echo -n "${PASSWORD}" | base64)
---
apiVersion: topohub.infrastructure.io/v1beta1
kind: HostEndpoint
metadata:
  name: ${NAME}
spec:
  # The IP address of the host endpoint (required)
  ipAddr: "${IP_ADDR}"
  # The cluster agent this host endpoint belongs to (optional)
  clusterName: "cluster-test"
  # Credentials for accessing the host endpoint (optional)
  secretName: "${NAME}-secret"
  secretNamespace: "topohub"
  # Communication settings (optional)
  https: true  # Defaults to true if not specified
  port: 443     
EOF

