# 快速开始

## 前提条件

1. 您需要一个可用的 Kubernetes 集群，且集群中至少有一个节点能够访问到待纳管主机的 BMC 网络

## 安装 BMC 组件

### 单集群纳管，以 hostnetwork 模式部署

BMC 组件内置了 DHCP server 功能，可以为 BMC 网络中的主机自动分配 IP 地址，从而实现主机的自动发现能力。因此，在 BMC 网络环境中没有 DHCP server 时，建议使用本安装模式。该模式既支持通过 DHCP 方式自动发现 BMC 主机，也支持通过静态 IP 方式纳管主机。

```bash
# 首先为节点打标签，标记该节点具备访问 BMC 网络的能力，这样可以确保 BMC agent 组件运行在该节点上
kubectl label node <node-name> topohub.infrastructure.io/bmcnetwork=true

helm repo add bmc https://infrastructure-io.github.io/bmc
helm repo update

# 创建配置文件
cat << EOF > my-values.yaml
# for china mirror
#global:
#  imageRegistryOverride: ghcr.m.daocloud.io

clusterAgent:
  agentYaml:
    hostNetwork: true
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: kubernetes.io/hostname
            operator: In
            values:
            - topohub.infrastructure.io/bmcnetwork
  feature:
    # 是否启用 DHCP server 功能，设置为 false 可以关闭
    enableDhcpServer: true
    # 当启用 DHCP server 时，需要配置以下参数
    dhcpServerConfig:
      dhcpServerInterface: "eth1"  # 指定主机上能够访问 BMC 网络的网卡名称
      subnet: "192.168.0.0/24"     # 指定 dhcpServerInterface 接口的子网，用于配置 DHCP server
      ipRange: "192.168.0.10-192.168.0.100"  # 指定 DHCP server 分配给纳管主机的 IP 地址范围
      gateway: "192.168.0.1"       # 指定 DHCP server 分配给纳管主机的网关地址
  endpoint:
    username: "admin"              # 指定所有纳管主机的默认 BMC 用户名
    password: "password"           # 指定所有纳管主机的默认 BMC 密码
  # 当启用 enableDhcpServer 时，需要配置数据存储方式
  storage:
    type: "pvc"                    # 指定 DHCP server 存储客户端分配 IP 数据的方式，支持 pvc（适用于生产环境）和 hostPath（适用于 POC 环境）
EOF

# 安装 BMC 组件
helm install bmc bmc/topohub \
    --namespace bmc  --create-namespace  --wait \
    -f my-values.yaml

# 验证安装结果
kubectl get pod -n bmc
NAME                                      READY   STATUS    RESTARTS   AGE
agent-bmc-clusteragent-6b9695698b-hphkj   1/1     Running   0          39m
bmc-topohub-7b4986f89c-bd9j9         1/1     Running   0          40m
```

### 多集群纳管，以 macvlan 模式部署

当您需要纳管多个 Kubernetes 集群中的主机时，建议采用 macvlan 网卡模式。这样，每个 agent 可以负责一个集群，它们可以运行在相同的主机上，通过 macvlan 接口共享主机的网卡，并且相互之间不会因为 hostnetwork 模式产生端口冲突。

部署步骤：

1. 首先确保已经安装了 spiderpool，使主机具备 macvlan CNI 能力
2. 然后安装 BMC 组件：

```bash
# 为节点打标签，标记该节点具备访问 BMC 网络的能力
kubectl label node <node-name> topohub.infrastructure.io/bmcnetwork=true

helm repo add bmc https://infrastructure-io.github.io/bmc
helm repo update

# 创建配置文件
cat << EOF > my-values.yaml
# for china mirror
#global:
#  imageRegistryOverride: ghcr.m.daocloud.io

clusterAgent:
  agentYaml:
    hostNetwork: false
    underlayInterface: "spiderpool/eth0-macvlan"  # 为 agent pod 配置 multus 的 secondary 网卡
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: kubernetes.io/hostname
            operator: In
            values:
            - topohub.infrastructure.io/bmcnetwork
  feature:
    enableDhcpServer: true
    dhcpServerConfig:
      dhcpServerInterface: "net1"  # 指定 POD 中的 secondary macvlan 网卡名，multus 默认取名为 net1
      subnet: "192.168.0.0/24"     # 指定 net1 接口的子网，用于配置 DHCP server
      ipRange: "192.168.0.10-192.168.0.100"  # 指定 DHCP server 分配给纳管主机的 IP 地址范围
      gateway: "192.168.0.1"       # 指定 DHCP server 分配给纳管主机的网关地址
      selfIp: "192.168.0.2/24"     # （可选）配置后，会将 net1 网卡的 IP 地址设置为该值，从而解耦 CNI 的 IPAM
  endpoint:
    username: "admin"              # 指定所有纳管主机的默认 BMC 用户名
    password: "password"           # 指定所有纳管主机的默认 BMC 密码
  storage:
    type: "pvc"                    # 指定 DHCP server 存储客户端分配 IP 数据的方式
EOF

# 安装 BMC 组件
helm install bmc bmc/topohub \
    --namespace bmc  --create-namespace  --wait \
    -f my-values.yaml

# 验证安装结果
kubectl get pod -n bmc
NAME                                      READY   STATUS    RESTARTS   AGE
agent-bmc-clusteragent-6b9695698b-hphkj   1/1     Running   0          39m
bmc-topohub-7b4986f89c-bd9j9         1/1     Running   0          40m
```

## 接入主机

1. 首先确认 agent 状态

BMC 组件支持运行多个 agent 来纳管多个集群。安装完成后，系统默认会运行一个 agent 实例，该实例由 clusteragent CRD 资源来表示：

```bash
# 查看 agent 实例状态
~# kubectl get clusteragent
NAME               READY
bmc-clusteragent   true

# 查看 agent 和 operator 的 Pod 状态
~# kubectl get pod -n bmc
NAME                                      READY   STATUS    RESTARTS   AGE
agent-bmc-clusteragent-6b9695698b-hphkj   1/1     Running   0          39m
bmc-topohub-7b4986f89c-bd9j9         1/1     Running   0          40m

# 查看 agent 实例的详细配置
~# kubectl get clusteragent bmc-clusteragent -o yaml
apiVersion: topohub.infrastructure.io/v1beta1
kind: ClusterAgent
metadata:
  name: bmc-clusteragent
spec:
  agentYaml:
    hostNetwork: false
    image: ghcr.io/infrastructure-io/topohub-agent:f069c88
    replicas: 1
    underlayInterface: spiderpool/eth0-macvlan
  endpoint:
    https: false
    port: 8000
  feature:
    dhcpServerConfig:
      dhcpServerInterface: net1
      enableDhcpDiscovery: true
      enableBindDhcpIP: true
      enableBindStaticIP: true
      gateway: 192.168.0.1
      ipRange: 192.168.0.100-192.168.0.200
      selfIp: 192.168.0.2/24
      subnet: 192.168.0.0/24
    enableDhcpServer: true
status:
  ready: true
```

2. 检查 DHCP 自动发现的主机状态

如果您启用了 BMC 组件的 DHCP server 功能，系统会自动发现和纳管 BMC 网络中的主机：

```bash
# 查看 hoststatus 实例，每个实例代表一个被纳管的 BMC 主机
# 确认 HEALTHY 状态为 true 表示主机已被成功纳管
~# kubectl get hoststatus -l topohub.infrastructure.io/mode=dhcp
NAME                             CLUSTERAGENT       HEALTHY   IPADDR          TYPE           AGE
bmc-clusteragent-192-168-0-100   bmc-clusteragent   true      192.168.0.100   dhcp           48m
bmc-clusteragent-192-168-0-101   bmc-clusteragent   true      192.168.0.101   dhcp           48m

# 查看主机的详细信息，包括 redfish 获取的系统信息
~# kubectl get hoststatus bmc-clusteragent-192-168-0-100 -o yaml
apiVersion: topohub.infrastructure.io/v1beta1
kind: HostStatus
metadata:
  name: bmc-clusteragent-192-168-0-100
status:
  basic:
    https: false
    ipAddr: 192.168.0.100
    mac: ce:b9:cc:10:30:26
    port: 8000
    type: dhcp
  clusterAgent: bmc-clusteragent
  healthy: true
  info:
    BiosVerison: P79 v1.45 (12/06/2017)
    BmcFirmwareVersion: 1.45.455b66-rev4
    BmcStatus: OK
    Cpu[0].Architecture: OEM
    ......
```

> 注意：
> 1. hoststatus 中的 status.info 信息是系统周期性从 BMC 主机获取的，默认周期为 60 秒
> 2. 您可以通过设置 agent pod 的环境变量 HOST_STATUS_UPDATE_INTERVAL 来调整这个周期
> 3. 或者在 helm 安装时通过 clusterAgent.feature.hostStatusUpdateInterval 参数来设置
> 4. agent 使用 dhcpd 来实现 DHCP server 功能，如果您需要调整 dhcpd 的配置，可以修改 configmap ${helm-release-name}-dhcp-config

3. 手动添加非 DHCP 接入的主机

当您通过 helm 安装 BMC 组件时，如果设置了 clusterAgent.endpoint.username 和 clusterAgent.endpoint.password，系统会创建一个包含这些认证信息的 secret：

```bash
# 查看认证信息 secret
~# kubectl get secrets -n bmc
NAME                        TYPE                 DATA   AGE
bmc-credentials             Opaque                3      71m
```

对于使用这些默认认证信息的主机，您可以使用以下方式创建主机对象：

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
EOF
```

对于使用不同认证信息的主机，您需要先创建包含认证信息的 secret，然后再创建主机对象：

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
  namespace: bmc
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
  secretName: "${NAME}"
  secretNamespace: "bmc"
EOF
```

创建主机对象后，BMC agent 会自动生成对应的 hoststatus 对象，并开始同步主机的 redfish 信息：

```bash
# 查看手动创建的主机对象状态
~# kubectl get hostendpoint
NAME                CLUSTERAGENT       HOSTIP
device10            bmc-clusteragent   10.64.64.42

# 查看所有主机的状态，确认新添加的主机状态为 HEALTHY
~# kubectl get hoststatus -l topohub.infrastructure.io/mode=hostEndpoint
NAME                             CLUSTERAGENT       HEALTHY   IPADDR          TYPE           AGE
bmc-clusteragent-10-64-64-42     bmc-clusteragent   true      10.64.64.42     hostEndpoint    1m
```

> 对于老的 BMC 系统，它的 tls 版本很低，证书套件很老，导致 gofish 无法正常建立链接
> 果更新了 secret 账户和密码，会立即生效
> 目前版本，只支持新建或者删除 HostEndpoint，不支持编辑

## 主机操作

完成主机接入后，您可以对主机进行电源管理等操作，具体请参考 [主机操作](./action.md) 章节。

## 故障运维

1. 查看 hoststatus 对象的 HEALTHY 健康状态，如果不健康，代表这该主机无法正常访问 BMC，也许是 IP 地址不对，也许是 BMC 用户名密码不对，也许是 BMC 主机不支持 redfish 协议，因此，需要人为进行排查故障

```bash
# kubectl get hoststatus
NAME                             CLUSTERAGENT       HEALTHY   IPADDR          TYPE           AGE
bmc-clusteragent-192-168-0-101   bmc-clusteragent   true      192.168.0.101   dhcp           2d14h
device-safe                      bmc-clusteragent   true      10.64.64.42     hostEndpoint   2d14h
gpu                              bmc-clusteragent   true      10.64.64.94     hostEndpoint   2d14h
test-hostendpoint                bmc-clusteragent   true      192.168.0.50    hostEndpoint   2d14h
```

2. 对于 DHCP 接入的主机，当使用绑定 IP 和 MAC 功能时，当期望解除 IP 和 MAC 的绑定，可按照如下流程：

    1. 进入 agent pod 中，查看 DHCP server 的实时 IP 分配文件 `/var/lib/dhcp/bmc-clusteragent-dhcpd.leases`，确认和删除其中期望解除绑定的 IP 地址
    2. `kubectl get hoststatus -l status.basic.ipAddr=<IP>` 查看 hoststatus 对象，确认其中的 IP 和 MAC 地址符合删除预期，然后手动删除对应的 hoststatus 对象 `kubectl delete hoststatus -l status.basic.ipAddr=192.168.0.101`
    3. 后端会自动更新 DHCP server 的配置，实现 IP 和 MAC 地址的解绑（可进入 agent pod 中，查看文件 `/etc/dhcp/dhcpd.conf` 确认）

3. 查看 BMC 主机的日志

```bash
# 获取所有 BMC 主机的日志
kubectl get events -n bmc --field-selector reason=BMCLogEntry
    LAST SEEN   TYPE      REASON        OBJECT                                      MESSAGE
    30s         Warning   BMCLogEntry   hoststatus/bmc-clusteragent-192-168-0-100   [2012-03-07T14:45:00Z][Critical]:  Temperature threshold exceeded
    2m13s       Warning   BMCLogEntry   hoststatus/bmc-clusteragent-192-168-0-101   [2012-03-07T14:45:00Z][Critical]:  Temperature threshold exceeded
    105s        Normal    BMCLogEntry   hoststatus/device-safe                      [2018-08-31T13:33:54+00:00][]:  [ PS1 Status ] Power Supply Failure

# 获取指定 BMC 主机的日志
kubectl get events -n bmc --field-selector reason=BMCLogEntry,involvedObject.name=${HoststatusName}

# 获取指定 BMC 主机的日志统计
kubectl get hoststatus ${HoststatusName} -n bmc -o jsonpath='{.status.log}' | jq .
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
