# DhcpSubnet

## CRD 定义和校验

在 pkg/k8s/apis/topohub.infrastructure.io/v1beta1/dhcpsubnet_types.go 文件下，我希望定义新的 crd，如下 

```
apiVersion: topohub.infrastructure.io/v1beta1
kind: DhcpSubnet
metadata:
    name: test
spec:
    ipv4Subnet:
        # Subnet for DHCP server (必选字段，格式校验)
        subnet: "192.168.0.0/24"
        # IP range for DHCP server (必选字段，格式校验)
        ipRange: "192.168.0.10-192.168.0.100,192.168.0.105"
        # Gateway for DHCP server (可选字段，格式校验)
        gateway: "192.168.0.1"
        # dns server (可选字段，格式校验)
        dns: "8.8.8.8"
    interface:
        # DHCP server interface (required) ,该网卡应该在主机上存在
        interface: "net1"
        # VLAN ID (可选字段，格式校验， 0-4094) ， 如果配置了本选项，会在 interface 网上创建对应的 vlan 子接口
        vlanId: 100
        # Self IP for DHCP server (必选字段，格式校验)，本 IP 地址会配置在 interface 或者 其上的 vlan 子接口中
        ipv4: "192.168.0.2/24"
    feature:
      enableSyncEndpoint:
        # 是否基于 DHCP client ip 分配情况，自动创建或者删除对应的 Endpoint 对象 
        dhcpClient: true
        # 是否周期主动扫描子网内的所有 ip，自动创建或者删除对应的 Endpoint 对象 
        scanEndpoint: true
        # 设置到同步创建的 Endpoint 对象的 ClusterName 字段
        DefaultClusterName: ""

      # 把 DHCP 分配的 ip 在 dhcp server 配置中进行 IP 绑定，避免 IP 发生变化
      enableBindDhcpIP: true

      # 把非 DHCP 分配IP 的 其它 Endpoint 的 ip，在 dhcp server 中进行 IP 预留，避免IP分配冲突
      enableReservceNoneDhcpIP: true

      # 对于主机的 带内子网，开启本功能后，支持 pxe 引导装机
      enablePxe: true

      # 对于交换机的管理子网，开启本功能后，支持 ZTP 配置下发
      enableZtp: true

      # 对于主机的 带外子网，开启本功能后，支持 redfish 获取信息
      enableRedfish: true

status:
  IpTotalAmount: 0
  IpAvailableAmount: 0
  IpAssignAmount: 0
  IpReservedAmount: 0

```

相关的 webhook 校验逻辑，可参考 pkg/webhook/hostendpoint/webhook.go ， 在 pkg/webhook/dhcpsubnet/webhook.go 中实现

对于 crd DhcpSubnet ，kubectl get -o wide 时，希望显示相应的 spec.ipv4Subnet.subnet , spec.feature.enablePxe , spec.feature.enableZtp , status.IpTotalAmount , status.IpAvailableAmount , status.IpAssignAmount , status.IpReservedAmount

在  文件中 
1 ValidateCreate 函数中  
验证 spec.ipv4Subnet.ipRange 中的 所有 ip 地址 ，需要属于 子网 spec.ipv4Subnet.subnet ； 
验证 spec.ipv4Subnet.gateway 中的 所有 ip 地址 ，需要属于 子网 spec.ipv4Subnet.subnet ； 
验证 spec.interface.ipv4 中的 ip 地址 ，需要属于 子网 spec.ipv4Subnet.subnet， 子网号相同 ； 
验证 spec.interface.interface 网卡在本地 存在


spec.feature.enablePxe ， spec.feature.enableZtp , dhcpSubnet.Spec.FeatureEnableRedfish 不允许同时开启

## 模块

