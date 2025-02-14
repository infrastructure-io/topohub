# DHCP Server

## 功能说明

topohub 可以创建 subnet 对象，实现在不同子网上启动 DHCP server 服务，它能够做到如下功能

* 分配 IP 地址
* 支持把 DHCP client 的 IP 固定到 DHCP server 的配置中， 从而实现 DHCP client 的 IP 固定。
* 支持在分配 IP 的响应中提供 PXE 服务选项，能开启 tftp 服务，从而支持 PXE 安装操作系统
* 支持交换机的 ZTP 配置服务

## 快速开始

## 功能说明

### 同步维护 dhcp client 的 hoststatus 

```
apiVersion: topohub.infrastructure.io/v1beta1
kind: Subnet
spec:
  feature:
    enableSyncEndpoint:
      dhcpClient: true
      defaultClusterName: cluster1
      endpointType: hoststatus
```

spec.enableBindDhcpIP 开启时，每当 DHCP server 分配一个 IP 地址后，同时会把该 client 的 IP 和 mac 地址的绑定关系写入到 dhcp server 的配置文件中，实现 IP 地址的固定。只有删除相应的 hoststatus 对象后，才会删除 dhcp server 的配置中的绑定关系。

### 自动固定 dhcp client 的 IP 

```
apiVersion: topohub.infrastructure.io/v1beta1
kind: Subnet
spec:
  feature:
    enableBindDhcpIP: true
```

spec.enableBindDhcpIP 开启时，每当 DHCP server 分配一个 IP 地址后，同时会把该 client 的 IP 和 mac 地址的绑定关系写入到 dhcp server 的配置文件中，实现 IP 地址的固定。

只有删除相应的 hoststatus 对象后，才会删除 dhcp server 的配置中的绑定关系。不过请注意的是，对于 dhcp client 的 hoststatus，如果它还活跃在网络中，那么 spec.feature.enableSyncEndpoint.dhcpClient 的开启也会重新创建出 hoststatus 对象。因此，当某个主机活跃于网络中时，删除 IP 地址的绑定关系才有意义。

### 手动固定 dhcp client 的 IP 

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

### 故障排查

如果 POD 使用 hostpath 存储，则 DHCP server 的目录默认位于 /var/lib/topohub/dhcp/, 否则位于 PVC 中
存储目录的 dhcp 目录下，有如下子目录
1. config目录：目录中存储了以 subnet 名字命名的 DHCP server 的配置文件
2. leases目录：目录中存储了以 subnet 名字命名的 lease 文件，存储了 DHCP client 的 IP 分配记录
3. log 目录：目录中存储了以 subnet 名字命名的日志文件

