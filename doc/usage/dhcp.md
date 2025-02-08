# DHCP Server

## 功能说明

Agent 中的 DHCP server

* 支持把 DHCP client 的 IP 固定到 DHCP server 的配置中， 从而实现 DHCP client 的 IP 固定。
* 支持在分配 IP 的响应中提供 PXE 服务选项，能开启 tftp 服务，从而支持 PXE 安装操作系统
* 支持交换机的 ZTP 配置服务

### DHCP server 的配置

DHCP server 的配置文件，位于 confimap bmc-dhcp-config 中，若有修改需求，设置后，再重启 agent pod

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


### 固定 dhcp client 的 IP 

```
apiVersion: topohub.infrastructure.io/v1beta1
kind: Subnet
spec:
  feature:
    enableBindDhcpIP: true
```

spec.enableBindDhcpIP 开启时，每当 DHCP server 分配一个 IP 地址后，同时会把该 client 的 IP 和 mac 地址的绑定关系写入到 dhcp server 的配置文件中，实现 IP 地址的固定。

只有删除相应的 hoststatus 对象后，才会删除 dhcp server 的配置中的绑定关系。不过请注意的是，对于 dhcp client 的 hoststatus，如果它还活跃在网络中，那么 spec.feature.enableSyncEndpoint.dhcpClient 的开启也会重新创建出 hoststatus 对象。因此，当某个主机活跃于网络中时，删除 IP 地址的绑定关系才有意义。

## 故障排查

如果 POD 使用 hostpath 存储，则 DHCP server 的目录默认位于 /var/lib/topohub/， 否则位于 PVC 中
存储目录的 dhcp 目录下，有如下子目录
1. config目录：目录中存储了以 subnet 名字命名的 DHCP server 的配置文件
2. leases目录：目录中存储了以 subnet 名字命名的 lease 文件，存储了 DHCP client 的 IP 分配记录
3. log 目录：目录中存储了以 subnet 名字命名的日志文件

