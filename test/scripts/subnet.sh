NAME=net0
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
      defaultClusterName: cluster1
      endpointType: hoststatus
    enableBindDhcpIP: true
    enablePxe: true
    enableZtp: false
EOF

cat <<EOF | kubectl apply -f -
apiVersion: topohub.infrastructure.io/v1beta1
kind: BindingIp
metadata:
  name: 192-168-1-199
spec:
  subnet: ${NAME}
  ipAddr: 192.168.1.199
  macAddr: 00:00:00:00:00:11
EOF

#----------------------------

NAME=net10
CIDR=192.168.10.0/24
IP_RANGE=192.168.10.100-192.168.10.200
GATEWAY=192.168.10.1
DNS=192.168.10.2
INT_NAME=eth1
INT_VLAN_ID=10
INT_IPV4=192.168.10.3/24
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
      defaultClusterName: cluster2
      endpointType: hoststatus
    enableBindDhcpIP: true
    enablePxe: true
    enableZtp: true
EOF

#-----------

NAME=net-invalid-nic
CIDR=192.168.99.0/24
IP_RANGE=192.168.99.100-192.168.99.200
GATEWAY=192.168.99.1
DNS=192.168.99.2
INT_NAME=eth99
INT_VLAN_ID=99
INT_IPV4=192.168.99.3/24
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
      defaultClusterName: cluster3
      endpointType: hoststatus
    enableBindDhcpIP: true
    enablePxe: true
    enableZtp: true
EOF
