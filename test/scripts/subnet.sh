NAME=default
CIDR=192.168.1.0/24
IP_RANGE=192.168.1.100-192.168.1.200
GATEWAY=192.168.1.1
DNS=192.168.1.2
INT_NAME=eth1
INT_VLAN_ID=0
INT_IPV4=192.168.1.3/24
cat <<EOF | kubectl apply -f -
apiVersion: topohub.infrastructure.io/v1beta1
kind: Subnet
metadata:
  name: ${NAME}
spec:
  ipv4Subnet:
    subnet: "${CIDR}"
    ipRange: "${IP_RANGE}"
    gateway: "${GATEWAY}"
    dns: "${DNS}"
  interface:
    interface: "${INT_NAME}"
    vlanId: ${INT_VLAN_ID}
    ipv4: "${INT_IPV4}"
  feature:
    enableSyncEndpoint:
      dhcpClient: true
      scanEndpoint: false
      defaultClusterName: cluster1
      endpointType: hoststatus
    enableBindDhcpIP: true
    enableReserveNoneDhcpIP: true
    enablePxe: false
    enableZtp: false
EOF

