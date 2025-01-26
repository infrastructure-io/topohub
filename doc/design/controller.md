# controller 


## crd hostEndpoint

需要实现一个 crd hostEndpoint ， 它的定义如下

```
apiVersion: topohub.infrastructure.io/v1beta1
kind: hostEndpoint
metadata:
  name: test
spec:
  clusterAgent: "default" //  可选， 当为空时， controller 的 mutating webhook 中，如果 集群中 只有一个  clusteragent 实例，那么把 该 实例的名字 赋给它 。  controller 的 validate webhook 进行校验，不允许它为空，且 它所代表的 crd clusterAgent 的实例必须存在

  ipAddr: "192.168.0.10" //必填， controller 的 validate webhook 进行校验， 它和 spec.clusterAgent 对应的 agent clusterAgent 实例中的 spec.dhcpServerConfig.subnet="192.168.0.0/24"  子网, 要属于该子网 . controller 的 validate webhook 进行校验， 它和存量的其它所有   hostEndpoint 实例的  spec.ipAddr 不能相同
  
  secretName: "test" // 可选， 默认为空串 。 
  
  secretNamespace: "bmc" // 可选， 默认为空串。当 spec.secretName 和 spec.secretNamespace 都不为空时， controller 的 validate webhook 进行校验 ， 确认所对应的 secrect 存在， 且 secrect 中的数据 有 username 和 password 的 key
  
  https: true // 用户可不填，默认为 true  
  port: 80  // 用户可不填，默认为 80
```

请在 @crd.yaml 中生成 crd ， 
在 pkg/webhook/hostendpoint 下 实现相关的 webhook，
在 @cmd/controller 中集成相关的逻辑
需要在 @chart/templates/webhook.yaml  下 创建相关的 webhook 对象，  使用 helm genCA 来生成 CA 证书的方式
该对象只允许创建，不允许修改

请不要修改和本问题无关的其他代码



