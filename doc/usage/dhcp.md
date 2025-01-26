# DHCP Server

## 功能说明

Agent 中的 DHCP server，支持把 DHCP client 的 IP 固定到 DHCP server 的配置中， 从而实现 DHCP client 的 IP 固定。

### DHCP server 的配置

DHCP server 的配置文件，位于 confimap bmc-dhcp-config 中，若有修改需求，设置后，再重启 agent pod

### DHCP client 的 IP 固定

当`EnableDhcpDiscovery`功能开启时：

1. **普通动态分配** (`EnableBindDhcpIP = false`)：
   - DHCP服务器动态分配IP地址
   - IP分配变化会实时同步到 hoststatus 对象
   - 当网络中的 dhcp client 进行 IP 释放时，对应的 hoststatus 对象会被自动删除

2. **固定IP绑定** (`EnableBindDhcpIP = true`)：
   - 所有已分配的 DHCP IP 会被固化到 DHCP server 的配置中，其中实现 IP 地址和 MAC 地址的绑定
   - 当网络中的 dhcp client 进行新 IP 分配时，会创建对应的hoststatus对象
   - 当网络中的 dhcp client 进行 IP 释放时，不会自动删除对应的 hoststatus 对象
   - 当需解除 DHCP server 配置中的 IP 和 MAC 地址的绑定，可按照如下流程：
     首先，进入 agent pod 中，查看 DHCP server 的实时 IP 分配文件 /var/lib/dhcp/bmc-clusteragent-dhcpd.leases ， 确认和删除其中期望解除绑定的 IP 地址；其次，`kubectl get hoststatus -l status.basic.ipAddr=<IP>` 查看 hoststatus 对象，确认其中的 IP 和 MAC 地址符合删除预期，然后手动删除对应的 hoststatus 对象 `kubectl delete hoststatus -l status.basic.ipAddr=192.168.0.101` ；最终，后端会自动更新 DHCP server 的配置，实现 IP 和 MAC 地址的解绑 ( 可进入 agent pod 中，查看 文件 /etc/dhcp/dhcpd.conf 确认)

### 通过 hostendpoint 对象创建的静态 IP 的固定

当手动创建 hostendpoint 对象时，如果 hostendpoint 的 IP 地址在 DHCP 服务器的子网范围内，且`EnableBindStaticIP`选项开启， 那么该 IP 地址会被自动添加到 DHCP 服务器的固定IP配置中

虽然这些静态 IP 不是通过 DHCP 服务器分配的，但是通过 hostendpoint 对象创建的静态 IP 也可以被固定到 DHCP 服务器的配置中， 从而实现 IP 固定，或者避免 IP 分配的冲突。

## 故障排查

agent 的 DHCP 服务器的配置，存储在 Agent 的 /etc/dhcp/dhcpd.conf 文件中

