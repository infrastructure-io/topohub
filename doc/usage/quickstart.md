# 快速开始

## Topohub 介绍

Topohub 是一套管理基础设施的 Kubernetes 组件，包括主机、交换机等，具体包括如下功能

* 为不同子网启动 dhcp server 多实例，并且支持为 dhcp client 分配的 IP 地址自动固定 IP 和 Mac。
   可管理主机的带内网络，分配 bmc 的 ip 地址
   可管理主机的带外网络，分配 OS 的 ip 地址，并且 dhcp server 支持 PXE 引导装机

* 主机带外网络的 BMC 管理 
    能基于 dhcp server 分配 IP 地址，或者手动配置，自动管理主机的 BMC，包括信息获取、重启、PXE 重启等

* 主机的 PXE 装机
    支持在带内子网中实现 PXE 引导安装操作系统

* 基于 WEBUI 进行后端文件管理，包括了 IOS 文件、tftp 文件、ZTP 文件、dhcp server 配置文件等

## 安装前提条件

您需要一个可用的 Kubernetes 集群

1. 集群中至少有一个节点能够访问到待纳管主机的带内网络、带外网络、交换机网络
2. 提供 PVC 或者足够的磁盘，用于存储配置文件和 ISO 文件

## 安装 Topohub 组件

1. Topohub 组件以 hostnetwork 模式运行在主机上，寻找一个或者两个节点，它们都有一个网卡以 trunk 模式接入到交换机，具备访问主机的带内、主机带外、交换机等网络的 vlan。给这些节点设置如下 label，作为运行 Topohub 的节点

```bash
kubectl label node <node-name> infrastructure.io/deploy=true
```

> 注意：Topohub 组件以 hostnetwork 模式运行在主机上，默认 Topohub 组件会占用端口 80、8080、8081、8082，请确保这些端口可用

2. 使用 helm 安装 Topohub

```bash

helm repo add topohub https://infrastructure-io.github.io/topohub
helm repo update

# 创建配置文件
cat << EOF > values.yaml
replicaCount: 1

# 中国用户，可以使用如下中国镜像源
#image:
#  registry: "ghcr.m.daocloud.io"

defaultConfig:
  redfish:
    # 连接主机 bmc 的默认账户名 
    username: "<<BmcDefaultUsername>>"
    # 连接主机 bmc 的默认账户密码
    password: "<<BmcDefaultPassword>>"
  dhcpServer:
    # 节点上能够访问所有管理设备网络的网卡名，其以 trunk 模式接入到交换机
    interface: "eth1"

storage:
  # POC 场景使用 hostPath， 生产环境请使用 pvc
  type: "hostPath"

fileBrowser:
  # 开启 filebrowser 服务，可提供 webui 来管理配置文件和 ISO 文件
  enabled: true

  # 中国用户，可以使用如下中国镜像源
  #image:
  #  registry: "docker.m.daocloud.io"
EOF

# 安装 Topohub 组件
helm install topohub topohub/topohub \
    --namespace topohub  --create-namespace  --wait \
    -f values.yaml

# 验证安装结果
kubectl get pod -n topohub
    NAME                      READY   STATUS    RESTARTS   AGE
    topohub-59db8c549-6x5m5   2/2     Running   0          10m

```

3. 如果安装了 fileBrowser 服务，可通过 topohub 运行主机的 IP 地址来访问其 webui，默认认证账户为 admin，密码为 admin，建议登录后修改默认密码
