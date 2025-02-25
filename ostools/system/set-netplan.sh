#!/bin/bash

:<<EOF
way1：
    INTERFACE="eth1" \
    IPV4_IP="172.16.0.10/24"  IPV4_GATEWAY="172.16.0.1" \
    IPV6_IP="fd00::10/64"     IPV6_GATEWAY="fd00::1" \
    MTU="9000" \
    set-netplan.sh

WAY2：
    INTERFACE="eth1" \
    DHCP4="true" \
    set-netplan.sh

EOF

set -o errexit

INTERFACE=${INTERFACE:-""}
IPV4_IP=${IPV4_IP:-""}
IPV4_GATEWAY=${IPV4_GATEWAY:-""}
IPV6_IP=${IPV6_IP:-""}
IPV6_GATEWAY=${IPV6_GATEWAY:-""}
MTU=${MTU:-""}
DHCP4=${DHCP4:-""}
DHCP6=${DHCP6:-""}


[ -n "${INTERFACE}" ] || { echo "ERROR: INTERFACE is empty"; exit 1; }

if [ "${DHCP4}" == "true" ] ; then
  IPV4_IP=""
  IPV4_GATEWAY=""
else
  DHCP4=""
fi

if [ "${DHCP6}" == "true" ] ; then
  IPV6_IP=""
  IPV6_GATEWAY=""
else
  DHCP6=""
fi

echo "INTERFACE=${INTERFACE}"
echo "IPV4_IP=${IPV4_IP}"
echo "IPV4_GATEWAY=${IPV4_GATEWAY}"
echo "IPV6_IP=${IPV6_IP}"
echo "IPV6_GATEWAY=${IPV6_GATEWAY}"
echo "MTU=${MTU}"
echo "DHCP4=${DHCP4}"
echo "DHCP6=${DHCP6}"

which netplan >/dev/null 2>&1 || { echo "ERROR: netplan is not installed"; exit 1; }

#========
Config_IP=""
[ -n "$IPV4_IP" ] && \
Config_IP="       - \"${IPV4_IP}\""
[ -n "$IPV6_IP" ] && \
Config_IP="\
${Config_IP}
       - \"${IPV6_IP}\""

Config_gw=""
if [ -n "$IPV4_GATEWAY" ] || [ -n "$IPV6_GATEWAY" ] ; then
Config_gw="      routes:"
[ -n "$IPV4_GATEWAY" ] && \
Config_gw="\
${Config_gw}
        - to: default
          via: ${IPV4_GATEWAY}
          metric: 10"
[ -n "$IPV6_GATEWAY" ] && \
Config_gw="\
${Config_gw}
        - to: default
          via: ${IPV6_GATEWAY}
          metric: 10"
fi
[ -n "$MTU" ] && \
Config_MTU="\
      mtu: ${MTU}"

[ "$DHCP4" == "true" ] && \
DHCP_CONFIG="\
      dhcp4: true"

[ "$DHCP6" == "true" ] && \
DHCP_CONFIG+="\
      dhcp6: true"

cat <<EOF >/etc/netplan/12-${INTERFACE}.yaml
network:
  version: 2
  renderer: networkd
  ethernets:
    ${INTERFACE}:
${DHCP_CONFIG}
$( [ -n "${DHCP_CONFIG}" ] || echo "      addresses:" ] )
      addresses:
${Config_IP}
${Config_gw}
${Config_MTU}
EOF

# Permissions for /etc/netplan/*.yaml are too open. Netplan configuration should NOT be accessible by others
chmod 600 /etc/netplan/*
netplan apply
