# agent

## yaml

- 需要 controller 的 生成 agent deployment 代码中，配置 健康检查的端口和配置， agent 通过 启动的命令参数，在指定的端口上 进行 启动 http server 提供健康检查


## yaml

- 需要 controller 的 生成 agent deployment 代码中，配置 健康检查的端口和配置， agent 通过 启动的命令参数，在指定的端口上 进行 启动 http server 提供健康检查


## 启动

agent 启动的初始代码中，获取 环境变量 CLUSTERAGENT_NAME 的值， 通过该来 作为 name ，去获取 相应的 clusteragent 实例的 yaml 定义， 使用其 spec.endpoint 和 spec.feature 中的 内容 生成 一组 struct config 配置，作为后续工作的基础

type agentConfig struct {
  ClusterAgentName string
  Endpoint         *EndpointConfig
  Feature          *FeatureConfig      
}

获取 config 后，需要对其进行 如下 校验
（1） spec.endpoint.https 如果该值为 true，需要 spec.endpoint.secretName 和 spec.endpoint.secretNamespace 必须有值， 否则 程序报错退出
（2） spec.endpoint.secretName 代表 secret name ， spec.endpoint.secretNamespace 代表 secret namespace， 如果他们有值，需要确认该 secret 是否存在，其中 要求 存在 tls 认证的 key 和 cert ，但不 要求一定有 ca   。  否则 程序报错退出
（3）spec.feature. dhcpServerInterface 必须有值，且确认该值所代表的网卡  是否存在于 网络接口中，且网卡 up 状态， 否则 程序报错退出


## dhcp server 模块

在 pkg/dhcpserver 下 实现一个 接口模块，使用 interface 对外暴露，  它应该具备如下方法，并实现它们

1 启动 dhcp server 接口
    它基于 dhcpd 二进制来启动 dhcp server 服务

    它基于 AgentConfig.objSpec.Feature.DhcpServerConfig 中的参数 工作
    模块的参数，工作的网卡名参数（必备），暴露 dhcp 分配 ip 的子网 参数（必备），可分配 ip 参数（必备） , 分配给 client 端的 子网网关 ip 参数（必备）。 这些参数 传递给 dhcpd 进行工作
    
    AgentConfig.objSpec.Feature.DhcpServerConfig.selfIp 如果有值， 那么 把 网卡 AgentConfig.objSpec.Feature.DhcpServerConfig.DhcpServerInterface  名参数所代表的 网卡上的 IP 地址 去除，然后 用 网卡 ip 参数 代替 生效

    模块一直监控 dhcpd 的运行，当 它 故障时，能够再次 尝试 拉起 它 

    模块一直监控 dhcpd 的运行，当 分配 或者 释放 一个 ip 时，有 日志 输出，该行日志中 包括 可分配 ip 的 总量和剩余量


2 获取 client 信息接口 
   接口输出 获取 dhcpd 分配出的 所有 client 的 ip 地址 和 对应的 mac 的列表

3 获取 ip 用量统计信息
   接口输出 当前可分配 ip 的 总量和剩余量 等

4 关闭 dhcp server 接口， 停止 dhcpd 服务


agent 的  main 函数 主框架中， 根据 自身的AgentConfig.objSpec.Feature.enableDhcpServer 配置，来决定 是否要调用 pkg/dhcpserver 模块中的接口，来启动 dhcp server


需要 考虑 dhcpd 的 /var/lib/dhcp/dhcpd.leases 持久化问题，这样，在 agent 重启后，dhcpd 的数据可以持久化，不会丢失
这样，在 helm 中的 values.yaml 中， 需要支持 二选一的 方式 来 进行持久化 
（1）在调试环境中，使用宿主机的 本地挂载， 把 宿主机的 /var/lib/dhcp/ 目录 挂载给 agent pod  的 /var/lib/dhcp/ 
（2）在生产环境中，可使用 pvc 来存储 dhcpd 的数据，把 pvc 挂载给 agent pod 的 /var/lib/dhcp/
    NewDhcpServer 函数中，需要额外传入  clusterAgentName， 该变量用于生成 dhcp server 的 lease  文件名 /var/lib/dhcp/${clusterAgentName}-dhcpd.leases ， 给 dhcp server 使用， 这样，即使在 本地 磁盘持久阿虎是，能实现 文件的不冲突 
    
    
root@bmc-e2e-worker:/# cat /var/lib/dhcp/bmc-clusteragent-dhcpd.leases
          # The format of this file is documented in the dhcpd.leases(5) manual page.
          # This lease file was written by isc-dhcp-4.4.3-P1

          # authoring-byte-order entry is generated, DO NOT DELETE
          authoring-byte-order little-endian;

          server-duid "\000\001\000\001.\366\204O:R\256\274g\331";

          lease 192.168.0.10 {
            starts 4 2024/12/19 07:14:33;
            ends 6 2025/01/18 07:14:33;
            cltt 4 2024/12/19 07:14:33;
            binding state active;
            next binding state free;
            rewind binding state free;
            hardware ethernet 72:21:aa:24:56:d9;
            client-hostname "redfish-redfish-mockup-ff6b7749c-7l95j";
          }
          lease 192.168.0.11 {
            starts 4 2024/12/19 07:14:38;
            ends 6 2025/01/18 07:14:38;
            cltt 4 2024/12/19 07:14:38;
            binding state active;
            next binding state free;
            rewind binding state free;
            hardware ethernet a6:8a:53:f3:a8:03;
            client-hostname "redfish-redfish-mockup-ff6b7749c-7njhv";
          }

## hoststatus 管理模块

新建定义一个 cluster 级别的 crd hoststatus

```
apiVersion: topohub.infrastructure.io/v1beta1
kind: hostStatus
metadata:
  name: agentname-ipaddress
  ownerReferences:
  - apiVersion: topohub.infrastructure.io/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: hostEndpoint
    name: hostendpointname
status:
  healthReady: true  // 必须有值
  clusterAgent: "default"   // 必须有值
  lastUpdateTime: "2024-12-19T07:14:33Z"
  basic:  // 必须有值
    type: "dhcp"/"hostEndpoint"  // 必须有值
    ipAddr: "192.168.0.10"    // 必须有值
    secretName: "test"   //可有值，可为空
    secretNamespace: "bmc"    //可有值，可为空
    https: true     // 必须有值
    port: 80    // 必须有值
    mac: "00:0c:29:2f:3a:2a"  //可有值，可为空
  info:  // 必须有值
    os: "ubuntu"/""  //可有值，可为空
    power: "on"/"off"/"unknown"   //可有值，可为空
    snmpServer: "1.2.3.4"   //可有值，可为空
    snmpPort: 161   //可有值，可为空
  other: // 可有值，可为空, 它通过 additionalProperties 定义， 使得 其下层级的 成员 支持 多变的 key-value 对象， 实现 map[string]string 效果
    xxx: xxx
    xxx: xxx
```


请在 @pkg/k8s/apis/topohub.infrastructure.io/v1beta1 中创建 crd 定义后， 可使用 make update_crd_sdk  来生成 配套的 client sdk，位于 @pkg/k8s/client 下 ， 相关的 deep copy 函数，也会生成在 @pkg/k8s/apis ， 相关的 crd 定义 生成在 @chart/crds 下

请在 @templates/agent-templates.yaml 中的 agent role 中赋予 hoststatus 的权限

@pkg/agent/hoststatus 目录下 创建一个 hoststatus 维护模块，它 通过 interface{} 对外暴露使用，它应该有如下接口

（1）创建  维护模块 实例
      传入 agent的 agentConfig 对象 作为 工作参数
      传入 k8sClient

（2）运行接口
      * 它 启动一个携程， list watch 所有的 hostEndpoint 实例，当有 新的 hostEndpoint 对象时
          确认 hostEndpoint 对象 的 对应的 hostStatus 对象存在， 如果不存在，就创建一个
             hostStatus 对象的创建，遵循
                  hostStatus metadata.name = hostEndpoint spec.clusterAgent +  hostEndpoint spec.ipAddr(把 . 替换成 -)
                  hostStatus metadata.ownerReferences 关联到 该  hostEndpoint 。 从而可实现 级联删除
                  hostStatus status.healthReady = false
                  hostStatus status.clusterAgent = hostEndpoint spec.clusterAgent
                  hostStatus status.basic.type = "hostEndpoint"
                  hostStatus status.basic.ipAddr = hostEndpoint spec.ipAddr
                  hostStatus status.basic.secretName = hostEndpoint spec.secretName
                  hostStatus status.basic.secretNamespace = hostEndpoint spec.secretNamespace
                  hostStatus status.basic.https = hostEndpoint spec.https
                  hostStatus status.basic.port = hostEndpoint spec.port
                  hostStatus status.basic.mac = ""
                  刷新 hostStatus status.lastUpdateTime

      * 它 启动一个携程，通过 暴露两个 channel 变量，  让 @pkg/dhcpserver/server.go 中的  func (s *dhcpServer) updateStats() error 中 发生事件 时主动通知， 获取 新增 和 删除 的 client 信息
            当有新的 dhcp client 分配了 ip ， 把么 创建对应的 hostStatus 对象
                  hostStatus metadata.name = agentConfig 中的 clusterAgentName + 新 client 的 ip (把 . 替换成 -)
                  hostStatus metadata.ownerReferences 关联到 空
                  hostStatus status.healthReady = false
                  hostStatus status.clusterAgent = agentConfig 中的 clusterAgentName
                  hostStatus status.basic.type = "dhcp"
                  hostStatus status.basic.ipAddr = 新 client 的 ip
                  hostStatus status.basic.secretName = agentConfig 中 AgentObjSpec.Endpoint.secretName
                  hostStatus status.basic.secretNamespace = agentConfig 中 AgentObjSpec.Endpoint.secretNamespace
                  hostStatus status.basic.https =  agentConfig 中 AgentObjSpec.Endpoint.https
                  hostStatus status.basic.port = agentConfig 中 AgentObjSpec.Endpoint.port
                  hostStatus status.basic.mac = 新 client 的 mac
                  刷新 hostStatus status.lastUpdateTime

            当有 dhcp client 被释放 ip ， 把么 删除 对应  hostStatus 对象

    * 它 启动一个携程， 监听所有的  hostStatus 对象 ，实现对  hostStatus status.info  的信息维护 （更新函数中的代码。暂时为空，只打印 监听到 hostStatus 对象 变化的日志 ）


（3）停止接口

在 @cmd/agent 集成以上 hoststatus 维护模块， 实现它的 启动 和 停止

请不要修改和本问题无关的其他代码

//---------------

在 pkg/agent/hoststatus/data 中实现一个 数据缓存的模块 ，定义如下

type hostData struct {
  Info *BasicInfo
  Username string
  Password string
}

type hostCache struct {
  lock 
  map[string]hostData
}

它可实现 数据存储, 提供如下方法

1 初始化

2 添加成员， 输入 name 和 hostData ， 存储到 hostCache 中的 map

3 删除成员，输入 name


 

## dhcp 的 僵死 ip

dhcp 不支持 主动探活 client ip  
对于 hoststatus 中的 HEALTHREADY = false， 因此 需要手动 确认 
然后 进入 agent pod 中， 删除  /var/lib/dhcp/bmc-clusteragent-dhcpd.leases 文件中的 无效 ip 即可 

## redfish 接口
 
在 pkg/redfish 下创建一个 redfish 模块， 它使用接口 interface 向外暴露 使用 

它应该具备 多个方法，它们向 一个目的 发起 redfish 请求 ， 因此 每个方法都有 入参 @cache.go  中的 HostConnectCon 参数
   ip 地址是  HostConnectCon.Info.IpAddr ， 
   端口号是 HostConnectCon.Info.Port ， 
   如果 HostConnectCon.Info.Https==true ，则发现 https 请求，否则发起 http 请求
   如果 HostConnectCon.Username 和 HostConnectCon.Password 非空，则发起 http 的  用户名和密码 认证 来发送 请求

redfish 通信库可使用 golang 库  https://pkg.go.dev/github.com/stmcginnis/gofish

1 health 方法

它向 使用 gofish 库，调用 ServiceRoot 方法 


##  hostOperation CRD 

我需要实现一个 新的 CRD  ， 它用于实现 对 host 的 运维操作

其中 ，controller 组件会监听 该 CRD 对象，实现进行 validate and mutating 校验
controller 组件可在 @pkg/webhook/hostoperation 下 实现相关的 webhook， 在 @cmd/controller 中集成相关的逻辑


```
apiVersion: topohub.infrastructure.io/v1beta1
kind: hostOperation
metadata:
  name: power-off
spec:
  action: "powerOff"|"powerOn"|"reboot"|"pxeReboot"  // 必填
  hostStatusName: "test"  // 必填， controller 组件确认，该名字 对应的 hostStatus crd 实例要存在，且其 status.healthy 要求为 true
status:
  status: "pending"|"success"|"failure"  // 对象创建后，默认初始是 pending 状态
  message: "xxxx"
  lastUpdateTime: "2024-12-19T07:14:33Z"
  clusterAgent: default-agent  //对象创建后，controller 组件的 mutating webhook ， 根据 spec.hostStatusName ，寻找对应的 hostStatus crd 实例 ， 把其中的 status.clusterAgent 赋值给 它
```

请在 @pkg/k8s/apis/topohub.infrastructure.io/v1beta1 中创建 crd 定义后， 可使用 make update_crd_sdk  来生成 配套的 client sdk，位于 @pkg/k8s/client 下 ， 相关的 deep copy 函数，也会生成在 @pkg/k8s/apis ， 相关的 crd 定义 生成在 @chart/crds 下


