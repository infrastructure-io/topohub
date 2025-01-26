# HostOperation 操作指南

本文档介绍了如何使用 HostOperation CRD 来管理物理机的电源状态。

## 支持的操作类型

HostOperation CRD 支持以下操作类型：

| Action | 描述 | 使用场景 |
|--------|------|----------|
| On | 正常开机 | 物理机关机状态下，需要启动时 |
| ForceOn | 强制开机 | 物理机可能处于异常关机状态时 |
| ForceOff | 强制关机，强制操作会立即执行，可能导致数据丢失 | 物理机无法正常关机时 |
| GracefulShutdown | 优雅关机，优雅操作会等待操作系统完成清理工作 | 正常关闭物理机，等待操作系统完成清理 |
| ForceRestart | 强制重启，强制操作会立即执行，可能导致数据丢失 | 物理机系统无响应需要强制重启时 |
| GracefulRestart | 优雅重启，优雅操作会等待操作系统完成清理工作 | 正常重启物理机，等待操作系统完成清理 |
| PxeReboot | PXE 重启，PXE 重启是实现 once 重启，即重启后。需要管理员在带内网络内手动部署 PXE 服务，本组件并不自动部署 PXE 服务 | 需要通过 PXE 引导安装系统时 |

## 操作流程

### 前提条件

1. 确保物理机已经被正确注册到系统中（存在对应的 HostStatus CR）
2. 确保物理机状态健康（HostStatus 的 status.healthy 为 true）
3. 确保有足够的权限执行这些操作

### 操作步骤

1. 查看当前可操作的物理机列表：
```bash
~# kubectl get hoststatus
NAME                             CLUSTERAGENT       HEALTHY   IPADDR          TYPE           AGE
bmc-clusteragent-host1           bmc-clusteragent   true      10.64.64.42     hostEndpoint   44s
bmc-clusteragent-192-168-0-100   bmc-clusteragent   true      192.168.0.100   dhcp           64s
```

2. 创建 HostOperation， 每个实例代表了一次对主机的相关操作：

创建操作：
```bash
cat <<EOF | kubectl create -f -
apiVersion: topohub.infrastructure.io/v1beta1
kind: HostOperation
metadata:
  name: host1-restart
spec:
  action: "GracefulRestart"
  hostStatusName: "bmc-clusteragent-host1"
EOF
```

> 注意：
> 1. spec.action 的值，必须是小节 [支持的操作类型](#支持的操作类型) 中的一种
> 2. spec.hostStatusName 的值，必须是步骤 1 中获取的已存在 hoststatus 实例的名字

3. 查看操作状态：
```bash
# 查看操作的完成状态
kubectl get hostoperation

```

操作状态可以通过 `status.status` 字段查看：

| 状态 | 描述 |
|------|------|
| pending | 操作正在执行中 |
| success | 操作执行成功 |
| failed | 操作执行失败 |
