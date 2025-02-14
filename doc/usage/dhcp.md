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

可编辑 configmap topohub-manualbinding 中的内容，输入如下一行样例，实现 IP 和 Mac 的绑定关系

```
192.168.1.186 00:00:00:00:00:11 host1
```

注意，配置中的 IP 地址如果属于某个 subnet 对象的 spec.ipv4Subnet.ipRange 范围内，那么这个 IP 地址会被 DHCP server 的配置文件中的绑定关系覆盖。
注意的，如果该手动设置的绑定关系与当前某个 DHCP client 的绑定关系不一致，该手动设定不会生效，以当前 DHCP client 的绑定关系优先生效

可通过查看 subnet 对象的 status 来进一步确认绑定生效状态


### 故障排查

如果 POD 使用 hostpath 存储，则 DHCP server 的目录默认位于 /var/lib/topohub/dhcp/, 否则位于 PVC 中
存储目录的 dhcp 目录下，有如下子目录
1. config目录：目录中存储了以 subnet 名字命名的 DHCP server 的配置文件
2. leases目录：目录中存储了以 subnet 名字命名的 lease 文件，存储了 DHCP client 的 IP 分配记录
3. log 目录：目录中存储了以 subnet 名字命名的日志文件

