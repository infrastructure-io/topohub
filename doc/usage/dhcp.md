# DHCP Server

## 功能说明

topohub 可以创建 subnet 对象，实现在不同子网上启动 DHCP server 服务，它能够做到如下功能

* 分配 IP 地址
* 支持把 DHCP client 的 IP 固定到 DHCP server 的配置中， 从而实现 DHCP client 的 IP 固定。
* 支持在分配 IP 的响应中提供 PXE 服务选项，能开启 tftp 服务，从而支持 PXE 安装操作系统
* 支持交换机的 ZTP 配置服务

## 快速开始

## 功能说明

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

### 同步维护 dhcp client 的 redfishstatus 和 绑定 IP 地址

```
apiVersion: topohub.infrastructure.io/v1beta1
kind: Subnet
spec:
  feature:
    syncRedfishstatus:
      enabled: true  # 基于 dhcp server 分配的 ip， 如果能成功访问其 bmc ，就会创建出 redfishstatus 对象
      defaultClusterName: cluster1
      enableBindDhcpIP: true  # 如果能够成功创建出 redfishstatus 对象，就会自动创建出 bindingIp 对象。当 redfishstatus 被删除时，其对应的 bindingIp 对象也会被删除。

```

如果希望删除某个 Redfishstatus 和其 bindingIp （自动级联删除）对象。确保该 Redfishstatus 对象在网络中真实不工作了，否则，请手动删除 /var/lib/topohub/dhcp/lease 中的 IP 分配记录，再删除 Redfishstatus 对象。如果不这么做，syncRedfishstatus.enabled 会使得 topohub 基于 dhcp 分配 ip 的记录，在确认其能够正常登录 bmc ， 会再次创建出 Redfishstatus 和 bindingIp

### 故障排查

如果 POD 使用 hostpath 存储，则 DHCP server 的目录默认位于 /var/lib/topohub/dhcp/, 否则位于 PVC 中
存储目录的 dhcp 目录下，有如下子目录
1. config目录：目录中存储了以 subnet 名字命名的 DHCP server 的配置文件
2. leases目录：目录中存储了以 subnet 名字命名的 lease 文件，存储了 DHCP client 的 IP 分配记录
3. log 目录：目录中存储了以 subnet 名字命名的日志文件

