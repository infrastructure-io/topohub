# 设计

写一个 kubernetes 下的 组件

##  总体架构

本组件由两个部分构成

- 一组 controller deployment ， 它主要负责 创建 agent 组件的 deployment
- 一组 agent deployment， 它主要分支完成具体的业务

## controller 组件

1.它能够通过 helm 进行 安装，必要的安装参数，请基于 helm values 进行暴露
安装后，默认安装 一个 1 pod 的 deployment 的 controller 组件

- helm 的 chart 代码 放置在工程的 chart 目录下

- helm中 的 deployment 的 镜像名为 infrastructure-io/topohub/controller
  helm中 的 deployment 的 镜像 tag 默认使用 chart.yaml 中的 version 也可以支持通过 values 来指定 

- controller pod 中的 进程 golang 代码放在 cmd/controller 目录下，所有的代码，要求有详细的 debug 级别日志，必要的 log 级别和 error 级别的日志，有必要的 代码注释

- chart 的  chart.yaml 中的 version 为工程根目录下的 VERSION 文件中的值

- deployment yaml 中 有 pod 健康检查的设置 ， 访问 8000 端口

2.helm  安装 了如下 crd

apiVersion: topohub.infrastructure.io/v1beta1
kind: clusterAgent
metadata:
  name: test
spec:
  agentYaml:
    underlayInterface: “kube-system/macvlan-pod-network”
    image: ""
    replicas: 1
    nodeAffinity
    hostNetwork: false 
  endpoint:
    port: 80
    secretName: "abc"
    secretNamespace: "bmc"
    https: true
  feature:
    enableDhcpServer: true
    dhcpServerConfig: // 可选，当 enableDhcpServer=true 时，controller 的 webhook 校验必须有值
      enableDhcpDiscovery: true	 // 必须有值，缺省 true
      dhcpServerInterface: "net1"  // 必须有值
      subnet: "192.168.0.0/24" // 必须有值， controller 的 webhook 校验 子网格式
      ipRange: "192.168.0.10-192.168.0.100" // 必须有值，controller 的 webhook 校验 格式
      gateway: "192.168.0.1"  // 必须有值， controller 的 webhook 校验 格式
      selfIp: "192.168.0.2/24" // 可选， controller 的 webhook 校验 格式
status:
  ready: true

所有的 crd 定义的 golang 代码， 放置在 pkg/apis 的 相关目录下

并且 ， helm 默认安装了一个 clusterAgent 的 cr 实例， 实例中 spec 的各个字段，基于 helm 的 values 中的配置进行填充，其中，
    spec.agentYaml.underlayInterface 是一个必写字段， helm values 默认为空， 
    spec.agentYaml.replicas 是一个可选字段， 默认 helm values 为 1
    spec.agentYaml.image 是一个可选字段，由 相应的 helm values 来渲染，该来渲染，该 value 由 两部分组成，image.repository 默认为 infrastructure-io/topohub/agent， image.tag 默认使用 chart.yaml 中的 version

    
3. controller 组件的 golang 代码中， 
     相应的 helm values 来渲染，该来渲染，该 value 由 两部分组成，image.repository 默认为 infrastructure-io/topohub/agent， image.tag 默认使用 chart.yaml 中的 version

3  controller 的 deployment 中的 一个环境变量 agentImage ， 该值代表镜像名，  通过 helm values 进行渲染，vlaues 由两个部分组成， image.repository 默认为 infrastructure-io/topohub/agent ，镜像 tag 默认使用 chart.yaml 中的 version  

3. 当用户创建了 crd clusterAgent 的实例后，controller 的 golang 代码程序中，需要在 kubernetes 中 创建出 对应的 一个 deployment 实例 ， 它名为 agent 组件
当用户 删除了 ， crd clusterAgent 的实例后，controller 的 golang 代码程序中 ， 应该尝试删除当初创建的 对于的 k8s 的对象

- 在 helm 的 chart 中 ，使用 configmap 来存储  agent 的 deployment 和 其对应的 role/rolebinding serviceaccount 的 yaml 目标，该 configmap 挂载到 controller pod 中， controller 的 golang 代码程序 基于 
该 yaml 模板 来渲染生成 agent 实例

- agent 组件 的  deployment 的 实例名， 为  "agent" + crd clusterAgent 的 metadata.name 

- agent 组件 的  deployment 的租户，与controller pod 相同  

- agent 组件 的  deployment 的 副本数，遵循 crd clusterAgent 的 spec.agentYaml.replicas ， 否则默认为 1   

- agent 组件 的  image ，遵循 crd clusterAgent 的 spec.agentYaml.image ， 如果该字段为空， 的实例后，controller 使用自己的 环境变量 agentImage 的值来 渲染

- agent 组件 的  yaml 中，在 8000 端口上的健康检查配置

- agent 组件 的  yaml 中，配置一些与该实例对应的 必要 label 和 annotation 

- 基于 crd clusterAgent 中的 spec.agentYaml.underlayInterface 的值， 设置  agent 组件 的 deployment 的 annotation 中带有如下
k8s.v1.cni.cncf.io/networks: "k8s.v1.cni.cncf.io/networks"

- 基于 crd clusterAgent 中的 meta.name 的值， 该 agent 组件 的 deployment 注入 环境变量，环境变量的 key  为 ClusterName， 环境变量的值为 meta.Name

- controller 的进程能够 监控 每个  crd clusterAgent 对应的 deployment 实例的状态，如果它的所有 pod 是 running 的，那么就 标记 CRD clusterAgent 中的 status.ready=true， 否则 status.ready=false

- controller 在创建  agent deployment 同时，创建必要的 serviceaccount  和 role/rolebinding

- controller 要监控 crd clusterAgent 的实例销毁事件，当发生时，要删除 对应的 agent 的所有 资源，包括 deployment 、 serviceaccount 、 role/rolebinding 等 

4. controller 代码 具备以下编程要点

- 能监听在的端口 8000 上，提供必要的 http 接口，用于进行 pod 的健康检查

- controller 要具备 kubernetes 的 leader 选主机制，当 controller deployment 的 副本数 大于1 时候，controller 的 leader 选举机制是必要的

5.  在 controller 代码中，添加对于 ClusterAgent crd 实例的 webhook 逻辑
- helm 自动创建相关的 service、webhook，使用 helm 为 webhook 注入 100 年可用的 tls 证书
- ClusterAgent 的 spec.agentYaml.image 是可选字段，如果创建的 cr 有该值，则使用它， 如果 没有， webhhook 对其修改，设置为 controller pod 的 yaml 中的 环境变量 AGENT_IMAGE , 该 环境变量的值 来自与 helm values 中的 clusterAgent.image.repository 和 clusterAgent.image.tag 的 渲染 
- ClusterAgent 的 spec.agentYaml.replicas 是可选字段，如果创建的 cr 有该值，则使用它, 但必须是一个 大于等于 0 的数字， 如果 没有， webhhook 对其修改，设置为 1 
- ClusterAgent 的 spec.agentYaml.underlayInterface 是必填字段
- controller 需要监控 每个 ClusterAgent cr 实例对应的 deployment 的状态， 如果是 所有副本正常 的 running ，则更新对应的 ClusterAgent cr 实例 的 status.ready=true， 否则 status.ready=false
 
 
## agent 组件

agent pod 中的 进程 golang 代码放在 cmd/agent 目录下， 所有的代码，要求有详细的 debug 级别日志，必要的 log 级别和 error 级别的日志
，有必要的 代码注释

- agent 的 server 进程，默认一直在 sleep 

- agent 组件 的  yaml 中，在 8000 端口上的健康检查配置


## 镜像 

构建镜像，遵循如下规则

- controller pod 的 镜像 构建 docker 位于 image/controller 目录下：
在 dockerfile 中, 先试用 golang:1.23.1 基础镜像，把整个工程 copy 进入镜像中，使用工程根目录下 makefile 中的能力，把 cmd/controller 目录下的 golang 代码进行编译。 在使用 ubuntu:24.10 基础镜像， 把 编译的 二进制拷贝过来，打包进入 镜像，作为镜像的启动程序入口 

- agent pod 的 镜像 构建 docker 位于 image/agent 目录下
在 dockerfile 中, 先试用 golang:1.23.1 基础镜像，把整个工程 copy 进入镜像中，使用工程根目录下 makefile 中的能力，把 cmd/agent 目录下的 golang 代码进行编译。 在使用 ubuntu:24.10 基础镜像， 把 编译的 二进制拷贝过来，打包进入 镜像，作为镜像的启动程序入口 


## makefile

工程根目录下，有个 makefile 文件，它是一个 makefile 的工程，可以使用 make 命令进行编译， 它要支持：
- 基于  image/controller/Dockerile， 能够生成 controller 的 docker 镜像，镜像名为  infrastructure-io/topohub/controller ，镜像的 tag 为工程根目录下的 VERSION 中的值

- 基于  image/agent/Dockerile， 能够生成 server 的 docker 镜像，镜像名为  infrastructure-io/topohub/server，镜像的 tag 为工程根目录下的 VERSION 中的值
 
- 生成 helm chart 

- 关于 makefile 的用法，支持 使用 make usage 进行输出显示

## 整个工程的编码

- 整个golang 的工程为 github.com/infrastructure-io/topohub ， 所以，所有功能内的相关代码引用，请遵循该规范

- 工程中所有的 golang、shell代码文件中，都使用英文，包括代码注释和日志

- 对所有的 golang 代码进行合理拆分，按照 功能 进行合理 文件规划，避免出现 单个巨型代码 文件

-  在 @main.go 和 @main.go  中，已经完成对  github.com/infrastructure-io/topohub/pkg/log 中的 初始化了，因此，请在整个工程的其他地方 打印日志时，请使用 github.com/infrastructure-io/topohub/pkg/log 中的 log.Logger 来打印日志， 使用 printf 风格， 请具体区分 相关的 日志界别，使用 log.Logger.Infof    log.Logger.Debugf  log.Logger.Errorf   等 风格 。 在修改时，只修改 打印日志的代码，不要优化 其它 不相关的逻辑

## 文档

在 根目录的 README.md 中, 描述一下内容：
- 整个工程的功能描述
- kubernetes 基本安装方法

在工程的所有 golang 代码目录下，创建必要的 readme.md 文件，对相关目录下的代码做简要的设计说明

## 坑

- 从一开始，注意 使用 go get -u ./... 升级所有库到最新 ，否则一些 api 兼容问题， ai 是很难解决的

- 注意 api 定义的 文件夹路径，使用 code gen 生成 client sdk  ， 不要用 ai 来生成

