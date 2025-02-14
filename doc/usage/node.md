# 管理主机

## 管理主机的带外网络

该功能，可实现对主机的 BMC 的 IP 管理，BMC 的信息管理，BMC 的日志管理，BMC 的电源管理等

### 创建 dhcp server 来管理 BMC 主机

1. 创建 Subnet 实例，它会在 topohub 运行节点的网卡上启动 DHCP 服务

```bash
NAME=bmc-net-1
CIDR=192.168.1.0/24
IP_RANGE=192.168.1.100-192.168.1.200
GATEWAY=192.168.1.1
INT_VLAN_ID=10
INT_IPV4=192.168.1.2/24
cat <<EOF | kubectl apply -f -
apiVersion: topohub.infrastructure.io/v1beta1
kind: Subnet
metadata:
  name: ${NAME}
spec:
  ipv4Subnet:
    # DHCP 子网号
    subnet: "${CIDR}"
    # DHCP 可分配 IP 地址
    ipRange: "${IP_RANGE}"
    # 该子网的网关
    gateway: "${GATEWAY}"
  interface:
    # 在子网的 vlan ID
    vlanId: ${INT_VLAN_ID}
    # 请分配一个可用 IP 地址，作为被 DHCP server 的工作 IP 地址
    ipv4: "${INT_IPV4}"
  feature:
    enableSyncEndpoint:
      dhcpClient: true
      defaultClusterName: cluster1
    enableBindDhcpIP: true
EOF

```

创建如上实例，请了解如下信息：

* 在安装 topohub 时，defaultConfig.dhcpServer.interface 的值指定了节点上的工作网卡，创建subnet 后，会以 spec.interface.vlanId 作为 vlan id， 在该工作网卡上创建 vlan 子接口，带上 “topohub.${vlanId}” 的后缀，并配置 spec.interface.ipv4 地址。如果 spec.interface.vlanId==0，那么不会创建 vlan 子接口，DHCP server 直接工作在该网卡上，并配置 spec.interface.ipv4 地址

* subnet 的 spec.feature.enableSyncEndpoint.dhcpClient 开启后，tophub 会基于 dhcp client 的 IP 分配情况，自动创建 hoststatus 对象实例，其用于管理主机的 BMC 信息。subnet 的 spec.feature.enableSyncEndpoint.defaultClusterName 会设置给 hoststatus 对象实例，便于对主机进行分组管理

* subnet 的 spec.feature.enableBindDhcpIP 开启后，tophub 会基于 dhcp client 的 IP 分配情况，自动在 DHCP server 的配置文件中创建该 client 的 IP 和 Mac 地址，实现持久化绑定。

后续运维过程中，当该主机不活跃于网络中或者换了 IP 地址，希望绑定原 IP 和 mac 绑定关系，可删除原 hoststatus 实例，即可自动删除 DHCP server 的配置文件中的绑定关系。注意，如果主机基于活跃于网络中，删除 hoststatus 对象是不能实现 IP 和 Mac 的解绑的，因为 subnet 的 spec.feature.enableSyncEndpoint.dhcpClient==true 会再次创建出 hoststatus 对象，再次实现 P 和 Mac 的绑定

* dhcp server 配置的模板存在于 configmap topohub-dhcp 中，可进行自定义调整。

2. 管理基于 DHCP 接入的 BMC 

如果网络中存在刚接入或者未分配 IP 地址的 BMC 主机，上一步中创建的 DHCP server 会分配 IP 地址给这些主机，并且，topohub 会自动创建 hoststatus 对象实例，用于管理这些主机的 BMC 信息。


```bash
# 查看 hoststatus 实例，每个实例代表一个被纳管的 BMC 主机
# 确认 HEALTHY 状态为 true 表示主机已被成功纳管
~# kubectl get hoststatus -l topohub.infrastructure.io/mode=dhcp
NAME                    CLUSTERNAME    HEALTHY   IPADDR           TYPE           AGE
192-168-1-142           cluster1       true      192.168.1.142    dhcp           1m
192-168-1-173           cluster1       true      192.168.1.173    dhcp           1m

# 查看主机的详细信息，包括 redfish 获取的系统信息
~# kubectl get hoststatus 192-168-1-142 -o yaml
apiVersion: topohub.infrastructure.io/v1beta1
kind: HostStatus
metadata:
  name: 192-168-1-142
status:
  basic:
    activeDhcpClient: true
    clusterName: cluster1
    dhcpExpireTime: "2025-03-10T17:37:47Z"
    https: false
    ipAddr: 192.168.1.142
    mac: 96:25:7e:27:f2:97
    port: 8000
    secretName: topohub-redfish-auth
    secretNamespace: topohub
    subnetName: bmc-net-1
    type: dhcp
  healthy: true
  info:
    BiosVerison: P79 v1.45 (12/06/2017)
    BmcFirmwareVersion: 1.45.455b66-rev4
    BmcStatus: OK
    Cpu[0].Architecture: OEM
    ......
```

> 注意：
> * hoststatus 中的 status.info 信息是系统周期性从 BMC 主机获取的，默认周期为 60 秒。您可以通过设置 configmap topohub-feature 中的 redfishHostStatusUpdateInterval 来调整这个周期

> * topohub 在连接每个基于 dhcp 接入的主机时，都是会使用 helm 安装 topohub 时的 helm 选项 defaultConfig.redfish.username 和 defaultConfig.redfish.password 来连接 BMC 主机，这些认证信息存储在 secret topohub-redfish-auth 中，您可以通过修改该 secret 来修改默认的认证信息。

3. 查看 subnet 中 dhcp 分配 ip 的用量信息

```bash
~# kubectl get subnet net0 -o yaml
apiVersion: topohub.infrastructure.io/v1beta1
kind: Subnet
metadata:
  name: net0
spec:
  feature:
    enableBindDhcpIP: true
    enablePxe: true
    enableSyncEndpoint:
      defaultClusterName: cluster1
      dhcpClient: true
      endpointType: hoststatus
    enableZtp: false
  interface:
    interface: eth1
    ipv4: 192.168.1.3/24
    vlanId: 0
  ipv4Subnet:
    dns: 192.168.1.2
    gateway: 192.168.1.1
    ipRange: 192.168.1.100-192.168.1.200
    subnet: 192.168.1.0/24
status:
  conditions:
  - lastTransitionTime: "2025-02-08T12:24:09Z"
    message: dhcp server is hosted by node topohub-worker2
    reason: hostChange
    status: "True"
    type: DhcpServer
  # dhcpClientDetails 包含了所有被分配出去的 IP 地址和被绑定的 IP 地址
  dhcpClientDetails: '{"192.168.1.114":{"mac":"02:52:5c:17:7f:95","manualBind":false,"autoBind":true,"hostname":"vlan0-dhcp-redfish-mockup-578554878-tnxbv"},"192.168.1.173":{"mac":"a6:6d:a6:e5:1f:58","manualBind":false,"autoBind":true,"hostname":"vlan0-dhcp-redfish-mockup-578554878-jsrrc"},"192.168.1.199":{"mac":"00:00:00:00:00:11","manualBind":true,"autoBind":false,"hostname":"192-168-1-199"}}'
  dhcpStatus:
    # 当前活跃的 DHCP client IP 数量
    dhcpIpActiveAmount: 2
    # 基于 spec.feature.enableBindDhcpIP 功能，对自动绑定的 Mac 绑定的 IP 数量
    dhcpIpAutoBindAmount: 2
    # 当前子网中可用于分配的 IP 剩余数量
    dhcpIpAvailableAmount: 98
    # 所有被 Mac 绑定的 IP 数量，它包含了 dhcpIpAutoBindAmount 和 dhcpIpManualBindAmount
    dhcpIpBindAmount: 3
    # 基于 BindingIp CRD 实例绑定的 IP 数量
    dhcpIpManualBindAmount: 1
    # dpch server 的 总 IP 数量
    dhcpIpTotalAmount: 101
  hostNode: topohub-worker2
```

### 手动绑定 DHCP 分配的 IP 地址

在主机未接入 DHCP 前，可以创建配置，基于主机的 MAC 地址来预先未绑定即将分配的 IP 地址

1. 创建如下 bindingIp 对象

```bash
cat <<EOF | kubectl apply -f -
apiVersion: topohub.infrastructure.io/v1beta1
kind: BindingIp
metadata:
  name: 192-168-1-199
spec:
  # 该值对应了希望生效的 subnet 对象的名字
  subnet: net0
  # 该值对应了希望绑定的 IP 地址，其务必属于 spec.subnet 对象的 ipRange
  ipAddr: 192.168.1.199
  # 该值对应了希望绑定的主机的网卡 MAC 地址
  macAddr: 00:00:00:00:00:11
EOf
```

2. 查看子网的状态

```bash
~# kubectl get subnet net0 -o yaml
apiVersion: topohub.infrastructure.io/v1beta1
kind: Subnet
metadata:
  name: net0
...
status:
  # dhcpClientDetails 包含了所有被分配出去的 IP 地址和被绑定的 IP 地址
  dhcpClientDetails: '{"192.168.1.114":{"mac":"02:52:5c:17:7f:95","manualBind":false,"autoBind":true,"hostname":"vlan0-dhcp-redfish-mockup-578554878-tnxbv"},"192.168.1.173":{"mac":"a6:6d:a6:e5:1f:58","manualBind":false,"autoBind":true,"hostname":"vlan0-dhcp-redfish-mockup-578554878-jsrrc"},"192.168.1.199":{"mac":"00:00:00:00:00:11","manualBind":true,"autoBind":false,"hostname":"192-168-1-199"}}'
  dhcpStatus:
    # 当前活跃的 DHCP client IP 数量
    dhcpIpActiveAmount: 2
    # 基于 spec.feature.enableBindDhcpIP 功能，对自动绑定的 Mac 绑定的 IP 数量
    dhcpIpAutoBindAmount: 2
    # 当前子网中可用于分配的 IP 剩余数量
    dhcpIpAvailableAmount: 98
    # 所有被 Mac 绑定的 IP 数量，它包含了 dhcpIpAutoBindAmount 和 dhcpIpManualBindAmount
    dhcpIpBindAmount: 3
    # 基于 BindingIp CRD 实例绑定的 IP 数量
    dhcpIpManualBindAmount: 1
    # dpch server 的 总 IP 数量
    dhcpIpTotalAmount: 101
```

### 手动创建主机对象来管理 BMC 主机

对于已经分配 IP 地址的 BMC 主机，您可以使用以下方式创建主机对象

```bash
NAME=device10
BMC_IP_ADDR=10.64.64.42
cat <<EOF | kubectl apply -f -
apiVersion: topohub.infrastructure.io/v1beta1
kind: HostEndpoint
metadata:
  name: ${NAME}
spec:
  ipAddr: "${BMC_IP_ADDR}"
  # 设置改主机的 clusterName，便于进行分组管理
  clusterName: cluster1
EOF
```

注意，topohub 在连接每个主机时，都是会使用 helm 安装 topohub 时的 helm 选项 defaultConfig.redfish.username 和 defaultConfig.redfish.password 来连接 BMC 主机，这些认证信息存储在 secret topohub-redfish-auth 中。如果接入的主机使用了不同的认证信息，可需要创建 secret 对象来存储认证信息，参考如下 

```bash
NAME=device10
USERNAME=root
PASSWORD=admin
BMC_IP_ADDR=10.64.64.42
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${NAME}
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
  ipAddr: "${BMC_IP_ADDR}"
  # 设置改主机的 clusterName，便于进行分组管理
  clusterName: cluster1
  secretName: "${NAME}"
  secretNamespace: "topohub"
EOF
```

创建主机对象后，BMC agent 会自动生成对应的 hoststatus 对象，并开始同步主机的 redfish 信息：

```bash
# 查看手动创建的主机对象状态
~#  
NAME                CLUSTERAGENT       HOSTIP
device10            cluster1     10.64.64.42

# 查看所有主机的状态，确认新添加的主机状态为 HEALTHY
~# kubectl get hoststatus -l topohub.infrastructure.io/mode=hostendpoint
NAME                    CLUSTERNAME    HEALTHY   IPADDR           TYPE           AGE
device10                cluster1        true      10.64.64.42      hostendpoint   1m
```

> 对于老的 BMC 系统，它的 tls 版本很低，证书套件很老，导致 gofish 无法正常建立链接
> 更新了 secret 账户和密码，会立即生效
> 目前版本，只支持新建或者删除 HostEndpoint，不支持编辑

### BMC 主机电源操作

完成主机接入后，您可以对主机进行电源管理等操作，具体请参考 [主机操作](./action.md) 章节。

### 故障运维

1. 查看 hoststatus 对象的 HEALTHY 健康状态，如果不健康，代表这该主机无法正常访问 BMC，也许是 IP 地址不对，也许是 BMC 用户名密码不对，也许是 BMC 主机不支持 redfish 协议，因此，需要人为进行排查故障

2. 查看 BMC 主机的日志

```bash
# 获取所有 BMC 主机的日志
kubectl get events -n topohub --field-selector reason=BMCLogEntry
    LAST SEEN   TYPE      REASON        OBJECT                                      MESSAGE
    30s         Warning   BMCLogEntry   hoststatus/bmc-clusteragent-192-168-0-100   [2012-03-07T14:45:00Z][Critical]:  Temperature threshold exceeded
    2m13s       Warning   BMCLogEntry   hoststatus/bmc-clusteragent-192-168-0-101   [2012-03-07T14:45:00Z][Critical]:  Temperature threshold exceeded
    105s        Normal    BMCLogEntry   hoststatus/device-safe                      [2018-08-31T13:33:54+00:00][]:  [ PS1 Status ] Power Supply Failure

# 获取指定 BMC 主机的日志
kubectl get events -n topohub --field-selector reason=BMCLogEntry,involvedObject.name=${HoststatusName}

# 获取指定 BMC 主机的日志统计
kubectl get hoststatus ${HoststatusName} -n topohub -o jsonpath='{.status.log}' | jq .
  {
    "lastestLog": {
      "message": "[2024-10-16T22:47:28Z][Critical]:  [GS-0002] GPU Temp, 6 is not present",
      "time": "2024-10-16T22:47:28Z"
    },
    "lastestWarningLog": {
      "message": "[2024-10-16T22:47:28Z][Critical]:  [GS-0002] GPU Temp, 6 is not present",
      "time": "2024-10-16T22:47:28Z"
    },
    "totalLogAccount": 52,
    "warningLogAccount": 35
  }

```

## 管理主机的带内网络

该功能，可实现对主机操作系统的带内网络的 IP 管理、PXE 引导装机等功能

### 创建 dhcp server 来管理带内子网

1. 创建 Subnet 实例，它会在 topohub 运行节点的网卡上启动 DHCP 服务，能够为主机分配 IP

```bash
NAME=os-net-1
CIDR=172.16.1.0/24
IP_RANGE=172.16.1.100-172.16.1.200
GATEWAY=172.16.1.1
INT_VLAN_ID=20
INT_IPV4=172.168.1.2/24
cat <<EOF | kubectl apply -f -
apiVersion: topohub.infrastructure.io/v1beta1
kind: Subnet
metadata:
  name: ${NAME}
spec:
  ipv4Subnet:
    # DHCP 子网号
    subnet: "${CIDR}"
    # DHCP 可分配 IP 地址
    ipRange: "${IP_RANGE}"
    # 该子网的网关
    gateway: "${GATEWAY}"
  interface:
    # 在子网的 vlan ID
    vlanId: ${INT_VLAN_ID}
    # 请分配一个可用 IP 地址，作为被 DHCP server 的工作 IP 地址
    ipv4: "${INT_IPV4}"
  feature:
    enableSyncEndpoint:
      dhcpClient: false
    enableBindDhcpIP: true
    enablePxe: true
EOF
```

### PXE 引导装机

在创建了具备 spec.feature.enablePxe == true 的 subnet 之后，其 dhcp server 在分配 IP 地址时，能够传递 PXE 引导信息，实现 PXE 安装操作系统

1. 通过 topohub 的 file browser 服务，上传 ISO 操作系统镜像文件 

访问 topohub 运行的主机 IP，即可访问 file browser 的 webui 服务，在 http/iso 目录下上传 ISO 文件

> 注：安装 topohub 后，file browser 默认认证账户为 admin，密码为 admin
> 注：subnet 对象开启了 spec.feature.enablePxe == true ，会开启 tftp 服务，它的工作目录位默认挂载到 POD 的 `/var/lib/topohub/tftp` 目录下，且该目录下默认内置了一个引导文件 "/var/lib/topohub/tftp/boot/grub/x86_64-efi/core.efi"

2. 对于新接入的主机，就会自动镜像 PXE 装机

3. 对于已经安装了操作系统的主机，如果希望重装操作系统，可给其对应的 hoststatus 对象下发 bmc 的 PXE 重启指令，实现 PXE 重启

```bash
cat <<EOF | kubectl create -f -
apiVersion: topohub.infrastructure.io/v1beta1
kind: HostOperation
metadata:
  name: host1-pxe-restart
spec:
  action: "PxeReboot"
  # 该对象的名字对应了 hoststatus 对象的名字
  hostStatusName: "host1"
EOF
```
