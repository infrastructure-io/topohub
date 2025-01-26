# readme


dhcp 固定IP 

在开启 EnableDhcpDiscovery 功能情况下：
1。 对于 dhcp client IP 的固定 
当关闭 EnableBindDhcpIP 选项，dhcp server 分配的 ip 变化，都会实时同步创建和删除 hoststatus 对象

当开启 EnableBindDhcpIP 选项时，所有曾经 分配的 DHCP ip 都会被固化到 dhcp server 的工作配置中，实现 IP 和 mac 的绑定。因此，dhcp server 分配的 ip 变化，会实时同步创建对应的 hoststatus 对象，但不会自动删除对应的 hoststatus 对象。如果希望解除该 IP 和 mac 的绑定，可手动 删除相关的 hoststatus 对象，后端自动实现 DHCP server 中的该 IP 绑定


2。 对于 static IP 的固定 
对于 手动创建的 hostendpoint 对象，如果其 IP 是 dhcp server 的子网，那么 ，当开启 

通过原理，通过监控 dhcp lease 文件中的 client ip 分配情况，从而控制是否 上报事件，来决定是否创建或者删除对应的 hoststatus 对象。
通过监控 hoststatus 对象，来决定是否 要 更新 dhcp server 的 配置文件来 固定 IP 



排障

进入 agent 的 hostpath 路径 或者 pvc， 查看 存储的 dhcp 配置  和 lease 分配 


/# ls var/lib/dhcp
bmc-clusteragent-dhcpd.conf  bmc-clusteragent-dhcpd.leases



bmc-clusteragent-dhcpd.conf 其中 是 生效了 固定 ip 的配置 

bmc-clusteragent-dhcpd.leases 其中是 dhcp server 实时 分配的 ip 
 
